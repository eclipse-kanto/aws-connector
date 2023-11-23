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

package passthrough

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/eclipse-kanto/aws-connector/config"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse/ditto-clients-golang/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type DummyShadowStateHolder struct {
	shadows           map[string]interface{}
	interractionCount int
}

var shadowStateHolder = DummyShadowStateHolder{shadows: map[string]interface{}{}}

func (h DummyShadowStateHolder) GetCurrentShadowState(shadowID string) interface{} {
	h.interractionCount++
	return h.shadows[shadowID]
}

func (h DummyShadowStateHolder) add(shadowID string, currentState interface{}) {
	h.shadows[shadowID] = currentState
}

func (h *DummyShadowStateHolder) cleanup() {
	h.interractionCount = 0
	h.shadows = map[string]interface{}{}
}

func TestCreateDefaultDeviceHandler(t *testing.T) {
	messageHandler := CreateDefaultDeviceHandler(shadowStateHolder)
	require.NoError(t, messageHandler.Init(settings(), watermill.NopLogger{}))
	assert.Equal(t, "passthrough_device_handler", messageHandler.Name())
	assert.Equal(t, "event/#,e/#,telemetry/#,t/#", messageHandler.Topics())
}

func TestHandleRootFeatureEvent(t *testing.T) {
	payload := `{
		"topic":"test/device/things/twin/commands/modify",
		"headers":{
			"response-required":false
		},
		"path":"/features/test",
		"value":{
			"properties":{
				"status":200
			}
		}
	}`
	topic := "event"

	messageTopic, messagePayload := requireValidMessage(t, topic, payload)
	assert.Equal(t, "$aws/things/test:device/shadow/name/test/update", messageTopic)
	assert.Equal(t, `{"state":{"reported":{"status":200}}}`, messagePayload)
}

func TestHandleFeatureNoProperties(t *testing.T) {
	payload := `{
		"topic":"test/device/things/twin/commands/modify",
		"headers":{
			"response-required":false
		},
		"path":"/features/test",
		"value":{
			"definition":["test:Definition:1.0.0"]
		}
	}`
	topic := "event"

	messageTopic, messagePayload := requireValidMessage(t, topic, payload)
	assert.Equal(t, "$aws/things/test:device/shadow/name/test/update", messageTopic)
	assert.Equal(t, `{"state":{"reported":{"definition":["test:Definition:1.0.0"]}}}`, messagePayload)
}

func TestHandleRootThingAttributes(t *testing.T) {
	payload := `{
		"topic":"test/device/things/twin/commands/modify",
		"headers":{
			"response-required":false
		},
		"path":"/attributes/test",
		"value":200
	}`
	topic := "event"

	messageTopic, messagePayload := requireValidMessage(t, topic, payload)
	assert.Equal(t, "$aws/things/test:device/shadow/update", messageTopic)
	assert.Equal(t, `{"state":{"reported":{"test":200}}}`, messagePayload)
}

func TestHandleChildFeatureEvent(t *testing.T) {
	payload := `{
		"topic":"test/device:edge:containers/things/twin/commands/modify",
		"headers":{
			"response-required":false
		},
		"path":"/features/test/properties/status",
		"value":200
	}`
	topic := "event"

	messageTopic, messagePayload := requireValidMessage(t, topic, payload)
	assert.Equal(t, "$aws/things/test:device/shadow/name/edge:containers:test/update", messageTopic)
	assert.Equal(t, `{"state":{"reported":{"status":200}}}`, messagePayload)
}

func TestHandleChildThingAttributes(t *testing.T) {
	payload := `{
		"topic":"test/device:edge:containers/things/twin/commands/modify",
		"headers":{
			"response-required":false
		},
		"path":"/attributes/test",
		"value":200
	}`
	topic := "event"

	messageTopic, messagePayload := requireValidMessage(t, topic, payload)
	assert.Equal(t, "$aws/things/test:device/shadow/name/edge:containers/update", messageTopic)
	assert.Equal(t, `{"state":{"reported":{"test":200}}}`, messagePayload)
}

func TestNoInteractionWithShadowStateHandlerWhenMerge(t *testing.T) {
	payload := `{
		"topic":"test/device/things/twin/commands/merge",
		"path":"/features/test/properties",
		"value":{
				"status":200
		}
	}`

	requireValidMessage(t, "event", payload)

	assert.Equal(t, 0, shadowStateHolder.interractionCount)
}

func TestUpdateRootFeaturePropertiesModify(t *testing.T) {
	payload := `{
		"topic":"test/device/things/twin/commands/modify",
		"path":"/features/test/properties",
		"value":{
				"status":200
		}
	}`

	currentState := map[string]interface{}{
		"status":   100,
		"obsolete": "true",
	}

	expected := `{"state":{"reported":{"obsolete":null,"status":200}}}`

	assertMergeWithCurrentState(t, currentState, payload, "test", expected)
}

func TestUpdateSingleRootFeatureProperty(t *testing.T) {
	payload := `{
		"topic":"test/device/things/twin/commands/modify",
		"path":"/features/test/properties/status",
		"value":200
	}`

	currentState := map[string]interface{}{
		"status": 100,
	}

	expected := `{"state":{"reported":{"status":200}}}`

	assertMergeWithCurrentState(t, currentState, payload, "test", expected)
}

func TestDeleteObsoleteRootFeatureProperty(t *testing.T) {
	payload := `{
		"topic":"test/device/things/twin/commands/modify",
		"path":"/features/test",
		"value":{
			"properties":{
				"status":200
			}
		}
	}`

	currentState := map[string]interface{}{
		"status":   100,
		"obsolete": "true",
	}

	expected := `{"state":{"reported":{"obsolete":null,"status":200}}}`

	assertMergeWithCurrentState(t, currentState, payload, "test", expected)
}

func TestPartiallyUpdateRootFeatureProperty(t *testing.T) {
	payload := `{
		"topic":"test/device/things/twin/commands/modify",
		"path":"/features/test/properties/status/code",
		"value":200
	}`

	currentState := map[string]interface{}{
		"status":   100,
		"obsolete": "false",
	}

	expected := `{"state":{"reported":{"status":{"code":200}}}}`

	assertMergeWithCurrentState(t, currentState, payload, "test", expected)
}

func TestPartiallyUpdateRootFeatureComplexSubProperty(t *testing.T) {
	payload := `{
		"topic":"test/device/things/twin/commands/modify",
		"path":"/features/test/properties/status/error",
		"value":{
			"code": 404,
			"message": "Not Found"
		}
	}`

	currentState := map[string]interface{}{
		"status": map[string]interface{}{
			"error": map[string]interface{}{
				"code":     200,
				"message":  "No Error",
				"obsolete": true,
			},
		},
		"notObsolete": "true",
	}

	expected := `{"state":{"reported":{"status":{"error":{"code":404,"message":"Not Found","obsolete":null}}}}}`
	assertMergeWithCurrentState(t, currentState, payload, "test", expected)
}

func TestDoesNotDeleteRootFeaturePropertiesWhenPartiallyUpdatingOtherProperty(t *testing.T) {
	payload := `{
		"topic":"test/device/things/twin/commands/modify",
		"path":"/features/test/properties/status",
		"value":{
			"code": 200
		}
	}`

	currentState := map[string]interface{}{
		"status": map[string]interface{}{
			"code": 100,
		},
		"obsolete": "false",
	}

	expected := `{"state":{"reported":{"status":{"code":200}}}}`
	assertMergeWithCurrentState(t, currentState, payload, "test", expected)
}

func TestDeleteObsoleteRootFeatureNestedMapProperty(t *testing.T) {
	payload := `{
		"topic":"test/device/things/twin/commands/modify",
		"path":"/features/test",
		"value":{
			"properties":{
				"nested": {
					"status": 200
				}
			}
		}
	}`

	currentState := map[string]interface{}{
		"nested": map[string]interface{}{
			"status":   100,
			"obsolete": "true",
		},
	}

	expected := `{"state":{"reported":{"nested":{"obsolete":null,"status":200}}}}`

	assertMergeWithCurrentState(t, currentState, payload, "test", expected)
}

func TestDeleteObsoleteRootFeatureNestedArrayProperty(t *testing.T) {
	payload := `{
		"topic":"test/device/things/twin/commands/modify",
		"path":"/features/test",
		"value":{
			"properties":{
				"nested": [
					"firstElementNew",
					{
						"status": 200
					}
				]
			}
		}
	}`

	currentState := map[string]interface{}{
		"nested": []interface{}{
			"firstElementOld",
			map[string]interface{}{
				"status":   100,
				"obsolete": "true",
			},
		},
	}

	expected := `{"state":{"reported":{"nested":["firstElementNew",{"obsolete":null,"status":200}]}}}`

	assertMergeWithCurrentState(t, currentState, payload, "test", expected)
}

func TestDeleteObsoleteElementFromArrayInFeatureWithoutProperties(t *testing.T) {
	payload := `{
		"topic":"test/device/things/twin/commands/modify",
		"path":"/features/test",
		"value":{
			"definition":["test:Definition:1.0.0"]
		}
	}`

	currentState := map[string]interface{}{
		"definition": []string{"old:Definition:1.0.0", "obsolete:1.0.0"},
	}

	expected := `{"state":{"reported":{"definition":["test:Definition:1.0.0"]}}}`

	assertMergeWithCurrentState(t, currentState, payload, "test", expected)
}

func TestDoNotRemoveRootThingAttributeWhenUpdatingAnotherAttribute(t *testing.T) {
	payload := `{
		"topic":"test/device/things/twin/commands/modify",
		"path":"/attributes/test",
		"value":200
	}`

	currentState := map[string]interface{}{
		"test":     100,
		"obsolete": "false",
	}

	expected := `{"state":{"reported":{"test":200}}}`

	assertMergeWithCurrentState(t, currentState, payload, "test:device", expected)
}

func TestDoesNotRemoveChildFeaturePropertyWhenUpdatingOtherProperty(t *testing.T) {
	payload := `{
		"topic":"test/device:edge:containers/things/twin/commands/modify",
		"path":"/features/test/properties/status",
		"value":200
	}`

	currentState := map[string]interface{}{
		"status":   100,
		"obsolete": "false",
	}

	expected := `{"state":{"reported":{"status":200}}}`

	assertMergeWithCurrentState(t, currentState, payload, "edge:containers:test", expected)
}

func TestDoNotRemoveChildThingAttributeWhenPartiallyUpdatingAnotherAttribute(t *testing.T) {
	payload := `{
		"topic":"test/device:edge:containers/things/twin/commands/modify",
		"path":"/attributes/test",
		"value":200
	}`

	currentState := map[string]interface{}{
		"test":     100,
		"obsolete": "false",
	}

	expected := `{"state":{"reported":{"test":200}}}`

	assertMergeWithCurrentState(t, currentState, payload, "edge:containers", expected)
}

func TestPayloadFilter(t *testing.T) {
	payload := `{
		"topic":"test/device:edge:containers/things/twin/commands/modify",
		"headers":{
			"response-required":false
		},
		"path":"/features/test/properties",
		"value":{
			"code":[
				{"value":"test"},
				{"keep":201},
				{"unwanted":500}
			],
			"status":200,
			"unwanted":1234
		}
	}`
	topic := "event"

	settings := filters(t, "", ".*/unwanted$", ".*/0/value$")
	messageTopic, messagePayload := requireValidMessageSettings(t, settings, topic, payload)
	assert.Equal(t, "$aws/things/test:device/shadow/name/edge:containers:test/update", messageTopic)
	assert.Equal(t, `{"state":{"reported":{"code":[{"keep":201}],"status":200}}}`, messagePayload)
}

func TestPayloadFilterEntireValue(t *testing.T) {
	payload := `{
		"topic":"test/device:edge:containers/things/twin/commands/modify",
		"headers":{
			"response-required":false
		},
		"path":"/features/test/properties",
		"value":{
			"status":200
		}
	}`
	topic := "event"

	settings := filters(t, "", ".*")
	messageHandler := CreateDefaultDeviceHandler(shadowStateHolder)
	require.NoError(t, messageHandler.Init(settings, watermill.NopLogger{}))

	message := &message.Message{Payload: []byte(payload)}
	message.SetContext(connector.SetTopicToCtx(message.Context(), topic))

	messages, err := messageHandler.HandleMessage(message)
	require.NoError(t, err)
	require.Equal(t, 0, len(messages))
}

func TestTopicFilter(t *testing.T) {
	payload := `{
		"topic":"test/device:edge:containers/things/twin/commands/modify",
		"headers":{
			"response-required":false
		},
		"path":"/features/test/properties",
		"value":{
			"status":200
		}
	}`
	topic := "event"

	settings := filters(t, "^test/device:edge:containers/.*")
	messageHandler := CreateDefaultDeviceHandler(shadowStateHolder)
	require.NoError(t, messageHandler.Init(settings, watermill.NopLogger{}))

	message := &message.Message{Payload: []byte(payload)}
	message.SetContext(connector.SetTopicToCtx(message.Context(), topic))

	messages, err := messageHandler.HandleMessage(message)
	require.NoError(t, err)
	require.Equal(t, 0, len(messages))
}

func TestDeleteOfInnerElements(t *testing.T) {
	// 1. Assert deletion of single attribute (root thing).
	assertDelete(t, "", "/attributes/attribute-1",
		"$aws/things/test:device/shadow/update",
		`{"state":{"reported":{"attribute-1":null}}}`)

	// 2. Assert deletion of single attribute (child thing).
	assertDelete(t, ":edge:containers", "/attributes/attribute-1",
		"$aws/things/test:device/shadow/name/edge:containers/update",
		`{"state":{"reported":{"attribute-1":null}}}`)

	// 3. Assert deletion of single feature property (root thing).
	assertDelete(t, "", "/features/feature-1/properties/property-1",
		"$aws/things/test:device/shadow/name/feature-1/update",
		`{"state":{"reported":{"property-1":null}}}`)

	// 4. Assert deletion of feature definition (root thing).
	assertDelete(t, "", "/features/feature-1/definition",
		"$aws/things/test:device/shadow/name/feature-1/update",
		`{"state":{"reported":{"definition":null}}}`)

	// 5. Assert deletion of single feature property (child thing).
	assertDelete(t, ":edge:containers", "/features/feature-1/properties/property-1",
		"$aws/things/test:device/shadow/name/edge:containers:feature-1/update",
		`{"state":{"reported":{"property-1":null}}}`)

	// 6. Assert deletion of feature definition (child thing).
	assertDelete(t, ":edge:containers", "/features/feature-1/definition",
		"$aws/things/test:device/shadow/name/edge:containers:feature-1/update",
		`{"state":{"reported":{"definition":null}}}`)
}

func TestDeleteEntireElements(t *testing.T) {
	// 1. Assert deletion of all attributes (root thing).
	assertDelete(t, "", "/attributes",
		"$aws/things/test:device/shadow/delete", "")

	// 2. Assert deletion of all attributes (child thing).
	assertDelete(t, ":edge:containers", "/attributes",
		"$aws/things/test:device/shadow/name/edge:containers/delete", "")

	// 3. Assert deletion of all attributes (root thing).
	assertDelete(t, "", "/features/feature-1",
		"$aws/things/test:device/shadow/name/feature-1/delete", "")

	// 4. Assert deletion of all attributes (child thing).
	assertDelete(t, ":edge:containers", "/features/feature-1",
		"$aws/things/test:device/shadow/name/edge:containers:feature-1/delete", "")
}

func assertMergeWithCurrentState(t *testing.T, currentState map[string]interface{}, payload string, shadowId string, expectedPayload string) {
	shadowStateHolder.add(shadowId, currentState)

	_, messagePayload := requireValidMessage(t, "event", payload)
	assert.Equal(t, expectedPayload, messagePayload)

	shadowStateHolder.cleanup()

}

func TestEvents(t *testing.T) {
	assertEvent(t, "event", "e")
	assertEvent(t, "event", "event")
	assertEvent(t, "event", "event/test-tenant-id/test:device")
	assertEvent(t, "telemetry", "t")
	assertEvent(t, "telemetry", "telemetry")
	assertEvent(t, "telemetry", "telemetry/test-tenant-id/test:device")
}

func assertEvent(t *testing.T, prefix string, topic string) {
	payload := `{
		"topic":"test/device/things/live/messages/heatUp",
		"path":"/inbox/messages/heatUp",
		"value":47
	}`

	messageTopic, messagePayload := requireValidMessage(t, topic, payload)
	assert.Equal(t, fmt.Sprintf("%s/test-tenant-id/test:device", prefix), messageTopic)

	env := &protocol.Envelope{Headers: protocol.NewHeaders()}
	require.NoError(t, json.Unmarshal([]byte(messagePayload), &env))
	assert.Equal(t, "test/device/things/live/messages/heatUp", env.Topic.String())
	assert.Equal(t, "/inbox/messages/heatUp", env.Path)
	assert.Equal(t, float64(47), env.Value)
	assert.NotEmpty(t, env.Headers.Generic("x-timestamp"))
}

func assertDelete(t *testing.T, child string, path string, expectedTopic string, expectedPayload string) {
	payload := fmt.Sprintf(`{
		"topic":"test/device%s/things/twin/commands/delete",
		"headers":{
			"response-required":false
		},
		"path":"%s"
	}`, child, path)
	topic := "event"

	messageTopic, messagePayload := requireValidMessage(t, topic, payload)
	assert.Equal(t, expectedTopic, messageTopic)
	assert.Equal(t, expectedPayload, messagePayload)
}

func requireValidMessage(t *testing.T, topic string, payload string) (string, string) {
	return requireValidMessageSettings(t, settings(), topic, payload)
}

func requireValidMessageSettings(t *testing.T, settings *config.CloudSettings, topic string, payload string) (string, string) {
	messageHandler := CreateDefaultDeviceHandler(shadowStateHolder)
	require.NoError(t, messageHandler.Init(settings, watermill.NopLogger{}))

	message := &message.Message{Payload: []byte(payload)}
	message.SetContext(connector.SetTopicToCtx(message.Context(), topic))

	messages, err := messageHandler.HandleMessage(message)
	require.NoError(t, err)
	require.Equal(t, 1, len(messages))

	messageTopic, ok := connector.TopicFromCtx(messages[0].Context())
	require.True(t, ok)
	return messageTopic, string(messages[0].Payload)
}

func settings() *config.CloudSettings {
	settings := &config.CloudSettings{}
	settings.TenantID = "test-tenant-id"
	settings.DeviceID = "test:device"
	return settings
}

func filters(t *testing.T, topic string, payload ...string) *config.CloudSettings {
	settings := settings()
	if len(topic) > 0 {
		settings.TopicFilter = topic
	}
	if len(payload) > 0 {
		settings.PayloadFilters = payload
	}
	require.NoError(t, settings.CompileFilters())
	return settings
}
