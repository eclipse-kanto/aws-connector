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

package config

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"regexp"
	"strings"

	"github.com/eclipse-kanto/suite-connector/config"
	"github.com/eclipse-kanto/suite-connector/logger"
	suiteUtil "github.com/eclipse-kanto/suite-connector/util"
	"github.com/pkg/errors"
)

// CloudSettings represents all configurable data that is used to setup the cloud connector.
type CloudSettings struct {
	config.LocalConnectionSettings
	config.HubConnectionSettings
	logger.LogSettings
	MessageFilterSettings
}

// MessageFilterSettings represents all configurable filters.
type MessageFilterSettings struct {
	TopicFilter          string `json:"topicFilter"`
	TopicFilterRegexp    *regexp.Regexp
	PayloadFilters       PayloadFiltersType `json:"payloadFilters"`
	PayloadFiltersRegexp []*regexp.Regexp
}

// PayloadFiltersType represents payload filters.
type PayloadFiltersType []string

func (v *PayloadFiltersType) String() string {
	if len(*v) > 0 {
		return "['" + strings.Join(*v, "', '") + "']"
	}
	return "[]"
}

// Set additional payload filter.
func (v *PayloadFiltersType) Set(value string) error {
	*v = append(*v, value)
	return nil
}

// Get payload filters.
func (v *PayloadFiltersType) Get() interface{} {
	return v
}

// DefaultSettings returns the AWS connector default settings.
func DefaultSettings() *CloudSettings {
	def := config.DefaultSettings()
	defSettings := &CloudSettings{
		LocalConnectionSettings: def.LocalConnectionSettings,
		HubConnectionSettings:   def.HubConnectionSettings,
		LogSettings:             def.LogSettings,
	}
	defSettings.CACert = "aws.crt"
	defSettings.Address = ""
	defSettings.TenantID = "default-tenant-id"
	defSettings.LogFile = "logs/aws-connector.log"
	defSettings.TopicFilter = ""
	return defSettings
}

// ReadDeviceID reads device id from PEM encoded certificate.
func (settings *CloudSettings) ReadDeviceID() error {
	raw, err := os.ReadFile(settings.Cert)
	if err != nil {
		return err
	}

	block, _ := pem.Decode(raw)
	if block == nil {
		return errors.New("empty device certificate file content")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil || cert == nil {
		return errors.Wrap(err, "error on parsing the device certificate")
	}

	settings.DeviceID = cert.Subject.CommonName
	return nil
}

// CompileFilters prepare regex filters.
func (settings *CloudSettings) CompileFilters() error {
	if len(settings.TopicFilter) > 0 {
		if exp, err := regexp.Compile(settings.TopicFilter); err == nil {
			settings.TopicFilterRegexp = exp
		} else {
			return err
		}
	}
	for _, v := range settings.PayloadFilters {
		if exp, err := regexp.Compile(v); err == nil {
			settings.PayloadFiltersRegexp = append(settings.PayloadFiltersRegexp, exp)
		} else {
			return err
		}
	}
	return nil
}

// Validate validates the settings.
func (settings *CloudSettings) Validate() error {
	if err := settings.LocalConnectionSettings.Validate(); err != nil {
		return err
	}

	if err := settings.HubConnectionSettings.Validate(); err != nil {
		return err
	}

	if err := settings.LogSettings.Validate(); err != nil {
		return err
	}

	if len(settings.CACert) > 0 && !suiteUtil.FileExists(settings.CACert) {
		return errors.New("failed to read CA certificates file")
	}
	return nil
}
