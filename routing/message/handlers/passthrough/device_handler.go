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
	"regexp"
	"strings"

	"github.com/eclipse-kanto/aws-connector/config"
	"github.com/eclipse-kanto/aws-connector/routing/message/handlers"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/eclipse-kanto/suite-connector/connector"
	"github.com/eclipse-kanto/suite-connector/routing"
	"github.com/eclipse/ditto-clients-golang/protocol"
)

const (
	deviceHandlerName = "passthrough_device_handler"
	topicsLocal       = "event/#,e/#,telemetry/#,t/#"

	valueAttributesTag = "attributes"
	valueFeaturesTag   = "features"
	valuePropertiesTag = "properties"
	valueDefinitionTag = "definition"
	valueStateTag      = "state"
	valueReportedTag   = "reported"

	// Used to update attributes of root thing.
	topicRootShadow = "$aws/things/%s/shadow/%s"
	// Used to update feature of root thing or attributes of child thing.
	topicNamedShadow = "$aws/things/%s/shadow/name/%s/%s"
	// Used to update feature of child thing.
	topicComplexNamedShadow = "$aws/things/%s/shadow/name/%s:%s/%s"

	topicUpdate = "update"
	topicDelete = "delete"
)

type deviceHandler struct {
	tenantID       string
	deviceID       string
	payloadFilters []*regexp.Regexp
	topicFilter    *regexp.Regexp
	logger         watermill.LoggerAdapter
	defaultHandler message.HandlerFunc
}

// CreateDefaultDeviceHandler instantiates a new passthrough handler that forwards messages received from local message broker on event and telemetry topics as device-to-cloud messages.
func CreateDefaultDeviceHandler() handlers.MessageHandler {
	return &deviceHandler{}
}

// Init gets the device ID that is needed for the message forwarding towards AWS IoT Hub.
func (h *deviceHandler) Init(settings *config.CloudSettings, logger watermill.LoggerAdapter) error {
	h.tenantID = settings.TenantID
	h.deviceID = settings.DeviceID
	h.payloadFilters = settings.PayloadFiltersRegexp
	h.topicFilter = settings.TopicFilterRegexp
	h.logger = logger
	h.defaultHandler = routing.AddTimestamp(routing.NewEventsHandler("", h.tenantID, h.deviceID))
	return nil
}

// Name returns the message handler name.
func (h *deviceHandler) Name() string {
	return deviceHandlerName
}

// Topics returns the configurable list of topics that are used for subscription.
func (h *deviceHandler) Topics() string {
	return topicsLocal
}

// Debug writes a debug entry to the log with included handler name.
func (h *deviceHandler) Debug(msg string, fields map[string]interface{}) {
	logFields := watermill.LogFields{"handler_name": h.Name()}
	for k, v := range fields {
		logFields[k] = v
	}
	h.logger.Debug(msg, logFields)
}

// HandleMessage creates a new message with the same payload as the incoming message and sets the correct topic so that the message can be forwarded to AWS IoT Hub
func (h *deviceHandler) HandleMessage(msg *message.Message) ([]*message.Message, error) {
	h.Debug("Handle message", map[string]interface{}{"payload": string(msg.Payload)})
	// Parse message payload (JSON)
	env := &protocol.Envelope{Headers: protocol.NewHeaders()}
	if err := json.Unmarshal(msg.Payload, &env); err == nil {
		// Convert incoming message to shadow messages (if needed)
		if messages, ok := h.toShadowMessages(env); ok {
			return messages, nil
		}
	}
	return h.defaultHandler(msg)
}

// toShadowTopic convert Ditto topic to its corresponding device shadow topic and if its an update message.
func (h *deviceHandler) toShadowTopic(topic *protocol.Topic, featureName string, value interface{}) (res string, update bool) {
	target := topicUpdate
	if topic.Action == protocol.ActionDelete && h.isEntireShadow(value) {
		target = topicDelete
		update = false
	} else {
		update = true
	}
	topicID := fmt.Sprintf("%s:%s", topic.Namespace, topic.EntityName)

	if len(h.deviceID) == len(topicID) {
		if featureName == "" {
			// Update root thing attributes.
			return fmt.Sprintf(topicRootShadow, h.deviceID, target), update
		}
		// Update root thing feature.
		return fmt.Sprintf(topicNamedShadow, h.deviceID, featureName, target), update
	}
	if featureName == "" {
		// Update child thing attributes.
		return fmt.Sprintf(topicNamedShadow, h.deviceID, topicID[len(h.deviceID)+1:], target), update
	}
	// Update child thing feature.
	return fmt.Sprintf(topicComplexNamedShadow, h.deviceID, topicID[len(h.deviceID)+1:], featureName, target), update
}

// isDittoRequest returns true if provided message is Ditto request to the connected device.
func (h *deviceHandler) isDittoRequest(env *protocol.Envelope) bool {
	id := fmt.Sprintf("%s:%s", env.Topic.Namespace, env.Topic.EntityName)
	return env.Status == 0 && env.Topic.Group == protocol.GroupThings &&
		(id == h.deviceID || strings.HasPrefix(id, h.deviceID+":"))
}

// isShadowMessage returns true if provided message is device twin/shadow request.
func (h *deviceHandler) isShadowMessage(env *protocol.Envelope) bool {
	topic := env.Topic
	return h.isDittoRequest(env) &&
		topic.Channel == protocol.ChannelTwin && topic.Criterion == protocol.CriterionCommands &&
		(topic.Action == protocol.ActionCreate || topic.Action == protocol.ActionModify ||
			topic.Action == protocol.ActionMerge || topic.Action == protocol.ActionDelete)
}

// isEntireShadow returns true if provided value is intended to update the entire shadow.
func (h *deviceHandler) isEntireShadow(value interface{}) bool {
	if v, ok := value.(map[string]interface{}); ok {
		return len(v) == 0
	}
	return true
}

// result function is used as a callback for search function.
type result func(map[string]interface{})

// search provided value for following JSON structure {"key":{}} and provide both maps to the result function.
func search(key string, value interface{}, fn result) {
	if v1, ok := value.(map[string]interface{}); ok {
		if v2, ok := v1[key]; ok {
			if v3, ok := v2.(map[string]interface{}); ok {
				fn(v3)
			} else {
				fn(nil)
			}
		}
	}
}

// integrate provided path to the provided value.
func integrate(path []string, value interface{}) interface{} {
	for i := len(path) - 1; i >= 0; i-- {
		if len(path[i]) > 0 {
			value = map[string]interface{}{path[i]: value}
		}
	}
	return value
}

// toShadowMessage convert Ditto data to device shadow message.
func (h *deviceHandler) toShadowMessage(env *protocol.Envelope, featureName string, value interface{}) *message.Message {
	topic, update := h.toShadowTopic(env.Topic, featureName, value)

	var payload message.Payload
	if update {
		// Adds shadow prefix infront: ["state", "reported"]
		value = integrate([]string{valueStateTag, valueReportedTag}, value)
		payload, _ = json.Marshal(value)
	}

	h.Debug("Send message", map[string]interface{}{"topic": topic, "payload": string(payload)})

	message := message.NewMessage(watermill.NewUUID(), payload)
	message.SetContext(connector.SetTopicToCtx(message.Context(), topic))
	return message
}

// getFeatureProperties return all feature properties, including the definition.
func (h *deviceHandler) getFeatureProperties(obj interface{}) (interface{}, bool) {
	res := map[string]interface{}{}
	if feature, ok := obj.(map[string]interface{}); ok {
		if pObj, ok := feature[valuePropertiesTag]; ok {
			if properties, ok := pObj.(map[string]interface{}); ok {
				res = properties
			}
		}

		if dObj, ok := feature[valueDefinitionTag]; ok {
			res[valueDefinitionTag] = dObj
		}
	}
	return res, len(res) > 0
}

// filterPayload removes the matched paths of provided JSON data.
func (h *deviceHandler) filterPayload(data interface{}) interface{} {
	if len(h.payloadFilters) > 0 {
		if rem, value := h._filterPayload("", data); !rem {
			return value
		}
		return nil
	}
	return data
}

// _filterPayload removes the matched paths of provided JSON data. This function will be called recursively.
func (h *deviceHandler) _filterPayload(path string, data interface{}) (bool, interface{}) {
	if mData, ok := data.(map[string]interface{}); ok {
		for name, value := range mData {
			rem, val := h._filterPayload(fmt.Sprintf("%s/%s", path, name), value)
			if rem {
				delete(mData, name)
			} else {
				mData[name] = val
			}
		}
		return len(mData) == 0, data
	} else if aData, ok := data.([]interface{}); ok {
		for index := len(aData) - 1; index >= 0; index-- {
			rem, val := h._filterPayload(fmt.Sprintf("%s/%d", path, index), aData[index])
			if rem {
				aData = append(aData[:index], aData[index+1:]...)
			} else {
				aData[index] = val
			}
		}
		return len(aData) == 0, aData
	}

	h.Debug("Found JSON path", map[string]interface{}{"path": path})
	for _, filter := range h.payloadFilters {
		if filter.MatchString(path) {
			h.Debug("Excluded JSON path", map[string]interface{}{"path": path})
			return true, data
		}
	}
	return false, data
}

// toShadowMessage convert Ditto message to device shadow message if needed.
func (h *deviceHandler) toShadowMessages(env *protocol.Envelope) ([]*message.Message, bool) {
	messages := []*message.Message{}
	if h.isShadowMessage(env) {
		if h.topicFilter != nil && h.topicFilter.MatchString(env.Topic.String()) {
			h.Debug("Excluded message with topic", map[string]interface{}{"topic": env.Topic.String()})
			return messages, true
		}

		value := integrate(strings.Split(env.Path, "/"), env.Value)
		value = h.filterPayload(value)
		if value == nil {
			return messages, true
		}

		// Prepare update messages for found attributes.
		search(valueAttributesTag, value, func(attributes map[string]interface{}) {
			messages = append(messages, h.toShadowMessage(env, "", attributes))
		})

		// Prepare update messages for every found feature.
		search(valueFeaturesTag, value, func(features map[string]interface{}) {
			for featureName, feature := range features {
				if feature == nil {
					messages = append(messages, h.toShadowMessage(env, featureName, nil))
				} else if properties, ok := h.getFeatureProperties(feature); ok {
					messages = append(messages, h.toShadowMessage(env, featureName, properties))
				}
			}
		})

		return messages, true
	}
	return messages, false
}
