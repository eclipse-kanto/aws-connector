// Copyright (c) 2023 Contributors to the Eclipse Foundation
//
// See the NOTICE file(s) distributed with this work for additional
// information regarding copyright ownership.
//
// This program and the accompanying materials are made available under the
// terms of the Eclipse Public License 2.0 which is available at
// https://www.eclipse.org/legal/epl-2.0, or the Apache License, Version 2.0
// which is available at https://www.apache.org/licenses/LICENSE-2.0.
//
// SPDX-License-Identifier: EPL-2.0 OR Apache-2.0

package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	awscfg "github.com/eclipse-kanto/aws-connector/config"
	"github.com/eclipse-kanto/aws-connector/routing/bus"
	"github.com/eclipse-kanto/aws-connector/routing/message/handlers"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/eclipse-kanto/suite-connector/cache"
	"github.com/eclipse-kanto/suite-connector/config"
	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/routing"
	"github.com/pkg/errors"
)

func startRouter(
	localClient *connector.MQTTConnection,
	settings *awscfg.CloudSettings,
	statusPub message.Publisher,
	deviceHandlers []handlers.MessageHandler,
	cloudHandlers []handlers.MessageHandler,
	done chan bool,
	logger logger.Logger,
) (*message.Router, error) {
	cloudClient, err := config.CreateCloudConnection(&settings.LocalConnectionSettings, false, logger)
	if err != nil {
		return nil, errors.Wrap(err, "cannot create mosquitto connection")
	}

	awsClient, cleanup, err := config.CreateHubConnection(&settings.HubConnectionSettings, false, logger)
	if err != nil {
		routing.SendStatus(routing.StatusConnectionError, statusPub, logger)
		return nil, errors.Wrap(err, "cannot create Hub connection")
	}

	logger.Info("Starting messages router...", nil)
	router, err := message.NewRouter(message.RouterConfig{}, logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create router")
	}

	paramsPub := connector.NewPublisher(localClient, connector.QosAtMostOnce, logger, nil)
	paramsSub := connector.NewSubscriber(cloudClient, connector.QosAtMostOnce, false, logger, nil)

	params := routing.NewGwParams(settings.DeviceID, settings.TenantID, "")
	routing.ParamsBus(router, params, paramsPub, paramsSub, logger)
	routing.SendGwParams(params, false, paramsPub, logger)

	awsPub := connector.NewPublisher(awsClient, connector.QosAtLeastOnce, logger, nil)
	awsSub := connector.NewSubscriber(awsClient, connector.QosAtMostOnce, false, logger, nil)
	mosquittoSub := connector.NewSubscriber(cloudClient, connector.QosAtLeastOnce, false, router.Logger(), nil)
	cloudPub := connector.NewPublisher(cloudClient, connector.QosAtLeastOnce, router.Logger(), nil)

	reqCache := cache.NewTTLCache()

	bus.MessageBus(router, awsPub, mosquittoSub, settings, deviceHandlers)
	bus.MessageBus(router, cloudPub, awsSub, settings, cloudHandlers)
	bus.CommandsReqBus(router, cloudPub, awsSub, reqCache, settings.DeviceID)
	routing.CommandsResBus(router, awsPub, mosquittoSub, reqCache, settings.DeviceID)

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			defer func() {
				routing.SendStatus(routing.StatusConnectionClosed, statusPub, logger)

				reqCache.Close()

				if cleanup != nil {
					cleanup()
				}

				logger.Info("Messages router stopped", nil)
				done <- true
			}()

			<-router.Running()

			statusHandler := &routing.ConnectionStatusHandler{
				Pub:    statusPub,
				Logger: logger,
			}
			cloudClient.AddConnectionListener(statusHandler)

			reconnectHandler := &routing.ReconnectHandler{
				Pub:    paramsPub,
				Params: params,
				Logger: logger,
			}
			cloudClient.AddConnectionListener(reconnectHandler)

			connHandler := &routing.CloudConnectionHandler{
				CloudClient: cloudClient,
				Logger:      logger,
			}
			awsClient.AddConnectionListener(connHandler)

			errorsHandler := &routing.ErrorsHandler{
				StatusPub: statusPub,
				Logger:    logger,
			}
			awsClient.AddConnectionListener(errorsHandler)

			if err := config.HonoConnect(nil, statusPub, awsClient, logger); err != nil {
				router.Close()
				return
			}

			<-ctx.Done()

			awsClient.RemoveConnectionListener(errorsHandler)
			awsClient.RemoveConnectionListener(connHandler)
			cloudClient.RemoveConnectionListener(reconnectHandler)
			cloudClient.RemoveConnectionListener(statusHandler)

			defer routing.SendStatus(routing.StatusConnectionClosed, statusPub, logger)

			defer awsClient.Disconnect()

			cloudClient.Disconnect()
		}()

		if err := router.Run(context.Background()); err != nil {
			logger.Error("Failed to create cloud router", err, nil)
		}

		logger.Info("Messages router stopped", nil)
	}()

	return router, nil
}

// MainLoop is the main loop of the application
func MainLoop(settings *awscfg.CloudSettings, log logger.Logger, deviceHandlers []handlers.MessageHandler, cloudHandlers []handlers.MessageHandler) error {
	localClient, err := config.CreateLocalConnection(&settings.LocalConnectionSettings, log)
	if err != nil {
		return errors.Wrap(err, "cannot create mosquitto connection")
	}
	if err := config.LocalConnect(context.Background(), localClient, log); err != nil {
		return errors.Wrap(err, "cannot connect to mosquitto")
	}
	defer localClient.Disconnect()

	statusPub := connector.NewPublisher(localClient, connector.QosAtLeastOnce, log, nil)
	defer statusPub.Close()

	done := make(chan bool, 1)
	awsRouter, err := startRouter(localClient, settings, statusPub, deviceHandlers, cloudHandlers, done, log)
	if err != nil {
		log.Error("Failed to create message bus", err, nil)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	stopRouter(awsRouter, done)

	return nil
}

func stopRouter(router *message.Router, done <-chan bool) {
	if router != nil {
		router.Close()
		<-done
	}
}
