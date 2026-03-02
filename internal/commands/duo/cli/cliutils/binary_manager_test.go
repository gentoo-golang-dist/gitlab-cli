//go:build !integration

package cliutils

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

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
