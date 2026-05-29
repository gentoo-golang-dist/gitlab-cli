//go:build !integration

package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDetectInstallMethodFromPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		exePath string
		gopath  string
		home    string
		want    InstallMethod
	}{
		{
			name:    "homebrew apple silicon",
			exePath: "/opt/homebrew/bin/glab",
			home:    "/Users/somebody",
			want:    InstallMethod{Name: installMethodHomebrew, UpgradeCommand: homebrewUpgradeCommand},
		},
		{
			name:    "homebrew intel cellar",
			exePath: "/usr/local/Cellar/glab/1.62.1/bin/glab",
			home:    "/Users/somebody",
			want:    InstallMethod{Name: installMethodHomebrew, UpgradeCommand: homebrewUpgradeCommand},
		},
		{
			name:    "linuxbrew",
			exePath: "/home/linuxbrew/.linuxbrew/bin/glab",
			home:    "/home/someone",
			want:    InstallMethod{Name: installMethodHomebrew, UpgradeCommand: homebrewUpgradeCommand},
		},
		{
			name:    "go install with explicit GOPATH",
			exePath: "/Users/someone/sdk/go/bin/glab",
			gopath:  "/Users/someone/sdk/go",
			home:    "/Users/someone",
			want:    InstallMethod{Name: installMethodGoInstall, UpgradeCommand: goInstallUpgradeCommand},
		},
		{
			name:    "go install via $HOME/go/bin fallback",
			exePath: "/Users/someone/go/bin/glab",
			home:    "/Users/someone",
			want:    InstallMethod{Name: installMethodGoInstall, UpgradeCommand: goInstallUpgradeCommand},
		},
		{
			name:    "unknown system path",
			exePath: "/usr/local/bin/glab",
			home:    "/Users/somebody",
			want:    InstallMethod{Name: installMethodUnknown},
		},
		{
			name:    "unknown local custom path",
			exePath: "/Users/somebody/projects/glab/cli/bin/glab",
			home:    "/Users/somebody",
			want:    InstallMethod{Name: installMethodUnknown},
		},
		{
			name:    "user-local path containing homebrew substring is not homebrew",
			exePath: "/home/user/opt/homebrew/bin/glab",
			home:    "/home/user",
			want:    InstallMethod{Name: installMethodUnknown},
		},
		{
			name:    "empty home does not match go-install",
			exePath: "/some/go/bin/glab",
			home:    "",
			want:    InstallMethod{Name: installMethodUnknown},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, detectInstallMethodFromPath(tc.exePath, tc.gopath, tc.home))
		})
	}
}
