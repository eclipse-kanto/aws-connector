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

package state

import (
	"fmt"
	"testing"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/eclipse-kanto/aws-connector/config"
	"github.com/eclipse-kanto/aws-connector/routing/message/handlers"
	"github.com/eclipse-kanto/aws-connector/routing/message/handlers/passthrough"
	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const validPayload = `{
		"state": {
			"reported": {
				"test": "value"
			}
		}
	}`

var expectedShadowState = map[string]interface{}{
	"test": "value",
}

func TestCreateDefaultHandler(t *testing.T) {
	handler := CreateDefaultShadowStateHandler()

	require.NoError(t, handler.Init(settings(), watermill.NopLogger{}))
	assert.Equal(t, "shadow_state_handler", handler.Name())
	assert.Equal(t, "$aws/things/test:device/shadow/update/accepted,$aws/things/test:device/shadow/delete/accepted,$aws/things/test:device/shadow/name/+/update/accepted,$aws/things/test:device/shadow/name/+/delete/accepted", handler.Topics())
}

func TestErrorWhenTopicMissingInMessage(t *testing.T) {
	handler := CreateDefaultShadowStateHandler()
	handler.Init(settings(), watermill.NopLogger{})

	message := &message.Message{Payload: []byte(validPayload)}

	result, err := handler.HandleMessage(message)
	assert.NotNil(t, err)
	assert.Nil(t, result)
}

func TestErrorWhenUpdatingAndPayloadNotMap(t *testing.T) {
	assertErrorWhenUpdatingAndPayloadIncorrect(t, "payload")
}

func TestErrorWhenUpdaingAndPayloadHasNoState(t *testing.T) {
	assertErrorWhenUpdatingAndPayloadIncorrect(t, `{"payload": "invalid"}`)
}

func TestErrorWhenUpdatingAndPayloadHasNoReported(t *testing.T) {
	assertErrorWhenUpdatingAndPayloadIncorrect(t, `{"state": {"invalid": {}"}}`)
}

func TestErrorWhenUpdatingAndStateNotMap(t *testing.T) {
	assertErrorWhenUpdatingAndPayloadIncorrect(t, `{"state": "invalid"}`)
}

func TestNoErrorWhenDeletingAndPayloadNotMap(t *testing.T) {
	asserNoErrorWhenDeletingAndPaylodIncorrect(t, "payload")
}

func TestNoErrorWhenDeletingAndStateNotMap(t *testing.T) {
	asserNoErrorWhenDeletingAndPaylodIncorrect(t, `{"state": "invalid"}`)
}

func TestNoErrorWhenDeletingAndPayloadHasNoState(t *testing.T) {
	asserNoErrorWhenDeletingAndPaylodIncorrect(t, `{"payload": "invalid"}`)
}

func TestNoErrorWhenDeletingAndPayloadHasNoReported(t *testing.T) {
	asserNoErrorWhenDeletingAndPaylodIncorrect(t, `{"state": {"invalid": {}}}`)
}

func TestUpdateRootShadow(t *testing.T) {
	assertUpdateShadow(t, "$aws/things/test:device/shadow", "test:device")
}

func TestDeleteRootShadow(t *testing.T) {
	assertDeleteShadow(t, "$aws/things/test:device/shadow", "test:device")
}

func TestUpdateChildShadow(t *testing.T) {
	assertUpdateShadow(t, "$aws/things/test:device/shadow/name/test:device:child", "test:device:child")
}

func TestDeleteChildShadow(t *testing.T) {
	assertDeleteShadow(t, "$aws/things/test:device/shadow/name/test:device:child", "test:device:child")
}

func assertErrorWhenUpdatingAndPayloadIncorrect(t *testing.T, payload string) {
	handler, message := setUp(payload, "$aws/things/test:device/shadow/update/accepted")
	result, err := handler.HandleMessage(message)
	assert.NotNil(t, err)
	assert.Nil(t, result)
}

func asserNoErrorWhenDeletingAndPaylodIncorrect(t *testing.T, payload string) {
	handler, message := setUp(payload, "$aws/things/test:device/shadow/delete/accepted")
	result, err := handler.HandleMessage(message)
	assert.Nil(t, err)
	assert.Nil(t, result)
}

func assertDeleteShadow(t *testing.T, topicBase string, shadowID string) {
	assertUpdateShadow(t, topicBase, shadowID)

	topic := fmt.Sprint(topicBase, "/delete/accepted")
	handler, message := setUp(validPayload, topic)
	shadowHolder := handler.(passthrough.ShadowStateHolder)

	result, err := handler.HandleMessage(message)
	assert.Nil(t, err)
	assert.Nil(t, result)

	currentState := shadowHolder.GetCurrentShadowState(settings().DeviceID)

	assert.Nil(t, currentState)
}

func assertUpdateShadow(t *testing.T, topicBase string, shadowID string) {
	topic := fmt.Sprint(topicBase, "/update/accepted")
	handler, message := setUp(validPayload, topic)
	shadowHolder := handler.(passthrough.ShadowStateHolder)

	result, err := handler.HandleMessage(message)
	assert.Nil(t, err)
	assert.Nil(t, result)

	currentState := shadowHolder.GetCurrentShadowState(shadowID)
	assert.Equal(t, expectedShadowState, currentState)
}

func setUp(payload string, topic string) (handlers.MessageHandler, *message.Message) {
	handler := CreateDefaultShadowStateHandler()
	handler.Init(settings(), watermill.NopLogger{})

	message := &message.Message{Payload: []byte(payload)}
	message.SetContext(connector.SetTopicToCtx(message.Context(), topic))

	return handler, message
}

func settings() *config.CloudSettings {
	settings := &config.CloudSettings{}
	settings.TenantID = "test-tenant-id"
	settings.DeviceID = "test:device"
	return settings
}
