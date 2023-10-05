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
	"context"
	"reflect"
	"testing"

	test "github.com/eclipse-kanto/aws-connector/routing/bus/internal/testing"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/eclipse-kanto/suite-connector/cache"
	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommandsReqBus(t *testing.T) {
	router, _ := message.NewRouter(message.RouterConfig{}, watermill.NopLogger{})
	reqCache := cache.NewTTLCache()

	CommandsReqBus(router, connector.NullPublisher(), test.NewDummySubscriber(), reqCache, deviceID)
	refRouterPtr := reflect.ValueOf(router)
	refRouter := reflect.Indirect(refRouterPtr)
	refHandlers := refRouter.FieldByName(fieldHandlers)
	assert.Equal(t, 1, refHandlers.Len())
	refHandler := refHandlers.MapIndex(refHandlers.MapKeys()[0])
	test.AssertRouterHandler(t, handlerName, topics, "", reflect.Indirect(refHandler))
}

func TestFilter(t *testing.T) {
	h := filter(deviceID, message.PassthroughHandler)
	assertAccepted(t, h, `{"topic":"test/device/things/live/messages/install"}`)
	assertAccepted(t, h, `{"topic":"test/device:child/things/live/messages/install"}`)
}

func assertAccepted(t *testing.T, h message.HandlerFunc, payload string) {
	topic := "command///req/id/install"
	msg := message.NewMessage("test", []byte(payload))
	msg.SetContext(connector.SetTopicToCtx(context.Background(), topic))

	msgs, err := h(msg)
	require.Equal(t, 1, len(msgs))
	assert.NoError(t, err)
	assert.Equal(t, payload, string(msgs[0].Payload))
	actualTopic, ok := connector.TopicFromCtx(msgs[0].Context())
	assert.True(t, ok)
	assert.Equal(t, topic, actualTopic)
}

func TestFilterNotAccepted(t *testing.T) {
	h := filter(deviceID, message.PassthroughHandler)
	assertNotAccepted(t, h, `{"topic":"test/devices/things/live/messages/install"}`)
	assertNotAccepted(t, h, `{"topic":"test/devica/things/live/messages/install"}`)
	assertNotAccepted(t, h, `{"topic":"atest/device/things/live/messages/install"}`)
}

func assertNotAccepted(t *testing.T, h message.HandlerFunc, payload string) {
	msg := message.NewMessage("test", []byte(payload))
	msgs, err := h(msg)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(msgs))
}
