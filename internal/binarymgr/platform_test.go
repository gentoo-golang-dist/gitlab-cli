//go:build !integration

package binarymgr

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/config"
)

func TestDetectPlatform_unsupportedOS(t *testing.T) {
	t.Parallel()

	spec := testSpec()
	spec.SupportedOS = []string{"plan9"} // none of the runtime OSes match
	_, err := detectPlatform(spec)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedPlatform))
}

func TestDetectPlatform_archNormalizerError(t *testing.T) {
	t.Parallel()

	spec := testSpec()
	spec.NormalizeArch = func(string, string) (string, error) {
		return "", ErrUnsupportedPlatform
	}
	// Force one of the supported OSes to make sure we hit the arch path.
	spec.SupportedOS = []string{"darwin", "linux", "windows"}
	_, err := detectPlatform(spec)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrUnsupportedPlatform))
}

func TestInstallDir_isUnderConfigDir(t *testing.T) {
	t.Parallel()
	assert.Equal(t, filepath.Join(config.ConfigDir(), "bin"), installDir())
}

func TestBinaryPath_usesInstalledName(t *testing.T) {
	t.Parallel()

	spec := testSpec()
	spec.InstalledName = func(os string) string {
		if os == "windows" {
			return "test.exe"
		}
		return "test"
	}

	assert.Equal(t, filepath.Join(installDir(), "test"), binaryPath(spec, Platform{OS: "darwin", Arch: "arm64"}))
	assert.Equal(t, filepath.Join(installDir(), "test"), binaryPath(spec, Platform{OS: "linux", Arch: "x64"}))
	assert.Equal(t, filepath.Join(installDir(), "test.exe"), binaryPath(spec, Platform{OS: "windows", Arch: "x64"}))
}

func TestAssetName_usesSpecCallback(t *testing.T) {
	t.Parallel()

	spec := testSpec()
	spec.AssetName = func(os, arch string) string {
		return "custom-" + os + "-" + arch + ".tar.gz"
	}

	assert.Equal(t, "custom-linux-x64.tar.gz", assetName(spec, Platform{OS: "linux", Arch: "x64"}))
}
