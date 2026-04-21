//go:build !integration

package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zalando/go-keyring"
	"go.yaml.in/yaml/v3"
)

func Test_configLock(t *testing.T) {
	defaultRoot := NewBlankRoot()
	cfg := NewConfig(defaultRoot)
	out, err := yaml.Marshal(defaultRoot)
	require.NoError(t, err)

	configLockPath := filepath.Join("config.yaml.lock")

	err = os.Chmod(configLockPath, 0o600)
	require.NoError(t, err)

	expected, yml, err := ParseConfigFile(configLockPath)
	require.NoError(t, err)
	assert.Equal(t, string(expected), string(out))

	lockCfg := NewConfig(yml)

	expectedHosts, err := cfg.Hosts()
	require.NoError(t, err)
	lockHosts, err := lockCfg.Hosts()
	require.NoError(t, err)
	assert.Equal(t, expectedHosts, lockHosts)

	expectedAliases, err := cfg.Aliases()
	require.NoError(t, err)
	lockAliases, err := lockCfg.Aliases()
	require.NoError(t, err)
	assert.Equal(t, expectedAliases.All(), lockAliases.All())
}

func Test_fileConfig_Set(t *testing.T) {
	defer StubConfig(`---
git_protocol: ssh
editor: vim
hosts:
  gitlab.com:
    token:
    git_protocol: https
    username: user
`, `
`)()

	mainBuf := bytes.Buffer{}
	aliasesBuf := bytes.Buffer{}
	defer StubWriteConfig(&mainBuf, &aliasesBuf)()

	c, err := ParseConfig("config.yml")
	require.NoError(t, err)

	assert.NoError(t, c.Set("", "editor", "nano"))
	assert.NoError(t, c.Set("gitlab.com", "git_protocol", "ssh"))
	assert.NoError(t, c.Set("example.com", "username", "testUser"))
	assert.NoError(t, c.Set("gitlab.com", "username", "hubot"))
	assert.NoError(t, c.WriteAll())

	expected := heredoc.Doc(`
git_protocol: ssh
editor: nano
hosts:
    gitlab.com:
        token:
        git_protocol: ssh
        username: hubot
    example.com:
        username: testUser
`)
	assert.Equal(t, expected, mainBuf.String())
}

func Test_fileConfig_Set_Empty_Removes(t *testing.T) {
	defer StubConfig(`---
git_protocol: ssh
editor: vim
hosts:
  gitlab.com:
    token: foobar
    git_protocol: https
    username: user
`, `
`)()

	mainBuf := bytes.Buffer{}
	aliasesBuf := bytes.Buffer{}
	defer StubWriteConfig(&mainBuf, &aliasesBuf)()

	c, err := ParseConfig("config.yml")
	require.NoError(t, err)

	assert.NoError(t, c.Set("", "editor", ""))
	assert.NoError(t, c.Set("gitlab.com", "token", ""))
	assert.NoError(t, c.WriteAll())

	expected := heredoc.Doc(`
git_protocol: ssh
hosts:
    gitlab.com:
        git_protocol: https
        username: user
`)
	assert.Equal(t, expected, mainBuf.String())
}

func Test_defaultConfig(t *testing.T) {
	mainBuf := bytes.Buffer{}
	hostsBuf := bytes.Buffer{}
	defer StubWriteConfig(&mainBuf, &hostsBuf)()

	cfg := NewBlankConfig()
	assert.NoError(t, cfg.Write())
	assert.Equal(t, "", hostsBuf.String())

	proto, err := cfg.Get("", "git_protocol")
	assert.Nil(t, err)
	assert.Equal(t, "ssh", proto)

	editor, err := cfg.Get("", "editor")
	assert.Nil(t, err)
	assert.Equal(t, os.Getenv("EDITOR"), editor)

	aliases, err := cfg.Aliases()
	assert.Nil(t, err)
	assert.Equal(t, len(aliases.All()), 2)
	expansion, _ := aliases.Get("co")
	assert.Equal(t, expansion, "mr checkout")
}

func Test_getFromKeyring(t *testing.T) {
	mainBuf := bytes.Buffer{}
	hostsBuf := bytes.Buffer{}
	defer StubWriteConfig(&mainBuf, &hostsBuf)()

	c := NewBlankConfig()

	// Ensure host exists and its token is empty
	err := c.Set("gitlab.com", "token", "")
	require.NoError(t, err)
	err = c.Write()
	require.NoError(t, err)

	keyring.MockInit()
	token, _, err := c.GetWithSource("gitlab.com", "token", false)
	assert.NoError(t, err)
	assert.Equal(t, "", token)

	err = keyring.Set("glab:gitlab.com", "", "glpat-1234")
	require.NoError(t, err)

	token, _, err = c.GetWithSource("gitlab.com", "token", false)

	assert.NoError(t, err)
	assert.Equal(t, "glpat-1234", token)
}

func Test_config_Get_NotFoundError(t *testing.T) {
	mainBuf := bytes.Buffer{}
	hostsBuf := bytes.Buffer{}
	defer StubWriteConfig(&mainBuf, &hostsBuf)()

	cfg := NewBlankConfig()

	local, err := cfg.Local()
	require.Nil(t, err)
	require.NotNil(t, local)

	_, err = local.FindEntry("git_protocol")
	require.Error(t, err)
	assert.True(t, isNotFoundError(err))
}

func TestCustomHeader_ResolvedValue_MissingEnvVar(t *testing.T) {
	// Ensure the environment variable doesn't exist
	os.Unsetenv("NONEXISTENT_VAR")

	header := CustomHeader{
		Name:         "X-Test-Header",
		ValueFromEnv: "NONEXISTENT_VAR",
	}

	value, err := header.ResolvedValue()
	require.Error(t, err)
	require.Empty(t, value)
	require.Contains(t, err.Error(), "environment variable \"NONEXISTENT_VAR\" for header \"X-Test-Header\" is not set or empty")
}

func TestCustomHeader_ResolvedValue_EmptyEnvVar(t *testing.T) {
	// Set environment variable to empty string
	t.Setenv("EMPTY_VAR", "")

	header := CustomHeader{
		Name:         "X-Test-Header",
		ValueFromEnv: "EMPTY_VAR",
	}

	value, err := header.ResolvedValue()
	require.Error(t, err)
	require.Empty(t, value)
	require.Contains(t, err.Error(), "environment variable \"EMPTY_VAR\" for header \"X-Test-Header\" is not set or empty")
}

func TestResolveCustomHeaders_MissingEnvVar(t *testing.T) {
	// Ensure the environment variable doesn't exist
	os.Unsetenv("MISSING_SECRET")

	configYAML := `
hosts:
  gitlab.com:
    custom_headers:
      - name: Cf-Access-Client-Secret
        valueFromEnv: MISSING_SECRET
`

	cfg := NewFromString(configYAML)
	headers, err := ResolveCustomHeaders(cfg, "gitlab.com")

	require.Error(t, err)
	require.Nil(t, headers)
	require.Contains(t, err.Error(), "failed to resolve header \"Cf-Access-Client-Secret\"")
	require.Contains(t, err.Error(), "environment variable \"MISSING_SECRET\" for header \"Cf-Access-Client-Secret\" is not set or empty")
}

func TestConfig_parseHosts_NoHosts(t *testing.T) {
	t.Parallel()

	cfg := &fileConfig{}
	// Create empty hosts node
	emptyHostsNode := &yaml.Node{Kind: yaml.MappingNode}

	_, err := cfg.parseHosts(emptyHostsNode)

	assert.True(t, isNotFoundError(err))
}

func Test_SetKeyring_StoresTokenInKeyringAndSetsIndicator(t *testing.T) {
	mainBuf := bytes.Buffer{}
	hostsBuf := bytes.Buffer{}
	defer StubWriteConfig(&mainBuf, &hostsBuf)()

	keyring.MockInit()
	cfg := NewBlankConfig()

	// Enable keyring mode
	err := cfg.Set("gitlab.com", "use_keyring", "true")
	require.NoError(t, err)

	// Set a token - should go to keyring
	err = cfg.Set("gitlab.com", "token", "glpat-secret-token")
	require.NoError(t, err)

	// Verify token is stored in keyring with new key format
	storedToken, err := keyring.Get("glab:gitlab.com:token", "")
	require.NoError(t, err)
	assert.Equal(t, "glpat-secret-token", storedToken)

	// Verify use_keyring indicator is set in config
	useKeyring, err := cfg.Get("gitlab.com", "use_keyring")
	require.NoError(t, err)
	assert.Equal(t, "true", useKeyring)

	// Verify token is NOT in config (removed/empty)
	err = cfg.Write()
	require.NoError(t, err)
	configContent := mainBuf.String()
	assert.NotContains(t, configContent, "glpat-secret-token", "Token should not be in plaintext config")
	assert.Contains(t, configContent, "use_keyring: \"true\"")
}

func Test_SetKeyring_OAuth2RefreshToken(t *testing.T) {
	mainBuf := bytes.Buffer{}
	hostsBuf := bytes.Buffer{}
	defer StubWriteConfig(&mainBuf, &hostsBuf)()

	keyring.MockInit()
	cfg := NewBlankConfig()

	// Enable keyring mode
	err := cfg.Set("gitlab.com", "use_keyring", "true")
	require.NoError(t, err)

	// Set a refresh token - should go to keyring
	err = cfg.Set("gitlab.com", "oauth2_refresh_token", "refresh-secret-token")
	require.NoError(t, err)

	// Verify refresh token is stored in keyring with new key format
	storedToken, err := keyring.Get("glab:gitlab.com:oauth2_refresh_token", "")
	require.NoError(t, err)
	assert.Equal(t, "refresh-secret-token", storedToken)

	// Verify use_keyring indicator is set in config
	useKeyring, err := cfg.Get("gitlab.com", "use_keyring")
	require.NoError(t, err)
	assert.Equal(t, "true", useKeyring)

	// Verify refresh token is NOT in config
	err = cfg.Write()
	require.NoError(t, err)
	configContent := mainBuf.String()
	assert.NotContains(t, configContent, "refresh-secret-token", "Refresh token should not be in plaintext config")
}

func Test_GetWithSource_RetrievesFromKeyringWhenUseKeyringSet(t *testing.T) {
	defer StubConfig(heredoc.Doc(`
		---
		hosts:
		  gitlab.com:
		    use_keyring: "true"
		    is_oauth2: true
	`), ``)()

	keyring.MockInit()

	// Store token in keyring with new key format
	err := keyring.Set("glab:gitlab.com:token", "", "glpat-from-keyring")
	require.NoError(t, err)

	// Store refresh token in keyring with new key format
	err = keyring.Set("glab:gitlab.com:oauth2_refresh_token", "", "refresh-from-keyring")
	require.NoError(t, err)

	cfg, err := ParseConfig("config.yml")
	require.NoError(t, err)

	// Retrieve token - should come from keyring, not config
	token, source, err := cfg.GetWithSource("gitlab.com", "token", false)
	require.NoError(t, err)
	assert.Equal(t, "glpat-from-keyring", token)
	assert.Equal(t, "keyring", source)

	// Retrieve refresh token - should come from keyring
	refreshToken, source, err := cfg.GetWithSource("gitlab.com", "oauth2_refresh_token", false)
	require.NoError(t, err)
	assert.Equal(t, "refresh-from-keyring", refreshToken)
	assert.Equal(t, "keyring", source)
}

func Test_GetWithSource_ErrorsWhenKeyringEnabledButTokenMissing(t *testing.T) {
	defer StubConfig(`---
hosts:
  gitlab.com:
    use_keyring: "true"
`, ``)()

	keyring.MockInit()
	// Don't store any token in keyring

	cfg, err := ParseConfig("config.yml")
	require.NoError(t, err)

	// Should error when trying to retrieve from keyring but token doesn't exist
	token, source, err := cfg.GetWithSource("gitlab.com", "token", false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "token not found in keyring")
	assert.Empty(t, token)
	assert.Empty(t, source)
}

func Test_SetKeyring_JobToken(t *testing.T) {
	mainBuf := bytes.Buffer{}
	hostsBuf := bytes.Buffer{}
	defer StubWriteConfig(&mainBuf, &hostsBuf)()

	keyring.MockInit()
	cfg := NewBlankConfig()

	// Enable keyring mode
	err := cfg.Set("gitlab.com", "use_keyring", "true")
	require.NoError(t, err)

	// Set a job token - should go to keyring
	err = cfg.Set("gitlab.com", "job_token", "job-token-value")
	require.NoError(t, err)

	// Verify job token is stored in keyring with new key format
	storedToken, err := keyring.Get("glab:gitlab.com:job_token", "")
	require.NoError(t, err)
	assert.Equal(t, "job-token-value", storedToken)

	// Verify use_keyring indicator is set
	useKeyring, err := cfg.Get("gitlab.com", "use_keyring")
	require.NoError(t, err)
	assert.Equal(t, "true", useKeyring)
}

func Test_SetKeyring_CleansUpExistingPlaintextToken(t *testing.T) {
	defer StubConfig(`---
hosts:
  gitlab.com:
    token: glpat-old-plaintext-token
`, ``)()

	mainBuf := bytes.Buffer{}
	hostsBuf := bytes.Buffer{}
	defer StubWriteConfig(&mainBuf, &hostsBuf)()

	keyring.MockInit()
	cfg, err := ParseConfig("config.yml")
	require.NoError(t, err)

	// Enable keyring mode
	err = cfg.Set("gitlab.com", "use_keyring", "true")
	require.NoError(t, err)

	// Set token - should go to keyring and remove plaintext token from config
	err = cfg.Set("gitlab.com", "token", "glpat-new-keyring-token")
	require.NoError(t, err)

	err = cfg.Write()
	require.NoError(t, err)

	// Verify old plaintext token is removed from config
	configContent := mainBuf.String()
	assert.NotContains(t, configContent, "glpat-old-plaintext-token")
	assert.NotContains(t, configContent, "glpat-new-keyring-token")
	assert.Contains(t, configContent, "use_keyring: \"true\"")

	// Verify new token is in keyring with new key format
	storedToken, err := keyring.Get("glab:gitlab.com:token", "")
	require.NoError(t, err)
	assert.Equal(t, "glpat-new-keyring-token", storedToken)
}

func Test_extractSubfolderFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "URL with subfolder",
			url:      "https://example.com/gitlab",
			expected: "gitlab",
		},
		{
			name:     "URL with nested subfolder",
			url:      "https://example.com/tools/gitlab",
			expected: "tools/gitlab",
		},
		{
			name:     "URL without subfolder",
			url:      "https://example.com",
			expected: "",
		},
		{
			name:     "URL with only slash",
			url:      "https://example.com/",
			expected: "",
		},
		{
			name:     "URL with trailing slash",
			url:      "https://example.com/gitlab/",
			expected: "gitlab",
		},
		{
			name:     "URL with port and subfolder",
			url:      "https://example.com:3000/gitlab",
			expected: "gitlab",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSubfolderFromURL(tt.url)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_GetFromEnvWithSource_CI_Subfolder(t *testing.T) {
	t.Run("subfolder from GITLAB_SUBFOLDER", func(t *testing.T) {
		t.Setenv("GITLAB_SUBFOLDER", "mysubfolder")
		t.Setenv("GLAB_ENABLE_CI_AUTOLOGIN", "")
		t.Setenv("GITLAB_CI", "")

		value, source := GetFromEnvWithSource("subfolder")
		assert.Equal(t, "mysubfolder", value)
		assert.Equal(t, "GITLAB_SUBFOLDER", source)
	})

	t.Run("subfolder from CI_SERVER_URL with path", func(t *testing.T) {
		t.Setenv("GITLAB_SUBFOLDER", "")
		t.Setenv("CI_SERVER_URL", "https://example.com/gitlab")
		t.Setenv("GLAB_ENABLE_CI_AUTOLOGIN", "true")
		t.Setenv("GITLAB_CI", "true")

		value, source := GetFromEnvWithSource("subfolder")
		assert.Equal(t, "gitlab", value)
		assert.Equal(t, "CI_SERVER_URL", source)
	})

	t.Run("subfolder from CI_SERVER_URL without path", func(t *testing.T) {
		t.Setenv("GITLAB_SUBFOLDER", "")
		t.Setenv("CI_SERVER_URL", "https://example.com")
		t.Setenv("GLAB_ENABLE_CI_AUTOLOGIN", "true")
		t.Setenv("GITLAB_CI", "true")

		value, source := GetFromEnvWithSource("subfolder")
		assert.Equal(t, "", value)
		assert.Equal(t, "", source)
	})

	t.Run("GITLAB_SUBFOLDER takes precedence over CI_SERVER_URL", func(t *testing.T) {
		t.Setenv("GITLAB_SUBFOLDER", "explicit")
		t.Setenv("CI_SERVER_URL", "https://example.com/gitlab")
		t.Setenv("GLAB_ENABLE_CI_AUTOLOGIN", "true")
		t.Setenv("GITLAB_CI", "true")

		value, source := GetFromEnvWithSource("subfolder")
		assert.Equal(t, "explicit", value)
		assert.Equal(t, "GITLAB_SUBFOLDER", source)
	})
}

func Test_GetFromEnvWithSource_CI_SSHHost(t *testing.T) {
	t.Run("ssh_host from GITLAB_SSH_HOST", func(t *testing.T) {
		t.Setenv("GITLAB_SSH_HOST", "ssh.example.com")
		t.Setenv("GLAB_ENABLE_CI_AUTOLOGIN", "")
		t.Setenv("GITLAB_CI", "")

		value, source := GetFromEnvWithSource("ssh_host")
		assert.Equal(t, "ssh.example.com", value)
		assert.Equal(t, "GITLAB_SSH_HOST", source)
	})

	t.Run("ssh_host from CI_SERVER_SHELL_SSH_HOST without port", func(t *testing.T) {
		t.Setenv("GITLAB_SSH_HOST", "")
		t.Setenv("CI_SERVER_SHELL_SSH_HOST", "git.example.com")
		t.Setenv("CI_SERVER_SHELL_SSH_PORT", "")
		t.Setenv("GLAB_ENABLE_CI_AUTOLOGIN", "true")
		t.Setenv("GITLAB_CI", "true")

		value, source := GetFromEnvWithSource("ssh_host")
		assert.Equal(t, "git.example.com", value)
		assert.Equal(t, "CI_SERVER_SHELL_SSH_HOST", source)
	})

	t.Run("ssh_host from CI_SERVER_SHELL_SSH_HOST with custom port", func(t *testing.T) {
		t.Setenv("GITLAB_SSH_HOST", "")
		t.Setenv("CI_SERVER_SHELL_SSH_HOST", "git.example.com")
		t.Setenv("CI_SERVER_SHELL_SSH_PORT", "2222")
		t.Setenv("GLAB_ENABLE_CI_AUTOLOGIN", "true")
		t.Setenv("GITLAB_CI", "true")

		value, source := GetFromEnvWithSource("ssh_host")
		assert.Equal(t, "git.example.com:2222", value)
		assert.Equal(t, "CI_SERVER_SHELL_SSH_HOST", source)
	})

	t.Run("ssh_host from CI_SERVER_SHELL_SSH_HOST with default port 22", func(t *testing.T) {
		t.Setenv("GITLAB_SSH_HOST", "")
		t.Setenv("CI_SERVER_SHELL_SSH_HOST", "git.example.com")
		t.Setenv("CI_SERVER_SHELL_SSH_PORT", "22")
		t.Setenv("GLAB_ENABLE_CI_AUTOLOGIN", "true")
		t.Setenv("GITLAB_CI", "true")

		value, source := GetFromEnvWithSource("ssh_host")
		assert.Equal(t, "git.example.com", value)
		assert.Equal(t, "CI_SERVER_SHELL_SSH_HOST", source)
	})

	t.Run("GITLAB_SSH_HOST takes precedence over CI_SERVER_SHELL_SSH_HOST", func(t *testing.T) {
		t.Setenv("GITLAB_SSH_HOST", "ssh.example.com:3000")
		t.Setenv("CI_SERVER_SHELL_SSH_HOST", "git.example.com")
		t.Setenv("CI_SERVER_SHELL_SSH_PORT", "2222")
		t.Setenv("GLAB_ENABLE_CI_AUTOLOGIN", "true")
		t.Setenv("GITLAB_CI", "true")

		value, source := GetFromEnvWithSource("ssh_host")
		assert.Equal(t, "ssh.example.com:3000", value)
		assert.Equal(t, "GITLAB_SSH_HOST", source)
	})
}
