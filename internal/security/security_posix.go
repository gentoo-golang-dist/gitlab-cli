//go:build !windows

package security

import (
	"fmt"
	"os"
	"syscall"
)

// CheckRoot inspects the running binary and exits with an error if unsafe
// POSIX permission bits (setuid/setgid) are present. The check is intentionally
// concise and prints a single remediation command to restore safe permissions.
func CheckRoot() {
	executablePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not determine executable path: %v\n", err)
		return
	}

	fileInfo, err := os.Stat(executablePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not stat executable: %v\n", err)
		return
	}

	mode := fileInfo.Mode()
	hasSetuid := mode&os.ModeSetuid != 0
	hasSetgid := mode&os.ModeSetgid != 0

	// If neither special bit is present, optionally warn about running as root
	if !hasSetuid && !hasSetgid {
		if os.Geteuid() == 0 {
			fmt.Fprintln(os.Stderr, "Warning: running glab as root is not recommended")
		}
		return
	}

	// Build a concise error message covering both cases.
	msg := "Error: unsafe file permissions detected on the glab binary.\n"
	msg += "The binary has the following special permission(s):"
	if hasSetuid {
		msg += " setuid"
	}
	if hasSetgid {
		if hasSetuid {
			msg += " and"
		}
		msg += " setgid"
	}
	msg += ".\n"
	msg += fmt.Sprintf("Please remove these bits and set safe permissions (for example 0755):\n  sudo chmod 0755 %s\n", executablePath)

	// If possible, also report if the file is owned by root (uid/gid == 0).
	if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
		if stat.Uid == 0 || stat.Gid == 0 {
			msg += fmt.Sprintf("Note: the file is owned by uid=%d gid=%d. If this binary should not be root-owned, consider changing ownership.\n", stat.Uid, stat.Gid)
		}
	}

	os.Exit(1)
}
