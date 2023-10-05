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

package bus

import (
	"github.com/eclipse-kanto/aws-connector/config"
	"github.com/eclipse-kanto/aws-connector/routing/message/handlers"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/eclipse-kanto/suite-connector/connector"
)

// MessageBus creates the message bus for processing & forwarding the messages from the Subscriber to the Publisher.
func MessageBus(router *message.Router,
	pub message.Publisher,
	sub message.Subscriber,
	settings *config.CloudSettings,
	handlers []handlers.MessageHandler,
) {
	for _, handler := range handlers {
		if err := handler.Init(settings, router.Logger()); err != nil {
			logFields := watermill.LogFields{"handler_name": handler.Name()}
			router.Logger().Error("skipping handler that cannot be initialized", err, logFields)
			continue
		}
		topics := handler.Topics()
		if len(topics) == 0 {
			logFields := watermill.LogFields{"handler_name": handler.Name()}
			router.Logger().Error("skipping handler without any topics", nil, logFields)
			continue
		}
		router.AddHandler(handler.Name(), topics, sub, connector.TopicEmpty, pub, handler.HandleMessage)
	}
}
