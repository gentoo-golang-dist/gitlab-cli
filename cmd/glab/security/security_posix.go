//go:build !windows

package security

import (
	"fmt"
	"os"
)

// CheckRoot performs a security check to ensure the binary is not run as root
// with setuid or setgid bits set, which is a security risk.
func CheckRoot() {
	// 1. Check if the effective user ID is root (EUID=0)
	if os.Geteuid() != 0 {
		// Not running as root, so we are safe.
		return
	}

	// 2. Get the path to the running executable
	executablePath, err := os.Executable()
	if err != nil {
		// If we can't get the executable path, we can't check its perms.
		// It's safer to warn, but we'll allow execution to continue.
		fmt.Fprintf(os.Stderr, "Warning: could not determine executable path: %v\n", err)
		return
	}

	// 3. Get the file stats (os.Stat follows symlinks)
	stat, err := os.Stat(executablePath)
	if err != nil {
		// Similar to above, warn but continue.
		fmt.Fprintf(os.Stderr, "Warning: could not stat executable: %v\n", err)
		return
	}

	// 4. Check the file mode for setuid or setgid bits
	mode := stat.Mode()
	if mode&os.ModeSetuid != 0 || mode&os.ModeSetgid != 0 {
		// 5. This is the dangerous condition. Print error and exit.
		fmt.Fprintln(os.Stderr, "Error: running as root with the setuid or setgid bit set is not allowed")
		fmt.Fprintln(os.Stderr, "Please remove the setuid/setgid bit from the binary: chmod -s $(which glab)")
		os.Exit(1)
	}
}
