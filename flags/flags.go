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

package flags

import (
	"flag"

	"github.com/eclipse-kanto/aws-connector/config"

	"github.com/eclipse-kanto/suite-connector/flags"
)

// Add the cloud connector flags and uses the given settings structure to store the values of the flags.
func Add(f *flag.FlagSet, settings *config.CloudSettings) {
	def := config.DefaultSettings()
	flags.AddLocalBroker(f, &settings.LocalConnectionSettings, &def.LocalConnectionSettings)
	flags.AddTLS(f, &settings.TLSSettings, &def.TLSSettings)
	flags.AddLog(f, &settings.LogSettings, &def.LogSettings)

	f.Var(flags.NewURLV(&settings.Address, def.Address), "address", "Remote endpoint `url`")
	f.StringVar(&settings.ClientID, "clientId", def.ClientID, "Remote client `ID`")
	f.StringVar(&settings.TenantID, "tenantId", def.TenantID, "Tenant `ID`")
	f.StringVar(&settings.TopicFilter, "topicFilter", def.TopicFilter, "Regex filter used to block incoming messages by their topic")
	f.Var(&settings.PayloadFilters, "payloadFilters", "Regex filters used to exclude parts of the incoming messages payload")
}
