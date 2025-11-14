//go:build !integration

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCmdCli(t *testing.T) {
	io, _, _, _ := cmdtest.TestIOStreams()
	cfg := config.NewBlankConfig()

	f := cmdtest.NewTestFactory(io, cmdtest.WithConfig(cfg))

	cmd := NewCmdCli(f)

	// Verify the command was created with the right properties
	assert.NotNil(t, cmd)
	assert.Equal(t, "cli", cmd.Use)
	assert.Equal(t, "Run the GitLab Duo CLI (EXPERIMENTAL)", cmd.Short)
	assert.Contains(t, cmd.Long, "npx")
	assert.Contains(t, cmd.Long, "Node.js version 22")
	assert.Contains(t, cmd.Long, "duo_cli_auto_run")
	assert.Contains(t, cmd.Long, "duo_cli_share_token")
	assert.NotNil(t, cmd.RunE)
}

func TestHandleMissingNpx(t *testing.T) {
	ios, _, _, _ := cmdtest.TestIOStreams()
	cfg := config.NewBlankConfig()
	f := cmdtest.NewTestFactory(ios, cmdtest.WithConfig(cfg))

	opts := &opts{
		Factory: f,
		IO:      ios,
	}

	err := opts.handleMissingNpx()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "npx is required but not installed")
	assert.Contains(t, err.Error(), "typically installed with Node.js")
	assert.Contains(t, err.Error(), "Please install Node.js version 22 or higher")
	assert.Contains(t, err.Error(), "https://nodejs.org/")
}

func TestHandleMissingNode(t *testing.T) {
	ios, _, _, _ := cmdtest.TestIOStreams()
	cfg := config.NewBlankConfig()
	f := cmdtest.NewTestFactory(ios, cmdtest.WithConfig(cfg))

	opts := &opts{
		Factory: f,
		IO:      ios,
	}

	err := opts.handleMissingNode()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Node.js is required but not installed")
	assert.Contains(t, err.Error(), "version 22 or higher")
	assert.Contains(t, err.Error(), "https://nodejs.org/")
}

func TestHandleOldNodeVersion(t *testing.T) {
	ios, _, _, _ := cmdtest.TestIOStreams()
	cfg := config.NewBlankConfig()
	f := cmdtest.NewTestFactory(ios, cmdtest.WithConfig(cfg))

	opts := &opts{
		Factory: f,
		IO:      ios,
	}

	err := opts.handleOldNodeVersion(18)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Node.js version 18 is installed")
	assert.Contains(t, err.Error(), "version 22 or higher is required")
	assert.Contains(t, err.Error(), "Please install Node.js version 22 or higher")
	assert.Contains(t, err.Error(), "https://nodejs.org/")
}

func TestSaveConfigValue(t *testing.T) {
	cfgStr := ""
	cfg := config.NewFromString(cfgStr)

	var configBuf bytes.Buffer
	var aliasesBuf bytes.Buffer
	restore := config.StubWriteConfig(&configBuf, &aliasesBuf)
	defer restore()

	ios, _, _, _ := cmdtest.TestIOStreams()
	f := cmdtest.NewTestFactory(ios, cmdtest.WithConfig(cfg))

	opts := &opts{
		Factory: f,
		IO:      ios,
	}

	err := opts.saveConfigValue("test_key", "test_value", "test preference")
	require.NoError(t, err)
	assert.Greater(t, configBuf.Len(), 0, "config should have been written")
}

func TestHasDuoCLIAuth(t *testing.T) {
	tests := []struct {
		name           string
		storageContent string
		createFile     bool
		expectAuth     bool
	}{
		{
			name:       "file doesn't exist",
			createFile: false,
			expectAuth: false,
		},
		{
			name:           "file exists but empty",
			createFile:     true,
			storageContent: "",
			expectAuth:     false,
		},
		{
			name:           "file exists with empty JSON",
			createFile:     true,
			storageContent: "{}",
			expectAuth:     false,
		},
		{
			name:           "file exists with duo-cli-config but no token",
			createFile:     true,
			storageContent: `{"duo-cli-config": {}}`,
			expectAuth:     false,
		},
		{
			name:           "file exists with empty token",
			createFile:     true,
			storageContent: `{"duo-cli-config": {"gitlabAuthToken": ""}}`,
			expectAuth:     false,
		},
		{
			name:           "file exists with valid token",
			createFile:     true,
			storageContent: `{"duo-cli-config": {"gitlabAuthToken": "glpat-test-token"}}`,
			expectAuth:     true,
		},
		{
			name:           "file exists with invalid JSON",
			createFile:     true,
			storageContent: `{"duo-cli-config": invalid json}`,
			expectAuth:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ios, _, _, _ := cmdtest.TestIOStreams()
			cfg := config.NewBlankConfig()
			f := cmdtest.NewTestFactory(ios, cmdtest.WithConfig(cfg))

			opts := &opts{
				Factory: f,
				IO:      ios,
			}

			// Create a temporary directory for testing
			tmpDir := t.TempDir()

			// Set up the storage file if needed
			if tt.createFile {
				gitlabDir := filepath.Join(tmpDir, ".gitlab")
				err := os.MkdirAll(gitlabDir, 0o755)
				require.NoError(t, err)

				storagePath := filepath.Join(gitlabDir, "storage.json")
				err = os.WriteFile(storagePath, []byte(tt.storageContent), 0o644)
				require.NoError(t, err)
			}

			// Temporarily override the home directory for testing
			t.Setenv("HOME", tmpDir)

			// Test the function
			hasAuth := opts.hasDuoCLIAuth()
			assert.Equal(t, tt.expectAuth, hasAuth)
		})
	}
}
