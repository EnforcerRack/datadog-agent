package telemetry

import "github.com/DataDog/datadog-agent/pkg/config"

// IsCheckEnabled returns if we want telemetry for the given check.
// Returns true if a * is present in the telemetry.checks list.
func IsCheckEnabled(checkName string) bool {
	// by default, we don't enable telemetry for every checks stats
	if config.Datadog.IsSet("telemetry.checks") {
		for _, check := range config.Datadog.GetStringSlice("telemetry.checks") {
			if check == "*" {
				return true
			} else if check == checkName {
				return true
			}
		}
	}
	return false
}
