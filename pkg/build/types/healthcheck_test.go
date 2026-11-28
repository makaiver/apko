// Copyright 2026 Chainguard, Inc.
// SPDX-License-Identifier: Apache-2.0

package types_test

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"chainguard.dev/apko/pkg/build/types"
)

func TestHealthcheckValidation(t *testing.T) {
	for _, tc := range []struct {
		name        string
		healthcheck types.ImageHealthcheck
		wantError   string
	}{
		{"missing test", types.ImageHealthcheck{}, "healthcheck.test is required"},
		{"unknown mode", types.ImageHealthcheck{Test: []string{"UNKNOWN"}}, "healthcheck.test mode"},
		{"missing executable", types.ImageHealthcheck{Test: []string{"CMD"}}, "CMD requires an executable"},
		{"empty executable", types.ImageHealthcheck{Test: []string{"CMD", ""}}, "CMD requires an executable"},
		{"NUL in exec argument", types.ImageHealthcheck{Test: []string{"CMD", "/bin/true", "arg\x00"}}, "healthcheck.test[2] contains a NUL byte"},
		{"empty shell command", types.ImageHealthcheck{Test: []string{"CMD-SHELL", " "}}, "CMD-SHELL requires one non-empty command"},
		{"multiple shell arguments", types.ImageHealthcheck{Test: []string{"CMD-SHELL", "true", "false"}}, "CMD-SHELL requires one non-empty command"},
		{"NUL in shell command", types.ImageHealthcheck{Test: []string{"CMD-SHELL", "true\x00"}}, "healthcheck.test[1] contains a NUL byte"},
		{"disabled with arguments", types.ImageHealthcheck{Test: []string{"NONE", "true"}}, "NONE takes no arguments"},
		{"invalid interval", types.ImageHealthcheck{Test: []string{"CMD", "/bin/true"}, Interval: "30"}, "invalid healthcheck.interval"},
		{"negative timeout", types.ImageHealthcheck{Test: []string{"CMD", "/bin/true"}, Timeout: "-1s"}, "healthcheck.timeout must be zero or at least 1ms"},
		{"sub-millisecond start period", types.ImageHealthcheck{Test: []string{"CMD", "/bin/true"}, StartPeriod: "1ns"}, "healthcheck.start-period must be zero or at least 1ms"},
		{"sub-nanosecond interval", types.ImageHealthcheck{Test: []string{"CMD", "/bin/true"}, Interval: "0.1ns"}, "healthcheck.interval must be zero or at least 1ms"},
		{"negative sub-nanosecond timeout", types.ImageHealthcheck{Test: []string{"CMD", "/bin/true"}, Timeout: "-0.1ns"}, "healthcheck.timeout must be zero or at least 1ms"},
		{"negative retries", types.ImageHealthcheck{Test: []string{"CMD", "/bin/true"}, Retries: -1}, "healthcheck.retries cannot be negative"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ic := types.ImageConfiguration{Healthcheck: &tc.healthcheck}
			require.ErrorContains(t, ic.Validate(), tc.wantError)
		})
	}
}

func TestHealthcheckFractionalRetries(t *testing.T) {
	for _, retries := range []string{"1.9", "-0.5"} {
		t.Run(retries, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "apko.yaml")
			config := "healthcheck:\n  test: [CMD, /bin/true]\n  retries: " + retries + "\n"
			require.NoError(t, os.WriteFile(path, []byte(config), 0600))
			var ic types.ImageConfiguration
			require.ErrorContains(t, ic.Load(t.Context(), path, nil, sha256.New()), "healthcheck.retries must be an integer")
		})
	}
}

func TestHealthcheckInclude(t *testing.T) {
	dir := t.TempDir()
	base := `healthcheck:
  test: [CMD, /bin/true]
  interval: 30s
  start-period: 10s
  retries: 3
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "base.yaml"), []byte(base), 0600))

	for _, tc := range []struct {
		name string
		yaml string
		want types.ImageHealthcheck
	}{
		{"inherit", "include: base.yaml\n", types.ImageHealthcheck{Test: []string{"CMD", "/bin/true"}, Interval: "30s", StartPeriod: "10s", Retries: 3}},
		{"disable", "include: base.yaml\nhealthcheck:\n  test: [NONE]\n", types.ImageHealthcheck{Test: []string{"NONE"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "apko.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tc.yaml), 0600))
			var ic types.ImageConfiguration
			require.NoError(t, ic.Load(t.Context(), path, []string{dir}, sha256.New()))
			require.NoError(t, ic.Validate())
			require.Equal(t, &tc.want, ic.Healthcheck)
		})
	}
}
