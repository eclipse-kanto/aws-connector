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

package flags_test

import (
	"flag"
	"io"
	"os"
	"testing"

	"github.com/eclipse-kanto/aws-connector/config"
	"github.com/eclipse-kanto/aws-connector/flags"

	suiteFlags "github.com/eclipse-kanto/suite-connector/flags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersionParse(t *testing.T) {
	exitCall := false
	exit := func(_ int) {
		exitCall = true
	}

	f := flag.NewFlagSet("testing", flag.ContinueOnError)
	cmd := new(config.CloudSettings)
	flags.Add(f, cmd)

	args := []string{
		"-version",
	}

	require.NoError(t, suiteFlags.Parse(f, args, "0.0.0", exit))
	require.True(t, exitCall)
}

func TestInvalidFlag(t *testing.T) {
	f := flag.NewFlagSet("testing", flag.ContinueOnError)
	f.SetOutput(io.Discard)
	cmd := new(config.CloudSettings)
	flags.Add(f, cmd)

	args := []string{
		"-invalid",
	}

	require.Error(t, suiteFlags.Parse(f, args, "0.0.0", os.Exit))
}

func TestFlagsSet(t *testing.T) {
	f := flag.NewFlagSet("testing", flag.ContinueOnError)
	cmd := new(config.CloudSettings)
	flags.Add(f, cmd)
	suiteFlags.AddGlobal(f)

	flagNames := []string{
		"tenantId",
		"clientId",
		"configFile",
		"address",
		"caCert",
		"cert",
		"key",
		"localAddress",
		"localUsername",
		"localPassword",
		"localCACert",
		"localCert",
		"localKey",
		"logFile",
		"logLevel",
		"logFileSize",
		"logFileCount",
		"logFileMaxAge",
		"tpmDevice",
		"tpmHandle",
		"tpmKey",
		"tpmKeyPub",
		"topicFilter",
		"payloadFilters",
	}
	for _, flagName := range flagNames {
		assertFlagExists(t, flagName, f)
	}
	assertFlagNotExists(t, "messageMapperConfig", f)
}

func assertFlagExists(t *testing.T, flagName string, f *flag.FlagSet) {
	flg := f.Lookup(flagName)
	assert.NotNil(t, flg)
}

func assertFlagNotExists(t *testing.T, flagName string, f *flag.FlagSet) {
	flg := f.Lookup(flagName)
	assert.Nil(t, flg)
}
