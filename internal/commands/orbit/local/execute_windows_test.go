//go:build windows && !integration

package local

import (
	"errors"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWrapExecError_emulationHintOnARM64(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skip("hint only applies on arm64")
	}
	err := wrapExecError(errors.New("fork/exec orbit.exe: %1 is not a valid Win32 application."))
	assert.ErrorContains(t, err, "x64 emulation")
}

func TestWrapExecError_corruptHintOnAMD64(t *testing.T) {
	if runtime.GOARCH != "amd64" {
		t.Skip("hint only applies on amd64")
	}
	err := wrapExecError(errors.New("fork/exec orbit.exe: %1 is not a valid Win32 application."))
	assert.ErrorContains(t, err, "corrupted")
}

func TestWrapExecError_genericMessage(t *testing.T) {
	err := wrapExecError(errors.New("some other failure"))
	assert.ErrorContains(t, err, "failed to execute Orbit local CLI")
}
