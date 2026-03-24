//go:build !integration

package cmdutils

import (
	"net/url"
	"testing"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/glinstance"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
)

func TestFactory_ResolveHostNameFromConfig(t *testing.T) {
	// GIVEN
	cfg := config.NewFromString(heredoc.Doc(`
		host: gitlab.example.com
	`))

	// WHEN
	f := NewFactory(nil, false, cfg, api.BuildInfo{})

	// THEN
	assert.Equal(t, "gitlab.example.com", f.defaultHostname)
}

func TestFactory_ResolveHostNameFromEnv(t *testing.T) {
	// GIVEN
	t.Setenv("GITLAB_HOST", "gitlab.example.com")
	cfg := config.NewFromString(heredoc.Doc(`
		host: another.gitlab.example.com
	`))

	// WHEN
	f := NewFactory(nil, false, cfg, api.BuildInfo{})

	// THEN
	assert.Equal(t, "gitlab.example.com", f.defaultHostname)
}

func TestFactory_ResolveToGitLabComByDefault(t *testing.T) {
	// GIVEN
	cfg := config.NewBlankConfig()

	// WHEN
	f := NewFactory(nil, false, cfg, api.BuildInfo{})

	// THEN
	assert.Equal(t, "gitlab.com", f.defaultHostname)
}

func TestFactory_GitLabClientUsesCorrectHost(t *testing.T) {
	// GIVEN
	tests := []struct {
		name                         string
		cfg                          config.Config
		env                          map[string]string
		expectedGitLabClientHostname *url.URL
	}{
		{
			name:                         "default",
			cfg:                          config.NewBlankConfig(),
			expectedGitLabClientHostname: mustURL(t, "https://gitlab.com/api/v4/"),
		},
		{
			name: "host from config",
			cfg: config.NewFromString(heredoc.Doc(`
				host: gitlab.example.com
			`)),
			expectedGitLabClientHostname: mustURL(t, "https://gitlab.example.com/api/v4/"),
		},
		{
			name: "host from env",
			env:  map[string]string{"GITLAB_HOST": "gitlab.example.com"},
			cfg: config.NewFromString(heredoc.Doc(`
				host: another.gitlab.example.com
			`)),
			expectedGitLabClientHostname: mustURL(t, "https://gitlab.example.com/api/v4/"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// GIVEN
			if tt.env != nil {
				for k, v := range tt.env {
					t.Setenv(k, v)
				}
			}

			// WHEN
			f := NewFactory(nil, false, tt.cfg, api.BuildInfo{})
			c, err := f.GitLabClient()
			require.NoError(t, err)

			// THEN
			assert.Equal(t, tt.expectedGitLabClientHostname, c.BaseURL())
		})
	}
}

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()

	u, err := url.Parse(s)
	require.NoError(t, err)

	return u
}

func TestFactory_RepoOverride_numericProjectID(t *testing.T) {
	defer func(orig func(*gitlab.Client, any) (*gitlab.Project, error)) {
		api.GetProject = orig
	}(api.GetProject)

	api.GetProject = func(_ *gitlab.Client, projectID any) (*gitlab.Project, error) {
		assert.EqualValues(t, int64(99), projectID)
		return &gitlab.Project{HTTPURLToRepo: "https://gitlab.com/acme/widgets.git"}, nil
	}

	cfg := config.NewFromString(heredoc.Doc(`
		hosts:
		  gitlab.com:
		    token: test-token
	`))
	f := NewFactory(nil, false, cfg, api.BuildInfo{})

	err := f.RepoOverride("99")
	require.NoError(t, err)

	baseRepo, err := f.BaseRepo()
	require.NoError(t, err)
	assert.Equal(t, "acme/widgets", baseRepo.FullName())
	assert.False(t, glrepo.IsProjectIDOnly(baseRepo))
	assert.Equal(t, glinstance.DefaultHostname, baseRepo.RepoHost())
}
