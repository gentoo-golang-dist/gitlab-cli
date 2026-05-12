//go:build !integration

package local

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/binarymgr"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmd_Structure(t *testing.T) {
	t.Parallel()

	ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(false))
	factory := cmdtest.NewTestFactory(ios)
	cmd := NewCmd(factory)

	assert.True(t, cmd.DisableFlagParsing, "DisableFlagParsing should be enabled for transparent pass-through")
	assert.Nil(t, cmd.Args, "Args should be nil to accept any arguments")
	assert.NotNil(t, cmd.RunE, "RunE should be set")

	assert.NotNil(t, cmd.Flags().Lookup("install"), "--install flag should be registered")
	assert.NotNil(t, cmd.Flags().Lookup("update"), "--update flag should be registered")
	yesFlag := cmd.Flags().Lookup("yes")
	assert.NotNil(t, yesFlag, "--yes flag should be registered")
	assert.Equal(t, "y", yesFlag.Shorthand, "--yes should have -y shorthand")
}

func TestSpec_Wiring(t *testing.T) {
	t.Parallel()

	s := Spec()
	assert.Equal(t, "Orbit local CLI", s.DisplayName)
	assert.Equal(t, "77960826", s.ProjectID)
	assert.Equal(t, "orbit-local", s.PackageName)
	assert.Equal(t, "orbit_local", s.ConfigPrefix)
	assert.Equal(t, "GLAB_ORBIT_LOCAL", s.EnvVarPrefix)
	assert.Zero(t, s.MaxCompatibleMajor, "Orbit is pre-1.0; major-version cap should be uncapped")
	assert.ElementsMatch(t, []string{"darwin", "linux"}, s.SupportedOS)
	assert.NotNil(t, s.Extract, "Orbit ships tarballs and requires an Extractor")
}

func TestOrbitNormalizeArch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		goos        string
		goarch      string
		want        string
		expectError bool
	}{
		{name: "amd64 darwin", goos: "darwin", goarch: "amd64", want: "x86_64"},
		{name: "amd64 linux", goos: "linux", goarch: "amd64", want: "x86_64"},
		{name: "arm64 darwin", goos: "darwin", goarch: "arm64", want: "aarch64"},
		{name: "arm64 linux", goos: "linux", goarch: "arm64", want: "aarch64"},
		{name: "aarch64 alias", goos: "linux", goarch: "aarch64", want: "aarch64"},
		{name: "amd64 windows unsupported", goos: "windows", goarch: "amd64", expectError: true},
		{name: "arm64 windows unsupported", goos: "windows", goarch: "arm64", expectError: true},
		{name: "unsupported arch", goos: "linux", goarch: "386", expectError: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := orbitNormalizeArch(tc.goos, tc.goarch)
			if tc.expectError {
				require.Error(t, err)
				assert.True(t, errors.Is(err, binarymgr.ErrUnsupportedPlatform))
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestOrbitAssetName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "orbit-local-darwin-aarch64.tar.gz", orbitAssetName("darwin", "aarch64"))
	assert.Equal(t, "orbit-local-darwin-x86_64.tar.gz", orbitAssetName("darwin", "x86_64"))
	assert.Equal(t, "orbit-local-linux-aarch64.tar.gz", orbitAssetName("linux", "aarch64"))
	assert.Equal(t, "orbit-local-linux-x86_64.tar.gz", orbitAssetName("linux", "x86_64"))
}

func TestOrbitInstalledName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "orbit", orbitInstalledName("darwin"))
	assert.Equal(t, "orbit", orbitInstalledName("linux"))
}

func TestRunWithCustomPath_Validation(t *testing.T) {
	if _, err := binarymgr.ManagedBinaryPath(Spec()); errors.Is(err, binarymgr.ErrUnsupportedPlatform) {
		t.Skipf("skipping on unsupported platform: %v", err)
	}

	t.Run("non-existent path returns clear error", func(t *testing.T) {
		ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(false))
		factory := cmdtest.NewTestFactory(ios)

		t.Setenv("GLAB_ORBIT_LOCAL_BINARY_PATH", "/nonexistent/path/to/orbit")
		runner := newRunner(factory.IO(), factory.Config(), Spec())
		err := runner.Run(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "GLAB_ORBIT_LOCAL_BINARY_PATH")
		assert.Contains(t, err.Error(), "orbit_local_binary_path")
		assert.Contains(t, err.Error(), "/nonexistent/path/to/orbit")
		assert.Contains(t, err.Error(), "was not found")
	})

	t.Run("non-executable file returns clear error", func(t *testing.T) {
		ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(false))
		factory := cmdtest.NewTestFactory(ios)

		dir := t.TempDir()
		nonExecFile := filepath.Join(dir, "orbit")
		require.NoError(t, os.WriteFile(nonExecFile, []byte("#!/bin/sh\n"), 0o644))

		t.Setenv("GLAB_ORBIT_LOCAL_BINARY_PATH", nonExecFile)
		runner := newRunner(factory.IO(), factory.Config(), Spec())
		err := runner.Run(t.Context())

		require.Error(t, err)
		assert.Contains(t, err.Error(), "GLAB_ORBIT_LOCAL_BINARY_PATH")
		assert.Contains(t, err.Error(), "is not executable")
	})
}

func TestRunE_InstallAndUpdateAreMutuallyExclusive(t *testing.T) {
	t.Parallel()

	ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(false))
	factory := cmdtest.NewTestFactory(ios)
	cmd := NewCmd(factory)
	cmd.SetArgs([]string{"--install", "--update"})

	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mutually exclusive")
}

func TestHandleInstall_CustomPath(t *testing.T) {
	if _, err := binarymgr.ManagedBinaryPath(Spec()); errors.Is(err, binarymgr.ErrUnsupportedPlatform) {
		t.Skipf("skipping on unsupported platform: %v", err)
	}

	ios, _, stderr, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(false))
	factory := cmdtest.NewTestFactory(ios)

	dir := t.TempDir()
	execFile := filepath.Join(dir, "orbit")
	require.NoError(t, os.WriteFile(execFile, []byte("#!/bin/sh\n"), 0o755))

	t.Setenv("GLAB_ORBIT_LOCAL_BINARY_PATH", execFile)
	runner := newRunner(factory.IO(), factory.Config(), Spec())
	err := runner.HandleInstall(t.Context())

	require.NoError(t, err)
	assert.Contains(t, stderr.String(), "Using custom Orbit local CLI binary:")
	assert.Contains(t, stderr.String(), execFile)
}
