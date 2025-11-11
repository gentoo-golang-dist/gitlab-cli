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

	// Only treat as an error when either setuid or setgid is present.
	if hasSetuid || hasSetgid {
		fmt.Fprintln(os.Stderr, "Error: unsafe file permissions detected on the glab binary.")

		if hasSetuid && hasSetgid {
			fmt.Fprintln(os.Stderr, "The permission bits that must be removed are: setuid, setgid.")
		} else if hasSetuid {
			fmt.Fprintln(os.Stderr, "The permission bits that must be removed are: setuid.")
		} else if hasSetgid {
			fmt.Fprintln(os.Stderr, "The permission bits that must be removed are: setgid.")
		}

		fmt.Fprintf(os.Stderr, "Please remove these bits and set safe permissions (for example 0755):\n  sudo chmod 0755 %s\n", executablePath)

		// If possible, also report if the file is owned by root (uid/gid == 0).
		if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
			if stat.Uid == 0 || stat.Gid == 0 {
				fmt.Fprintf(os.Stderr, "Note: the file is owned by uid=%d gid=%d. If this binary should not be root-owned, consider changing ownership.\n", stat.Uid, stat.Gid)
			}
		}

		os.Exit(1)
	}

	// If the binary is root-owned but does not have setuid/setgid, print an informational note only.
	if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
		if stat.Uid == 0 || stat.Gid == 0 {
			fmt.Fprintf(os.Stderr, "Note: the file is owned by uid=%d gid=%d. The binary does not have setuid/setgid but is root-owned.\n", stat.Uid, stat.Gid)
		}
	}
}
