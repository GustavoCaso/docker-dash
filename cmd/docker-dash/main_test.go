package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/GustavoCaso/docker-dash/internal/config"
)

func TestValidateIntervals_InvalidUpdateCheckInterval(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		UpdateCheck: config.UpdateCheckConfig{
			Enabled:  true,
			Interval: "not-a-duration",
		},
	}

	var stderr bytes.Buffer

	err := validateIntervals(cfg, &stderr)
	if err == nil {
		t.Fatal("validateIntervals should return an error for an invalid update check interval")
	}

	if !strings.Contains(err.Error(), `invalid update check interval "not-a-duration"`) {
		t.Fatalf("unexpected error: %v", err)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output for invalid interval, got %q", stderr.String())
	}
}

func TestValidateIntervals_NonPositiveUpdateCheckIntervalDisablesChecks(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		UpdateCheck: config.UpdateCheckConfig{
			Enabled:  true,
			Interval: "0s",
		},
	}

	var stderr bytes.Buffer

	err := validateIntervals(cfg, &stderr)
	if err != nil {
		t.Fatalf("validateIntervals returned unexpected error: %v", err)
	}

	if cfg.UpdateCheck.Enabled {
		t.Fatal("validateIntervals should disable update checks for a non-positive interval")
	}

	if !strings.Contains(stderr.String(), `non-positive interval configured "0s"`) {
		t.Fatalf("expected non-positive interval warning, got %q", stderr.String())
	}
}

func TestValidateIntervals_IgnoresDisabledUpdateCheckInterval(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		UpdateCheck: config.UpdateCheckConfig{
			Enabled:  false,
			Interval: "not-a-duration",
		},
	}

	var stderr bytes.Buffer

	err := validateIntervals(cfg, &stderr)
	if err != nil {
		t.Fatalf("validateIntervals returned unexpected error: %v", err)
	}

	if stderr.Len() != 0 {
		t.Fatalf("expected no stderr output when update checks are disabled, got %q", stderr.String())
	}
}
