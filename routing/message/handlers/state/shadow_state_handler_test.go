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

package state_test

import (
	"testing"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/eclipse-kanto/aws-connector/config"
	"github.com/eclipse-kanto/aws-connector/routing/message/handlers/passthrough"
	"github.com/eclipse-kanto/aws-connector/routing/message/handlers/state"
	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDefaultHandler(t *testing.T) {
	handler := state.CreateDefaultShadowStateHandler()

	require.NoError(t, handler.Init(settings(), watermill.NopLogger{}))
	assert.Equal(t, "shadow_state_handler", handler.Name())
	assert.Equal(t, "$aws/things/test:device/shadow/update/accepted,$aws/things/test:device/shadow/delete/accepted,$aws/things/test:device/shadow/name/+/update/accepted,$aws/things/test:device/shadow/name/+/delete/accepted", handler.Topics())
}

func TestErrorWhenTopicMissingInMessage(t *testing.T) {
	handler := state.CreateDefaultShadowStateHandler()
	handler.Init(settings(), watermill.NopLogger{})

	message := &message.Message{Payload: []byte("payload")}
	result, err := handler.HandleMessage(message)
	assert.NotNil(t, err)
	assert.Nil(t, result)
}

func TestErrorWhenUpdatingAndPayloadNotMap(t *testing.T) {
	handler := state.CreateDefaultShadowStateHandler()
	handler.Init(settings(), watermill.NopLogger{})

	message := &message.Message{Payload: []byte("payload")}
	message.SetContext(connector.SetTopicToCtx(message.Context(), "$aws/things/test:device/shadow/update/accepted"))

	result, err := handler.HandleMessage(message)
	assert.NotNil(t, err)
	assert.Nil(t, result)
}

func TestNoErrorWhenDeletingAndPayloadNotMap(t *testing.T) {
	handler := state.CreateDefaultShadowStateHandler()
	handler.Init(settings(), watermill.NopLogger{})

	message := &message.Message{Payload: []byte("payload")}
	message.SetContext(connector.SetTopicToCtx(message.Context(), "$aws/things/test:device/shadow/delete/accepted"))

	result, err := handler.HandleMessage(message)
	assert.Nil(t, err)
	assert.Nil(t, result)
}

func TestErrorWhenPayloadHasNoStateFieldStructure(t *testing.T) {
	handler := state.CreateDefaultShadowStateHandler()
	handler.Init(settings(), watermill.NopLogger{})

	message := &message.Message{Payload: []byte(`{"payload": "invalid"}`)}
	message.SetContext(connector.SetTopicToCtx(message.Context(), "$aws/things/test:device/shadow/update/accepted"))

	result, err := handler.HandleMessage(message)
	assert.NotNil(t, err)
	assert.Nil(t, result)
}

func TestUpdateRootShadow(t *testing.T) {
	handler := state.CreateDefaultShadowStateHandler()
	handler.Init(settings(), watermill.NopLogger{})
	shadowHolder := handler.(passthrough.ShadowStateHolder)
	payload := `{
		"state": {
			"reported": {
				"test": "value"
			}
		}
	}`

	expected := map[string]interface{}{
		"test": "value",
	}

	message := &message.Message{Payload: []byte(payload)}
	message.SetContext(connector.SetTopicToCtx(message.Context(), "$aws/things/test:device/shadow/update/accepted"))

	result, err := handler.HandleMessage(message)
	assert.Nil(t, err)
	assert.Nil(t, result)

	currentState := shadowHolder.GetCurrentShadowState("test:device")
	assert.Equal(t, expected, currentState)
}

func TestDeleteRootShadow(t *testing.T) {
	handler := state.CreateDefaultShadowStateHandler()
	handler.Init(settings(), watermill.NopLogger{})
	shadowHolder := handler.(passthrough.ShadowStateHolder)
	payload := `{
		"state": {
			"reported": {
				"test": "value"
			}
		}
	}`

	message := &message.Message{Payload: []byte(payload)}
	message.SetContext(connector.SetTopicToCtx(message.Context(), "$aws/things/test:device/shadow/update/accepted"))

	result, err := handler.HandleMessage(message)
	assert.Nil(t, err)
	assert.Nil(t, result)

	currentState := shadowHolder.GetCurrentShadowState("test:device")
	assert.NotNil(t, currentState)

	message.SetContext(connector.SetTopicToCtx(message.Context(), "$aws/things/test:device/shadow/delete/accepted"))
	handler.HandleMessage(message)

	currentState = shadowHolder.GetCurrentShadowState("test:device")
	assert.Nil(t, currentState)
}

func TestUpdateChildShadow(t *testing.T) {
	handler := state.CreateDefaultShadowStateHandler()
	handler.Init(settings(), watermill.NopLogger{})
	shadowHolder := handler.(passthrough.ShadowStateHolder)
	payload := `{
			"state": {
				"reported": {
					"test": "value"
				}
			}
		}`

	expected := map[string]interface{}{
		"test": "value",
	}

	message := &message.Message{Payload: []byte(payload)}
	message.SetContext(connector.SetTopicToCtx(message.Context(), "$aws/things/test:device/shadow/name/test:device:child/update/accepted"))

	result, err := handler.HandleMessage(message)
	assert.Nil(t, err)
	assert.Nil(t, result)

	currentState := shadowHolder.GetCurrentShadowState("test:device:child")
	assert.Equal(t, expected, currentState)
}

func TestDeleteChildShadow(t *testing.T) {
	handler := state.CreateDefaultShadowStateHandler()
	handler.Init(settings(), watermill.NopLogger{})
	shadowHolder := handler.(passthrough.ShadowStateHolder)
	payload := `{
			"state": {
				"reported": {
					"test": "value"
				}
			}
		}`

	message := &message.Message{Payload: []byte(payload)}
	message.SetContext(connector.SetTopicToCtx(message.Context(), "$aws/things/test:device/shadow/name/test:device:child/update/accepted"))

	result, err := handler.HandleMessage(message)
	assert.Nil(t, err)
	assert.Nil(t, result)

	currentState := shadowHolder.GetCurrentShadowState("test:device:child")
	assert.NotNil(t, currentState)

	message.SetContext(connector.SetTopicToCtx(message.Context(), "$aws/things/test:device/shadow/name/test:device:child/delete/accepted"))

	result, err = handler.HandleMessage(message)
	currentState = shadowHolder.GetCurrentShadowState("test:device:child")

	assert.Nil(t, currentState)
}

func settings() *config.CloudSettings {
	settings := &config.CloudSettings{}
	settings.TenantID = "test-tenant-id"
	settings.DeviceID = "test:device"
	return settings
}
