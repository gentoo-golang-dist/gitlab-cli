//go:build !integration

package cliutils

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/config"
)

func TestDetectPlatform(t *testing.T) {
	t.Parallel()

	// This test runs on the actual platform, so we verify it works correctly
	platform, err := detectPlatform()
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

func TestNormalizeArch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		goos        string
		goarch      string
		want        string
		expectError bool
	}{
		{
			name:   "amd64 on darwin",
			goos:   "darwin",
			goarch: "amd64",
			want:   "x64",
		},
		{
			name:   "amd64 on linux",
			goos:   "linux",
			goarch: "amd64",
			want:   "x64",
		},
		{
			name:   "amd64 on windows",
			goos:   "windows",
			goarch: "amd64",
			want:   "x64-baseline",
		},
		{
			name:   "arm64 on darwin",
			goos:   "darwin",
			goarch: "arm64",
			want:   "arm64",
		},
		{
			name:   "arm64 on linux",
			goos:   "linux",
			goarch: "arm64",
			want:   "arm64",
		},
		{
			name:        "arm64 on windows (not supported)",
			goos:        "windows",
			goarch:      "arm64",
			expectError: true,
		},
		{
			name:   "aarch64 on linux",
			goos:   "linux",
			goarch: "aarch64",
			want:   "arm64",
		},
		{
			name:        "unsupported architecture",
			goos:        "linux",
			goarch:      "386",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeArch(tt.goos, tt.goarch)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestIsSupportedOS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		os   string
		want bool
	}{
		{"darwin", "darwin", true},
		{"linux", "linux", true},
		{"windows", "windows", true},
		{"freebsd", "freebsd", false},
		{"openbsd", "openbsd", false},
		{"invalid", "invalid", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := isSupportedOS(tt.os)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPlatformBinaryName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		platform platform
		want     string
	}{
		{
			name:     "macOS Intel",
			platform: platform{os: "darwin", arch: "x64"},
			want:     "duo-darwin-x64",
		},
		{
			name:     "macOS Apple Silicon",
			platform: platform{os: "darwin", arch: "arm64"},
			want:     "duo-darwin-arm64",
		},
		{
			name:     "Linux x64",
			platform: platform{os: "linux", arch: "x64"},
			want:     "duo-linux-x64",
		},
		{
			name:     "Linux ARM64",
			platform: platform{os: "linux", arch: "arm64"},
			want:     "duo-linux-arm64",
		},
		{
			name:     "Windows x64",
			platform: platform{os: "windows", arch: "x64-baseline"},
			want:     "duo-windows-x64-baseline.exe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.platform.binaryName()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPlatformInstallDir(t *testing.T) {
	t.Parallel()

	// All platforms now use config directory + bin
	expectedPath := filepath.Join(config.ConfigDir(), "bin")

	tests := []struct {
		name     string
		platform platform
	}{
		{
			name:     "macOS",
			platform: platform{os: "darwin", arch: "arm64"},
		},
		{
			name:     "Linux",
			platform: platform{os: "linux", arch: "x64"},
		},
		{
			name:     "Windows",
			platform: platform{os: "windows", arch: "x64-baseline"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.platform.installDir()
			assert.Equal(t, expectedPath, got)
		})
	}
}

func TestPlatformBinaryPath(t *testing.T) {
	t.Parallel()

	// All platforms now use config directory + bin
	configBinDir := filepath.Join(config.ConfigDir(), "bin")

	tests := []struct {
		name     string
		platform platform
		want     string
	}{
		{
			name:     "macOS",
			platform: platform{os: "darwin", arch: "arm64"},
			want:     filepath.Join(configBinDir, "duo"),
		},
		{
			name:     "Linux",
			platform: platform{os: "linux", arch: "x64"},
			want:     filepath.Join(configBinDir, "duo"),
		},
		{
			name:     "Windows",
			platform: platform{os: "windows", arch: "x64-baseline"},
			want:     filepath.Join(configBinDir, "duo.exe"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.platform.binaryPath()
			assert.Equal(t, tt.want, got)
		})
	}
}
