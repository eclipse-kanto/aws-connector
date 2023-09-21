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
	"flag"
	"os"
	"testing"

	suiteConfig "github.com/eclipse-kanto/suite-connector/config"
	suiteFlags "github.com/eclipse-kanto/suite-connector/flags"
	"github.com/eclipse-kanto/suite-connector/logger"
	"github.com/eclipse-kanto/suite-connector/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testConfig     = "testdata/config.json"
	testCert       = "testdata/certificate.pem"
	testPrivateKey = "testdata/private.key"
)

func temporary(t *testing.T, path string, content string) func() {
	err := os.WriteFile(path, []byte(content), os.ModePerm)
	require.NoError(t, err)
	require.True(t, util.FileExists(path))
	return func() {
		os.Remove(path)
	}
}

func TestPayloadFilter(t *testing.T) {
	f := flag.NewFlagSet("testing", flag.ContinueOnError)
	cmd := new(CloudSettings)
	f.Var(&cmd.PayloadFilters, "payloadFilters", "")

	args := []string{
		"-payloadFilters",
		".*",
		"-payloadFilters",
		"test",
	}

	require.NoError(t, suiteFlags.Parse(f, args, "0.0.0", os.Exit))
	require.NotEmpty(t, cmd.PayloadFilters)
	require.NoError(t, cmd.CompileFilters())
	require.NotEmpty(t, cmd.PayloadFiltersRegexp)
	require.NotNil(t, cmd.PayloadFilters.Get())
	require.Equal(t, cmd.PayloadFilters.String(), "['.*', 'test']")
}

func TestPayloadFilterInvalid(t *testing.T) {
	settings := new(CloudSettings)
	settings.PayloadFilters = append(settings.PayloadFilters, "[")

	require.Error(t, settings.CompileFilters())
	require.Empty(t, settings.PayloadFiltersRegexp)
}

func TestTopicFilter(t *testing.T) {
	settings := new(CloudSettings)
	settings.TopicFilter = "test"

	require.NoError(t, settings.CompileFilters())
	require.NotEmpty(t, settings.TopicFilterRegexp)
}

func TestTopicFilterInvalid(t *testing.T) {
	settings := new(CloudSettings)
	settings.TopicFilter = "["

	require.Error(t, settings.CompileFilters())
	require.Empty(t, settings.TopicFilterRegexp)
}

func TestConfigEmpty(t *testing.T) {
	configPath := "configEmpty.json"
	defer temporary(t, configPath, "")()
	cmd := new(CloudSettings)
	assert.NoError(t, suiteConfig.ReadConfig(configPath, cmd))
}

func TestConfigInvalid(t *testing.T) {
	settings := DefaultSettings()
	assert.Error(t, suiteConfig.ReadConfig("settings_test.go", settings))
	assert.Error(t, suiteConfig.ReadConfig("settings_test.go", nil))

	assert.Error(t, settings.Validate(), "Expected - device ID or pattern is missing")

	assert.Error(t, settings.ReadDeviceID(), "Expected - no such file or directory")

	settings.Cert = testConfig
	assert.Error(t, settings.ReadDeviceID(), "Expected - empty device certificate file content")

	settings.Cert = testPrivateKey
	assert.Error(t, settings.ReadDeviceID(), "Expected - error on parsing the device certificate")

	settings.Cert = testCert
	assert.NoError(t, settings.ReadDeviceID())

	assert.Error(t, settings.Validate(), "Expected - remote broker address is missing")

	settings.Address = "tls://amazonaws.com:8883"
	settings.LogFileCount = 0
	assert.Error(t, settings.Validate(), "Expected - logFileCount <= 0")

	settings.LogFileCount = 1
	settings.LocalAddress = ""
	assert.Error(t, settings.Validate(), "Expected - local address is missing")

	settings.LocalAddress = "tcp://localhost:1883"
	settings.CACert = "missing.crt"
	assert.Error(t, settings.Validate(), "Expected - failed to read CA certificates file")
}

func TestConfig(t *testing.T) {
	expSettings := DefaultSettings()
	expSettings.CACert = ""
	expSettings.Cert = testCert
	expSettings.Address = "tls://amazonaws.com:8883"
	expSettings.LocalUsername = "localUsername_config"
	expSettings.LocalPassword = "localPassword_config"
	expSettings.LogFile = "logFile_config"
	expSettings.LogLevel = logger.DEBUG
	expSettings.PayloadFilters = append(expSettings.PayloadFilters, ".*", "test")
	expSettings.TopicFilter = "test"

	settings := DefaultSettings()
	require.NoError(t, suiteConfig.ReadConfig(testConfig, settings))
	assert.Equal(t, expSettings, settings)

	require.NoError(t, settings.ReadDeviceID())
	assert.Equal(t, "test:device", settings.DeviceID)
	assert.NoError(t, settings.Validate())
}

func TestDefaults(t *testing.T) {
	settings := DefaultSettings()
	assert.Error(t, settings.Validate())

	assert.Equal(t, "default-tenant-id", settings.TenantID)
	assert.Empty(t, settings.Address)

	defConnectorSettings := suiteConfig.DefaultSettings()
	assert.Equal(t, defConnectorSettings.LocalConnectionSettings, settings.LocalConnectionSettings)

	defTLSSettings := suiteConfig.TLSSettings{
		CACert: "aws.crt",
	}
	assert.Equal(t, defTLSSettings, settings.TLSSettings)

	defLogSettings := defConnectorSettings.LogSettings
	defLogSettings.LogFile = "logs/aws-connector.log"
	assert.Equal(t, defLogSettings, settings.LogSettings)
}
