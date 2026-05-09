//go:build integration

package binarymgr

import (
	"errors"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetectPlatform_runtime exercises platform detection against the
// running host. On unsupported architectures (e.g. 386) it skips for
// ErrUnsupportedPlatform only — any other error fails the test.
func TestDetectPlatform_runtime(t *testing.T) {
	t.Parallel()

	// Use a generous spec that mirrors duo's coverage so the test runs on
	// every supported host.
	spec := Spec{
		SupportedOS: []string{"darwin", "linux", "windows"},
		NormalizeArch: func(goos, goarch string) (string, error) {
			switch goarch {
			case "amd64":
				if goos == "windows" {
					return "x64-baseline", nil
				}
				return "x64", nil
			case "arm64", "aarch64":
				if goos == "windows" {
					return "", ErrUnsupportedPlatform
				}
				return "arm64", nil
			}
			return "", ErrUnsupportedPlatform
		},
	}

	p, err := detectPlatform(spec)
	if errors.Is(err, ErrUnsupportedPlatform) {
		t.Skipf("skipping on unsupported platform: %v", err)
	}
	require.NoError(t, err)
	assert.NotEmpty(t, p.OS)
	assert.NotEmpty(t, p.Arch)
	assert.Equal(t, runtime.GOOS, p.OS)

	switch runtime.GOARCH {
	case "amd64":
		if runtime.GOOS == "windows" {
			assert.Equal(t, "x64-baseline", p.Arch)
		} else {
			assert.Equal(t, "x64", p.Arch)
		}
	case "arm64", "aarch64":
		assert.Equal(t, "arm64", p.Arch)
	}
}
