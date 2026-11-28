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
		{"empty shell command", types.ImageHealthcheck{Test: []string{"CMD-SHELL", " "}}, "CMD-SHELL requires one non-empty command"},
		{"multiple shell arguments", types.ImageHealthcheck{Test: []string{"CMD-SHELL", "true", "false"}}, "CMD-SHELL requires one non-empty command"},
		{"disabled with arguments", types.ImageHealthcheck{Test: []string{"NONE", "true"}}, "NONE takes no arguments"},
		{"invalid interval", types.ImageHealthcheck{Test: []string{"CMD", "/bin/true"}, Interval: "30"}, "invalid healthcheck.interval"},
		{"negative timeout", types.ImageHealthcheck{Test: []string{"CMD", "/bin/true"}, Timeout: "-1s"}, "healthcheck.timeout must be zero or at least 1ms"},
		{"sub-millisecond start period", types.ImageHealthcheck{Test: []string{"CMD", "/bin/true"}, StartPeriod: "1ns"}, "healthcheck.start-period must be zero or at least 1ms"},
		{"negative retries", types.ImageHealthcheck{Test: []string{"CMD", "/bin/true"}, Retries: -1}, "healthcheck.retries cannot be negative"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ic := types.ImageConfiguration{Healthcheck: &tc.healthcheck}
			require.ErrorContains(t, ic.Validate(), tc.wantError)
		})
	}
}

func TestHealthcheckInclude(t *testing.T) {
	dir := t.TempDir()
	base := `healthcheck:
  test: [CMD, /bin/true]
  interval: 30s
  start-period: 10s
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "base.yaml"), []byte(base), 0600))

	for _, tc := range []struct {
		name string
		yaml string
		want types.ImageHealthcheck
	}{
		{"inherit", "include: base.yaml\n", types.ImageHealthcheck{Test: []string{"CMD", "/bin/true"}, Interval: "30s", StartPeriod: "10s"}},
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
