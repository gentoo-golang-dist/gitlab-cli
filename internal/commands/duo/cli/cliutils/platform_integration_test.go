//go:build integration

package cliutils

import (
	"errors"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectPlatform(t *testing.T) {
	t.Parallel()

	// This test runs on the actual platform, so we verify it works correctly.
	// On unsupported architectures (e.g., i686/386), detectPlatform returns
	// ErrUnsupportedPlatform — we skip only for that specific error.
	platform, err := detectPlatform()
	if errors.Is(err, ErrUnsupportedPlatform) {
		t.Skipf("skipping on unsupported platform: %v", err)
	}
	require.NoError(t, err)

	assert.NotEmpty(t, platform.os)
	assert.NotEmpty(t, platform.arch)

	// Verify the platform matches current runtime
	assert.Equal(t, runtime.GOOS, platform.os)

	// Verify architecture normalization
	switch runtime.GOARCH {
	case "amd64":
		if runtime.GOOS == "windows" {
			assert.Equal(t, "x64-baseline", platform.arch)
		} else {
			assert.Equal(t, "x64", platform.arch)
		}
	case "arm64", "aarch64":
		assert.Equal(t, "arm64", platform.arch)
	}
}
