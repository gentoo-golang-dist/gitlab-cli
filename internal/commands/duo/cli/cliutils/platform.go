package cliutils

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"

	"gitlab.com/gitlab-org/cli/internal/config"
)

// ErrUnsupportedPlatform is returned when the current OS or architecture
// is not supported by the Duo CLI.
var ErrUnsupportedPlatform = errors.New("unsupported platform")

// platform represents the target operating system and architecture for the Duo CLI binary.
type platform struct {
	os   string // darwin, linux, windows
	arch string // x64, arm64, x64-baseline
}

// detectPlatform identifies the current platform (OS and architecture).
// It normalizes architecture names to match Duo CLI binary naming conventions.
func detectPlatform() (platform, error) {
	goos := runtime.GOOS
	goarch := runtime.GOARCH

	if !isSupportedOS(goos) {
		return platform{}, fmt.Errorf("%w: operating system %s (supported: darwin, linux, windows)", ErrUnsupportedPlatform, goos)
	}

	arch, err := normalizeArch(goos, goarch)
	if err != nil {
		return platform{}, err
	}

	return platform{
		os:   goos,
		arch: arch,
	}, nil
}

// binaryName returns the Duo CLI binary filename for this platform.
// Examples: duo-darwin-arm64, duo-linux-x64, duo-windows-x64-baseline.exe
func (p platform) binaryName() string {
	name := fmt.Sprintf("duo-%s-%s", p.os, p.arch)
	if p.os == "windows" {
		name += ".exe"
	}
	return name
}

// installDir returns the installation directory for the Duo CLI binary.
// Uses glab's config directory to co-locate the binary with glab configuration.
// This follows the package manager pattern (npm, cargo, asdf) and avoids PATH pollution.
//
// Installation directory: <config-dir>/bin
//   - Unix (macOS/Linux): ~/.config/glab-cli/bin (or $XDG_CONFIG_HOME/glab-cli/bin)
//   - Windows: %APPDATA%\glab-cli\bin (or %GLAB_CONFIG_DIR%\bin if set)
func (p platform) installDir() string {
	configDir := config.ConfigDir()
	return filepath.Join(configDir, "bin")
}

// binaryPath returns the full path to the installed Duo CLI binary.
func (p platform) binaryPath() string {
	installDir := p.installDir()

	binaryName := "duo"
	if p.os == "windows" {
		binaryName = "duo.exe"
	}

	return filepath.Join(installDir, binaryName)
}

// isSupportedOS checks if the operating system is supported.
func isSupportedOS(os string) bool {
	switch os {
	case "darwin", "linux", "windows":
		return true
	default:
		return false
	}
}

// normalizeArch converts Go architecture names to Duo CLI binary naming conventions.
// - amd64 → x64 (or x64-baseline for Windows)
// - arm64 → arm64 (unchanged)
// - Windows ARM64 is not supported
func normalizeArch(goos, goarch string) (string, error) {
	switch goarch {
	case "amd64":
		if goos == "windows" {
			return "x64-baseline", nil
		}
		return "x64", nil
	case "arm64", "aarch64":
		if goos == "windows" {
			return "", fmt.Errorf("%w: Windows ARM64 is not supported by GitLab Duo CLI", ErrUnsupportedPlatform)
		}
		return "arm64", nil
	default:
		return "", fmt.Errorf("%w: architecture %s (supported: amd64/x64, arm64)", ErrUnsupportedPlatform, goarch)
	}
}
