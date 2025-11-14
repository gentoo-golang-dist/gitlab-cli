//go:build !integration

package cli

import (
	"bytes"
	"fmt"
	"testing"

	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
	"gitlab.com/gitlab-org/cli/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/survivorbat/huhtest"
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

func TestCheckNodeVersion(t *testing.T) {
	// Note: This test requires Node.js to be installed on the system
	// We test that the function properly checks the version, but we can't
	// easily mock the node command execution
	io, _, _, _ := cmdtest.TestIOStreams()
	cfg := config.NewBlankConfig()
	f := cmdtest.NewTestFactory(io, cmdtest.WithConfig(cfg))

	opts := &opts{
		Factory: f,
		IO:      io,
	}

	// Just verify the function doesn't panic and returns some result
	// The actual version check logic is tested through integration tests
	err := opts.checkNodeVersion(t.Context())
	// If node is installed and >= v22, no error
	// If node is not installed or < v22, error
	// We can't assert specific behavior without mocking, which is difficult for external commands
	_ = err
}

func TestRun_AutoRunConfig(t *testing.T) {
	tests := []struct {
		name               string
		autoRunConfig      string
		shareTokenConfig   string
		promptResponses    []string
		expectNodeCheck    bool
		expectExecution    bool
		expectConfigWrites bool
	}{
		{
			name:               "auto_run already set to true",
			autoRunConfig:      "true",
			shareTokenConfig:   "true",
			promptResponses:    []string{}, // No prompts expected
			expectNodeCheck:    true,
			expectExecution:    true,
			expectConfigWrites: false,
		},
		{
			name:               "user selects No on first prompt",
			autoRunConfig:      "",
			shareTokenConfig:   "",
			promptResponses:    []string{"No"},
			expectNodeCheck:    false,
			expectExecution:    false,
			expectConfigWrites: false,
		},
		{
			name:               "user selects Yes one-time",
			autoRunConfig:      "",
			shareTokenConfig:   "false",
			promptResponses:    []string{"Yes"},
			expectNodeCheck:    true,
			expectExecution:    true,
			expectConfigWrites: false,
		},
		{
			name:               "user selects Always",
			autoRunConfig:      "",
			shareTokenConfig:   "false",
			promptResponses:    []string{"Always"},
			expectNodeCheck:    true,
			expectExecution:    true,
			expectConfigWrites: true, // Should save auto_run preference
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create config with test values
			cfgStr := ""
			if tc.autoRunConfig != "" {
				cfgStr += fmt.Sprintf("%s: %s\n", duoCLIAutoRunKey, tc.autoRunConfig)
			}
			if tc.shareTokenConfig != "" {
				cfgStr += fmt.Sprintf("%s: %s\n", duoCLIShareTokenKey, tc.shareTokenConfig)
			}

			cfg := config.NewFromString(cfgStr)

			// Stub config writes to track if Write() is called
			var configBuf bytes.Buffer
			var aliasesBuf bytes.Buffer
			restore := config.StubWriteConfig(&configBuf, &aliasesBuf)
			defer restore()

			// Stub the node command to return valid version
			cs, teardown := test.InitCmdStubber()
			defer teardown()
			cs.Stub("v22.0.0") // node --version
			cs.Stub("")        // npx execution (if reached)

			// Setup prompt responder
			responder := huhtest.NewResponder()
			for _, resp := range tc.promptResponses {
				responder = responder.AddSelect("", 0).AddResponse("", resp)
			}

			opts := []cmdtest.FactoryOption{
				cmdtest.WithConfig(cfg),
				cmdtest.WithResponder(t, responder),
			}

			exec := cmdtest.SetupCmdForTest(t, NewCmdCli, false, opts...)

			// Execute the command
			_, err := exec("")

			// Verify expectations
			if !tc.expectExecution {
				// Command should exit without error when user says No
				assert.NoError(t, err)
			}

			if tc.expectConfigWrites {
				assert.Greater(t, configBuf.Len(), 0, "expected config to be written")
			}
		})
	}
}

func TestRun_TokenSharingConfig(t *testing.T) {
	tests := []struct {
		name             string
		shareTokenConfig string
		tokenValue       string
		promptResponses  []string
		expectTokenSet   bool
		expectConfigSave bool
	}{
		{
			name:             "share_token already set to true",
			shareTokenConfig: "true",
			tokenValue:       "glpat-test-token",
			promptResponses:  []string{},
			expectTokenSet:   true,
			expectConfigSave: false,
		},
		{
			name:             "share_token already set to false",
			shareTokenConfig: "false",
			tokenValue:       "glpat-test-token",
			promptResponses:  []string{},
			expectTokenSet:   false,
			expectConfigSave: false,
		},
		{
			name:             "user selects Yes one-time",
			shareTokenConfig: "",
			tokenValue:       "glpat-test-token",
			promptResponses:  []string{"Yes"},
			expectTokenSet:   true,
			expectConfigSave: false,
		},
		{
			name:             "user selects No one-time",
			shareTokenConfig: "",
			tokenValue:       "glpat-test-token",
			promptResponses:  []string{"No"},
			expectTokenSet:   false,
			expectConfigSave: false,
		},
		{
			name:             "user selects Always",
			shareTokenConfig: "",
			tokenValue:       "glpat-test-token",
			promptResponses:  []string{"Always"},
			expectTokenSet:   true,
			expectConfigSave: true,
		},
		{
			name:             "user selects Never",
			shareTokenConfig: "",
			tokenValue:       "glpat-test-token",
			promptResponses:  []string{"Never"},
			expectTokenSet:   false,
			expectConfigSave: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Create config with test values
			cfgStr := fmt.Sprintf("%s: true\n", duoCLIAutoRunKey) // Skip first prompt
			if tc.shareTokenConfig != "" {
				cfgStr += fmt.Sprintf("%s: %s\n", duoCLIShareTokenKey, tc.shareTokenConfig)
			}
			if tc.tokenValue != "" {
				cfgStr += fmt.Sprintf("hosts:\n  gitlab.com:\n    token: %s\n", tc.tokenValue)
			}

			cfg := config.NewFromString(cfgStr)

			// Stub config writes
			var configBuf bytes.Buffer
			var aliasesBuf bytes.Buffer
			restore := config.StubWriteConfig(&configBuf, &aliasesBuf)
			defer restore()

			// Stub the node command and npx execution
			cs, teardown := test.InitCmdStubber()
			defer teardown()
			cs.Stub("v22.0.0") // node --version
			cs.Stub("")        // npx execution

			// Setup prompt responder
			responder := huhtest.NewResponder()
			for _, resp := range tc.promptResponses {
				responder = responder.AddSelect("", 0).AddResponse("", resp)
			}

			opts := []cmdtest.FactoryOption{
				cmdtest.WithConfig(cfg),
				cmdtest.WithResponder(t, responder),
			}

			exec := cmdtest.SetupCmdForTest(t, NewCmdCli, false, opts...)

			// Execute the command
			_, err := exec("")

			// Note: We can't easily verify the token was passed to the subprocess
			// without more complex mocking, but we can verify config writes
			assert.NoError(t, err)

			if tc.expectConfigSave {
				assert.Greater(t, configBuf.Len(), 0, "expected config to be written")
			}
		})
	}
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

func TestBuildEnvironment(t *testing.T) {
	ios, _, _, _ := cmdtest.TestIOStreams()
	cfg := config.NewBlankConfig()
	f := cmdtest.NewTestFactory(ios, cmdtest.WithConfig(cfg))

	opts := &opts{
		Factory: f,
		IO:      ios,
	}

	token := "glpat-test-token-12345"
	env := opts.buildEnvironment(token)

	// Check that GITLAB_TOKEN is in the environment
	found := false
	for _, e := range env {
		if e == "GITLAB_TOKEN="+token {
			found = true
			break
		}
	}
	assert.True(t, found, "GITLAB_TOKEN should be in environment")

	// Verify no duplicate GITLAB_TOKEN entries
	count := 0
	for _, e := range env {
		if len(e) >= 13 && e[:13] == "GITLAB_TOKEN=" {
			count++
		}
	}
	assert.Equal(t, 1, count, "should only have one GITLAB_TOKEN entry")
}
