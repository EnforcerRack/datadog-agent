//go:generate go run github.com/mailru/easyjson/easyjson -gen_build_flags=-mod=mod -no_std_marshalers $GOFILE

// Unless explicitly stated otherwise all files in this repository are licensed
// under the Apache License Version 2.0.
// This product includes software developed at Datadog (https://www.datadoghq.com/).
// Copyright 2016-present Datadog, Inc.

package module

// AgentContext serializes the agent context to JSON
// easyjson:json
type AgentContext struct {
	RuleID        string `json:"rule_id"`
	RuleVersion   string `json:"rule_version,omitempty"`
	PolicyName    string `json:"policy_name,omitempty"`
	PolicyVersion string `json:"policy_version,omitempty"`
	Version       string `json:"version,omitempty"`
}

// BackendEvent - Rule event wrapper used to send an event to the backend
// easyjson:json
type BackendEvent struct {
	AgentContext `json:"agent"`
	Title        string `json:"title"`
}
