//go:build !integration

package cliutils

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	goversion "github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestBinaryManager_isBinaryValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setupFile func(t *testing.T) string
		want      bool
	}{
		{
			name: "valid executable file",
			setupFile: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				binPath := filepath.Join(tmpDir, "duo")
				err := os.WriteFile(binPath, []byte("fake binary"), 0o755)
				require.NoError(t, err)
				return binPath
			},
			want: true,
		},
		{
			name: "non-executable file",
			setupFile: func(t *testing.T) string {
				t.Helper()
				tmpDir := t.TempDir()
				binPath := filepath.Join(tmpDir, "duo")
				err := os.WriteFile(binPath, []byte("fake binary"), 0o644)
				require.NoError(t, err)
				return binPath
			},
			want: false,
		},
		{
			name: "non-existent file",
			setupFile: func(t *testing.T) string {
				t.Helper()
				return "/nonexistent/duo"
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(false))
			manager := NewBinaryManager(ios)
			binPath := tc.setupFile(t)

			got := manager.isBinaryValid(binPath)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestBinaryManager_verifyChecksum(t *testing.T) {
	t.Parallel()

	// Create a test file with known content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.bin")
	testContent := []byte("test content for checksum")
	err := os.WriteFile(testFile, testContent, 0o644)
	require.NoError(t, err)

	// Calculate expected checksum
	hash := sha256.New()
	hash.Write(testContent)
	expectedChecksum := hex.EncodeToString(hash.Sum(nil))

	tests := []struct {
		name             string
		filePath         string
		expectedChecksum string
		wantsErr         bool
	}{
		{
			name:             "valid checksum",
			filePath:         testFile,
			expectedChecksum: expectedChecksum,
			wantsErr:         false,
		},
		{
			name:             "invalid checksum",
			filePath:         testFile,
			expectedChecksum: "invalid_checksum",
			wantsErr:         true,
		},
		{
			name:             "empty checksum (skipped)",
			filePath:         testFile,
			expectedChecksum: "",
			wantsErr:         false,
		},
		{
			name:             "non-existent file",
			filePath:         "/nonexistent/file",
			expectedChecksum: expectedChecksum,
			wantsErr:         true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(false))
			manager := NewBinaryManager(ios)

			err := manager.verifyChecksum(tc.filePath, tc.expectedChecksum)

			if tc.wantsErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDuoMaxCompatibleMajorVersion(t *testing.T) {
	t.Parallel()

	// Verify that the version library correctly identifies which versions are
	// blocked by the major version ceiling.
	tests := []struct {
		version       string
		expectBlocked bool
	}{
		{"8.0.0", false},
		{"8.79.0", false},
		{"v8.5.0", false},
		{"9.0.0", true},
		{"9.1.2", true},
		{"10.0.0", true},
	}

	for _, tc := range tests {
		t.Run(tc.version, func(t *testing.T) {
			t.Parallel()
			v, err := goversion.NewVersion(tc.version)
			require.NoError(t, err)
			blocked := v.Segments()[0] > duoMaxCompatibleMajorVersion
			assert.Equal(t, tc.expectBlocked, blocked)
		})
	}
}
