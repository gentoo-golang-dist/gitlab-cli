package update

import (
	"os"
	"path/filepath"
	"strings"
)

// InstallMethod identifies how the running glab binary was installed and the
// command we should suggest to upgrade it. UpgradeCommand is empty when we
// can't determine the install method; callers should fall back to pointing
// users at the release notes URL.
type InstallMethod struct {
	Name           string
	UpgradeCommand string
}

const (
	installMethodHomebrew  = "homebrew"
	installMethodGoInstall = "go-install"
	installMethodUnknown   = "unknown"

	homebrewUpgradeCommand  = "brew upgrade glab"
	goInstallUpgradeCommand = "go install gitlab.com/gitlab-org/cli/cmd/glab@latest"
)

// DetectInstallMethod inspects the resolved path of the running executable and
// returns its install method. It never panics: any error walking the path
// resolves to installMethodUnknown.
func DetectInstallMethod() InstallMethod {
	exe, err := os.Executable()
	if err != nil {
		return InstallMethod{Name: installMethodUnknown}
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return detectInstallMethodFromPath(exe, os.Getenv("GOPATH"), homeDir())
}

func homeDir() string {
	if h, err := os.UserHomeDir(); err == nil {
		return h
	}
	return ""
}

// detectInstallMethodFromPath is the pure form of DetectInstallMethod, used
// for testing without touching the filesystem or process state.
func detectInstallMethodFromPath(exePath, gopath, home string) InstallMethod {
	p := filepath.ToSlash(exePath)

	homebrewPrefixes := []string{
		"/opt/homebrew/",
		"/usr/local/Cellar/",
		"/home/linuxbrew/.linuxbrew/",
	}
	for _, prefix := range homebrewPrefixes {
		if strings.HasPrefix(p, prefix) {
			return InstallMethod{Name: installMethodHomebrew, UpgradeCommand: homebrewUpgradeCommand}
		}
	}

	for _, dir := range goBinDirs(gopath, home) {
		if dir == "" {
			continue
		}
		if strings.HasPrefix(p, filepath.ToSlash(dir)+"/") {
			return InstallMethod{Name: installMethodGoInstall, UpgradeCommand: goInstallUpgradeCommand}
		}
	}

	return InstallMethod{Name: installMethodUnknown}
}

func goBinDirs(gopath, home string) []string {
	var dirs []string
	if gopath != "" {
		for _, g := range filepath.SplitList(gopath) {
			if g != "" {
				dirs = append(dirs, filepath.Join(g, "bin"))
			}
		}
	}
	if home != "" {
		dirs = append(dirs, filepath.Join(home, "go", "bin"))
	}
	return dirs
}
