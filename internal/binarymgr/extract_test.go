//go:build !integration

package binarymgr

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeTarGz(t *testing.T, entries []tarEntry) string {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Name:     e.name,
			Mode:     e.mode,
			Size:     int64(len(e.body)),
			Typeflag: e.typeflag,
		}))
		_, err := tw.Write(e.body)
		require.NoError(t, err)
	}
	require.NoError(t, tw.Close())
	require.NoError(t, gz.Close())

	path := filepath.Join(t.TempDir(), "archive.tar.gz")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))
	return path
}

type tarEntry struct {
	name     string
	mode     int64
	body     []byte
	typeflag byte
}

func TestTarGzExtractor_extractsBinary(t *testing.T) {
	t.Parallel()

	src := writeTarGz(t, []tarEntry{
		{name: "README", mode: 0o644, body: []byte("readme"), typeflag: tar.TypeReg},
		{name: "orbit", mode: 0o755, body: []byte("orbit-binary-bytes"), typeflag: tar.TypeReg},
	})

	dest := t.TempDir()
	got, err := TarGzExtractor("orbit")(src, dest)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dest, "orbit"), got)

	data, err := os.ReadFile(got)
	require.NoError(t, err)
	assert.Equal(t, []byte("orbit-binary-bytes"), data)

	if runtime.GOOS != "windows" {
		info, err := os.Stat(got)
		require.NoError(t, err)
		assert.NotZero(t, info.Mode().Perm()&0o111, "extracted file should be executable")
	}
}

func TestTarGzExtractor_findsBinaryInSubdir(t *testing.T) {
	t.Parallel()

	src := writeTarGz(t, []tarEntry{
		{name: "release/orbit", mode: 0o755, body: []byte("nested"), typeflag: tar.TypeReg},
	})

	dest := t.TempDir()
	got, err := TarGzExtractor("orbit")(src, dest)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dest, "orbit"), got)
}

func TestTarGzExtractor_missingBinary(t *testing.T) {
	t.Parallel()

	src := writeTarGz(t, []tarEntry{
		{name: "README", mode: 0o644, body: []byte("readme"), typeflag: tar.TypeReg},
	})

	_, err := TarGzExtractor("orbit")(src, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `does not contain "orbit"`)
}

func TestTarGzExtractor_rejectsZipSlip(t *testing.T) {
	t.Parallel()

	src := writeTarGz(t, []tarEntry{
		{name: "../escape/orbit", mode: 0o755, body: []byte("evil"), typeflag: tar.TypeReg},
	})

	_, err := TarGzExtractor("orbit")(src, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes")
}

func TestTarGzExtractor_rejectsAbsolutePath(t *testing.T) {
	t.Parallel()

	src := writeTarGz(t, []tarEntry{
		{name: "/etc/orbit", mode: 0o755, body: []byte("evil"), typeflag: tar.TypeReg},
	})

	_, err := TarGzExtractor("orbit")(src, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes")
}

func TestTarGzExtractor_skipsSymlinks(t *testing.T) {
	t.Parallel()

	src := writeTarGz(t, []tarEntry{
		{name: "orbit", mode: 0o777, typeflag: tar.TypeSymlink},
		{name: "release/orbit", mode: 0o755, body: []byte("real"), typeflag: tar.TypeReg},
	})

	dest := t.TempDir()
	got, err := TarGzExtractor("orbit")(src, dest)
	require.NoError(t, err)

	data, err := os.ReadFile(got)
	require.NoError(t, err)
	assert.Equal(t, []byte("real"), data)
}

type zipEntry struct {
	name string
	mode os.FileMode
	body []byte
}

func writeZip(t *testing.T, entries []zipEntry) string {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		hdr := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		mode := e.mode
		if mode == 0 {
			mode = 0o644
		}
		hdr.SetMode(mode)
		w, err := zw.CreateHeader(hdr)
		require.NoError(t, err)
		_, err = w.Write(e.body)
		require.NoError(t, err)
	}
	require.NoError(t, zw.Close())

	path := filepath.Join(t.TempDir(), "archive.zip")
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0o644))
	return path
}

func TestZipExtractor_extractsBinary(t *testing.T) {
	t.Parallel()

	src := writeZip(t, []zipEntry{
		{name: "README", mode: 0o644, body: []byte("readme")},
		{name: "orbit.exe", mode: 0o755, body: []byte("orbit-binary-bytes")},
	})

	dest := t.TempDir()
	got, err := ZipExtractor("orbit.exe")(src, dest)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dest, "orbit.exe"), got)

	data, err := os.ReadFile(got)
	require.NoError(t, err)
	assert.Equal(t, []byte("orbit-binary-bytes"), data)

	if runtime.GOOS != "windows" {
		info, err := os.Stat(got)
		require.NoError(t, err)
		assert.NotZero(t, info.Mode().Perm()&0o111, "extracted file should be executable")
	}
}

func TestZipExtractor_findsBinaryInSubdir(t *testing.T) {
	t.Parallel()

	src := writeZip(t, []zipEntry{
		{name: "release/orbit.exe", mode: 0o755, body: []byte("nested")},
	})

	dest := t.TempDir()
	got, err := ZipExtractor("orbit.exe")(src, dest)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dest, "orbit.exe"), got)
}

func TestZipExtractor_missingBinary(t *testing.T) {
	t.Parallel()

	src := writeZip(t, []zipEntry{
		{name: "README", mode: 0o644, body: []byte("readme")},
	})

	_, err := ZipExtractor("orbit.exe")(src, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), `does not contain "orbit.exe"`)
}

func TestZipExtractor_rejectsZipSlip(t *testing.T) {
	t.Parallel()

	src := writeZip(t, []zipEntry{
		{name: "../escape/orbit.exe", mode: 0o755, body: []byte("evil")},
	})

	_, err := ZipExtractor("orbit.exe")(src, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes")
}

func TestZipExtractor_rejectsAbsolutePath(t *testing.T) {
	t.Parallel()

	src := writeZip(t, []zipEntry{
		{name: "/etc/orbit.exe", mode: 0o755, body: []byte("evil")},
	})

	_, err := ZipExtractor("orbit.exe")(src, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "escapes")
}
