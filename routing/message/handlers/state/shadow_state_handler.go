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
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/eclipse-kanto/aws-connector/config"
	"github.com/eclipse-kanto/aws-connector/routing/message/handlers"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/eclipse-kanto/suite-connector/connector"
)

const topicBaseTemplate = "$aws/things/%s/shadow"
const updateSuffix = "/update/accepted"
const deleteSuffix = "/delete/accepted"
const namedShadowAdditionTopicTemplate = "/name/+"

var shadows = make(map[string]interface{})

type shadowStateHandler struct {
	tenantID string
	deviceID string
	logger   watermill.LoggerAdapter
	topics   string
}

// CreateDefaultShadowStateHandler instantiates a new shadow state handler that
// receives update and delete accepted messages from aws and keeps track of the last known shadow state
func CreateDefaultShadowStateHandler() handlers.MessageHandler {
	return &shadowStateHandler{}
}

// Init gets the device ID that is needed to build the topics AWS IoT Hub to subscribe to.
func (h *shadowStateHandler) Init(settings *config.CloudSettings, logger watermill.LoggerAdapter) error {
	h.tenantID = settings.TenantID
	h.deviceID = settings.DeviceID
	h.logger = logger

	topicBase := fmt.Sprintf(topicBaseTemplate, settings.DeviceID)
	rootShadowUpdatedTopic := fmt.Sprint(topicBase, updateSuffix)
	rootShadowDeletedTopic := fmt.Sprint(topicBase, deleteSuffix)
	childShadowUpdatedTopic := fmt.Sprint(topicBase, namedShadowAdditionTopicTemplate, updateSuffix)
	childShadowDeletedTopic := fmt.Sprint(topicBase, namedShadowAdditionTopicTemplate, deleteSuffix)

	h.topics = strings.Join([]string{rootShadowUpdatedTopic, rootShadowDeletedTopic, childShadowUpdatedTopic, childShadowDeletedTopic}, ",")

	return nil
}

// HandleMessage processes an AWS shadow update/accepted or delete/accepted message.
// In case of update/accepted the current state of the shadow is replaced with the new one.
// In case of update/deleted the shadow state is deleted.
// This handler provides no messages and returns nil value for the message.Message array.
func (h *shadowStateHandler) HandleMessage(message *message.Message) ([]*message.Message, error) {
	topic, ok := connector.TopicFromCtx(message.Context())

	if !ok {
		h.debug("topic missing", nil)
		return nil, errors.New("No topic in context")
	}
	shadowID := h.getShadowID(topic)

	if strings.HasSuffix(topic, "/delete/accepted") {
		delete(shadows, shadowID)
		return nil, nil
	}

	var payload interface{}
	if err := json.Unmarshal(message.Payload, &payload); err != nil {
		h.debug("Could not parse message.", map[string]interface{}{"payload": string(message.Payload)})
		return nil, errors.New("Invalid json payload")
	}

	reported, found := getReportedState(payload)
	if !found {
		h.debug("Reported state missing", map[string]interface{}{"payload": string(message.Payload)})
		return nil, errors.New("Invalid Payload structure")
	}

	shadows[shadowID] = reported

	return nil, nil
}

func getReportedState(payload interface{}) (interface{}, bool) {
	state, found := find("state", payload)
	if !found {
		return nil, false
	}

	return find("reported", state)
}

func find(name string, values interface{}) (interface{}, bool) {
	valuesMap, ok := values.(map[string]interface{})
	if !ok {
		return nil, false
	}

	result, found := valuesMap[name]

	if !found || result == nil {
		return nil, false
	}

	return result, true
}

func (h shadowStateHandler) getShadowID(topic string) string {
	const shadowIDIndex = 5

	if !strings.Contains(topic, "/name/") {
		return h.deviceID
	}

	return strings.Split(topic, "/")[shadowIDIndex]
}

// Name returns the name of the message handler.
func (h shadowStateHandler) Name() string {
	return "shadow_state_handler"
}

// Topics returns a comma separated list of AWS topics to subscribe to.
func (h shadowStateHandler) Topics() string {
	return h.topics
}

// GetCurrentShadowState returns the last known state of the shadow with the given id.
// The root shadow state is kept under <DEVICVE_ID>.
// If no shadow state for the given id is available a nil value is returned.
func (h shadowStateHandler) GetCurrentShadowState(shadowID string) interface{} {
	return shadows[shadowID]
}

func (h *shadowStateHandler) debug(msg string, fields map[string]interface{}) {
	logFields := watermill.LogFields{"handler_name": h.Name()}
	for k, v := range fields {
		logFields[k] = v
	}
	h.logger.Debug(msg, logFields)
}
