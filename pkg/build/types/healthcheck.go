// Copyright 2026 Chainguard, Inc.
// SPDX-License-Identifier: Apache-2.0

package types

import (
	"fmt"
	"slices"
	"strings"
	"time"

	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// ImageHealthcheck configures a Docker-compatible container healthcheck.
type ImageHealthcheck struct {
	// Test is a list starting with CMD or CMD-SHELL, followed by the command,
	// or a single NONE to disable the check.
	Test []string `json:"test" yaml:"test" jsonschema:"minItems=1"`
	// Interval is the time between checks, expressed as a duration such as 30s.
	Interval string `json:"interval,omitempty" yaml:"interval,omitempty"`
	// Timeout is the maximum duration of an individual check.
	Timeout string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	// StartPeriod is the duration of the startup grace period.
	StartPeriod string `json:"start-period,omitempty" yaml:"start-period,omitempty"`
	// Retries is the number of consecutive failures before the container is unhealthy.
	Retries int `json:"retries,omitempty" yaml:"retries,omitempty" jsonschema:"minimum=0"`
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
	if h.Retries < 0 {
		return nil, fmt.Errorf("healthcheck.retries cannot be negative")
	}

	config := &v1.HealthConfig{Test: slices.Clone(h.Test), Retries: h.Retries}
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
		// Docker accepts zero for an unset duration and requires at least 1ms otherwise.
		if duration != 0 && duration < time.Millisecond {
			return nil, fmt.Errorf("healthcheck.%s must be zero or at least 1ms", timing.name)
		}
		*timing.dst = duration
	}
	return config, nil
}
