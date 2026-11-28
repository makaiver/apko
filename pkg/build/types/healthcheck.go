// Copyright 2026 Chainguard, Inc.
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"fmt"
	"slices"
	"strings"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"gopkg.in/yaml.v3"
)

// ImageHealthcheck configures a Docker-compatible container healthcheck.
type ImageHealthcheck struct {
	// Test is a list starting with CMD or CMD-SHELL, followed by the command,
	// or a single NONE to disable the check.
	Test HealthcheckTest `json:"test" yaml:"test" jsonschema:"minItems=1"`
	// Interval is the time between checks, expressed as a duration such as 30s.
	Interval string `json:"interval,omitempty" yaml:"interval,omitempty"`
	// Timeout is the maximum duration of an individual check.
	Timeout string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	// StartPeriod is the duration of the startup grace period.
	StartPeriod string `json:"start-period,omitempty" yaml:"start-period,omitempty"`
	// Retries is the number of consecutive failures before the container is unhealthy.
	Retries HealthcheckRetries `json:"retries,omitempty" yaml:"retries,omitempty" jsonschema:"minimum=0"`
}

// HealthcheckTest contains the healthcheck mode and its arguments.
type HealthcheckTest []string

// JSONSchemaAlias keeps the field's minItems constraint on the array schema.
func (HealthcheckTest) JSONSchemaAlias() any { return []string(nil) }

func (t *HealthcheckTest) UnmarshalYAML(node *yaml.Node) error {
	// yaml.v3 skips null entries when decoding []string, shifting later arguments.
	if node.Kind == yaml.SequenceNode {
		for i, arg := range node.Content {
			if arg.ShortTag() == "!!null" {
				return fmt.Errorf("healthcheck.test[%d] cannot be null", i)
			}
		}
	}
	return node.Decode((*[]string)(t))
}

// HealthcheckRetries is an integer retry count that rejects floating-point YAML values.
type HealthcheckRetries int

func (r *HealthcheckRetries) UnmarshalYAML(node *yaml.Node) error {
	if node.ShortTag() != "!!int" {
		return fmt.Errorf("healthcheck.retries must be an integer")
	}
	var count int
	if err := node.Decode(&count); err != nil {
		return fmt.Errorf("invalid healthcheck.retries: %w", err)
	}
	*r = HealthcheckRetries(count)
	return nil
}

// ToHealthConfig validates the healthcheck and converts it to image metadata.
// A nil healthcheck leaves the image's existing healthcheck intact.
func (h *ImageHealthcheck) ToHealthConfig() (*v1.HealthConfig, error) {
	if h == nil {
		return nil, nil
	}
	if len(h.Test) == 0 {
		return nil, fmt.Errorf("healthcheck.test is required")
	}
	switch h.Test[0] {
	case "CMD":
		if len(h.Test) < 2 || h.Test[1] == "" {
			return nil, fmt.Errorf("healthcheck.test CMD requires an executable")
		}
	case "CMD-SHELL":
		if len(h.Test) != 2 || strings.TrimSpace(h.Test[1]) == "" {
			return nil, fmt.Errorf("healthcheck.test CMD-SHELL requires one non-empty command string")
		}
	case "NONE":
		if len(h.Test) != 1 {
			return nil, fmt.Errorf("healthcheck.test NONE takes no arguments")
		}
	default:
		return nil, fmt.Errorf("healthcheck.test mode %q must be CMD, CMD-SHELL, or NONE", h.Test[0])
	}
	for i, arg := range h.Test[1:] {
		if strings.ContainsRune(arg, '\x00') {
			return nil, fmt.Errorf("healthcheck.test[%d] contains a NUL byte", i+1)
		}
	}
	if h.Retries < 0 {
		return nil, fmt.Errorf("healthcheck.retries cannot be negative")
	}

	config := &v1.HealthConfig{Test: slices.Clone(h.Test), Retries: int(h.Retries)}
	for _, timing := range []struct {
		name  string
		value string
		dst   *time.Duration
	}{
		{"interval", h.Interval, &config.Interval},
		{"timeout", h.Timeout, &config.Timeout},
		{"start-period", h.StartPeriod, &config.StartPeriod},
	} {
		if timing.value == "" {
			continue
		}
		duration, err := time.ParseDuration(timing.value)
		if err != nil {
			return nil, fmt.Errorf("invalid healthcheck.%s: %w", timing.name, err)
		}
		// ParseDuration truncates fractional nanoseconds. Only a duration with
		// all zero digits can use zero to select the runtime default.
		if duration < time.Millisecond && strings.ContainsAny(timing.value, "123456789") {
			return nil, fmt.Errorf("healthcheck.%s must be zero or at least 1ms", timing.name)
		}
		*timing.dst = duration
	}
	return config, nil
}
