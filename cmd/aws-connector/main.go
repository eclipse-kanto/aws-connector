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

package main

import (
	"flag"
	"log"
	"os"

	"github.com/imdario/mergo"
	"github.com/pkg/errors"

	"github.com/eclipse-kanto/suite-connector/config"
	"github.com/eclipse-kanto/suite-connector/logger"

	"github.com/eclipse-kanto/aws-connector/cmd/aws-connector/app"
	awscfg "github.com/eclipse-kanto/aws-connector/config"
	"github.com/eclipse-kanto/aws-connector/flags"
	"github.com/eclipse-kanto/aws-connector/routing/message/handlers"
	"github.com/eclipse-kanto/aws-connector/routing/message/handlers/passthrough"
	suiteFlags "github.com/eclipse-kanto/suite-connector/flags"
)

var (
	version = "development"
)

func main() {
	f := flag.NewFlagSet("aws-connector", flag.ContinueOnError)

	cmd := new(awscfg.CloudSettings)
	flags.Add(f, cmd)
	fConfigFile := suiteFlags.AddGlobal(f)

	if err := suiteFlags.Parse(f, os.Args[1:], version, os.Exit); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		} else {
			os.Exit(2)
		}
	}

	settings := awscfg.DefaultSettings()
	if err := config.ReadConfig(*fConfigFile, settings); err != nil {
		log.Fatal(errors.Wrap(err, "cannot parse config"))
	}

	cli := suiteFlags.Copy(f)
	if err := mergo.Map(settings, cli, mergo.WithOverwriteWithEmptyValue); err != nil {
		log.Fatal(errors.Wrap(err, "cannot process settings"))
	}

	if err := settings.ReadDeviceID(); err != nil {
		log.Fatal(errors.Wrap(err, "cannot read deviceId from its certificate"))
	}

	if err := settings.CompileFilters(); err != nil {
		log.Fatal(errors.Wrap(err, "cannot compile regular expression filters"))
	}

	if err := settings.Validate(); err != nil {
		log.Fatal(errors.Wrap(err, "settings validation error"))
	}

	loggerOut, logger := logger.Setup("aws-connector", &settings.LogSettings)
	defer loggerOut.Close()

	logger.Infof("Starting aws connector %s", version)
	suiteFlags.ConfigCheck(logger, *fConfigFile)

	deviceHandlers := []handlers.MessageHandler{
		passthrough.CreateDefaultDeviceHandler(),
	}
	cloudHandlers := []handlers.MessageHandler{}

	if err := app.MainLoop(settings, logger, deviceHandlers, cloudHandlers); err != nil {
		logger.Error("Init failure", err, nil)
		loggerOut.Close()
		os.Exit(1)
	}
}
