//go:build !integration

package lint

import (
	"path/filepath"
	"runtime"
)

func testdataPath(parts ...string) string {
	_, filename, _, _ := runtime.Caller(0)
	base := filepath.Join(filepath.Dir(filename), "testdata")
	return filepath.Join(append([]string{base}, parts...)...)
}
