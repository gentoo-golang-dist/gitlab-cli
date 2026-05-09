package binarymgr

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"slices"

	"gitlab.com/gitlab-org/cli/internal/config"
)

// ErrUnsupportedPlatform is returned when the running OS or architecture is
// not supported by the Spec.
var ErrUnsupportedPlatform = errors.New("unsupported platform")

// Platform is the resolved (OS, arch) pair for the running host. arch is
// already normalized to the upstream naming used by Spec.AssetName.
type Platform struct {
	OS   string
	Arch string
}

// detectPlatform resolves the running platform against the Spec's supported
// OS list and arch normalizer.
func detectPlatform(spec Spec) (Platform, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	if !slices.Contains(spec.SupportedOS, goos) {
		return Platform{}, fmt.Errorf("%w: operating system %s (supported: %v)", ErrUnsupportedPlatform, goos, spec.SupportedOS)
	}

	arch, err := spec.NormalizeArch(goos, goarch)
	if err != nil {
		return Platform{}, err
	}

	return Platform{OS: goos, Arch: arch}, nil
}

// installDir is the shared bin directory under glab's config dir
// (~/.config/glab-cli/bin on Unix, %APPDATA%/glab-cli/bin on Windows).
// All managed binaries co-locate here, mirroring the package-manager pattern
// (npm, cargo, asdf) and avoiding PATH pollution.
func installDir() string {
	return filepath.Join(config.ConfigDir(), "bin")
}

// binaryPath returns the absolute install path for the managed binary.
func binaryPath(spec Spec, p Platform) string {
	return filepath.Join(installDir(), spec.InstalledName(p.OS))
}

// assetName returns the upstream asset filename for the resolved platform.
func assetName(spec Spec, p Platform) string {
	return spec.AssetName(p.OS, p.Arch)
}

// ManagedBinaryPath returns the absolute path where Spec's binary lives
// after installation. Used by callers that need to compare against a
// user-configured custom path.
func ManagedBinaryPath(spec Spec) (string, error) {
	p, err := detectPlatform(spec)
	if err != nil {
		return "", err
	}
	return binaryPath(spec, p), nil
}
