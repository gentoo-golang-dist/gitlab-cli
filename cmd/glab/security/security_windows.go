//go:build windows

package security

// CheckRoot is a no-op on Windows, as setuid/setgid permissions
// are a POSIX concept and do not apply.
func CheckRoot() {
	// Do nothing
}
