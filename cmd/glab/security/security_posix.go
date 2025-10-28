//go:build !windows

package security

import (
	"fmt"
	"os"
)

// CheckRoot performs a security check to ensure the binary is not run as root
// with setuid or setgid bits set, which is a security risk.
func CheckRoot() {
	// Get the path to the running executable
	executablePath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not determine executable path: %v\n", err)
		return
	}

	// Get file info and check raw mode bits
	fileInfo, err := os.Stat(executablePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not stat executable: %v\n", err)
		return
	}

	// Get raw mode bits including setuid/setgid bits
	mode := fileInfo.Mode()
	fmt.Fprintf(os.Stderr, "Debug: File mode: %v\n", mode)

	// Check for setuid/setgid using Mode's built-in checks
	if mode&os.ModeSetuid != 0 {
		fmt.Fprintln(os.Stderr, "Error: setuid bit is set on the binary which is not allowed")
		fmt.Fprintf(os.Stderr, "Please remove the setuid bit: sudo chmod u-s %s\n", executablePath)
		os.Exit(1)
	}

	if mode&os.ModeSetgid != 0 {
		fmt.Fprintln(os.Stderr, "Error: setgid bit is set on the binary which is not allowed")
		fmt.Fprintf(os.Stderr, "Please remove the setgid bit: sudo chmod g-s %s\n", executablePath)
		os.Exit(1)
	}

	// Warn if running as root
	if os.Geteuid() == 0 {
		fmt.Fprintln(os.Stderr, "Warning: Running glab as root is not recommended")
	}
}
