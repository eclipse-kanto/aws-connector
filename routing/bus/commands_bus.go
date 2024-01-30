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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/eclipse-kanto/suite-connector/cache"
	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/routing"
	"github.com/eclipse/ditto-clients-golang/protocol"
)

const (
	handlerName = "passthrough_commands_request_handler"
	topics      = "command//+/req/#,cmd//+/q/#"
)

// CommandsReqBus creates the commands request bus.
func CommandsReqBus(router *message.Router,
	pub message.Publisher,
	sub message.Subscriber,
	reqCache *cache.Cache,
	deviceID string,
) *message.Handler {
	handler := filter(deviceID, routing.NewCommandRequestHandler(reqCache, "", deviceID, false))
	return router.AddHandler(handlerName, topics, sub, connector.TopicEmpty, pub, handler)
}

// filter creates middleware handler which filter all messages not associated with provided deviceId.
func filter(deviceID string, h message.HandlerFunc) message.HandlerFunc {
	return func(msg *message.Message) ([]*message.Message, error) {
		env := &protocol.Envelope{Headers: protocol.NewHeaders()}
		if err := json.Unmarshal(msg.Payload, &env); err == nil {
			id := fmt.Sprintf("%s:%s", env.Topic.Namespace, env.Topic.EntityName)
			if id == deviceID || strings.HasPrefix(id, deviceID+":") {
				return h(msg)
			}
		}
		return []*message.Message{}, nil
	}
}
