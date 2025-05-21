package hooks

import (
	"testing"

	"github.com/stretchr/testify/require"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	gitlab_testing "gitlab.com/gitlab-org/api/client-go/testing"
	"go.uber.org/mock/gomock"

	"gitlab.com/gitlab-org/cli/commands/cmdtest"
	"gitlab.com/gitlab-org/cli/commands/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
)

func Test_sendTelemetryData(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		command     string
		subcommand  string
		fullCommand string
	}{
		{
			name:        "command with args",
			args:        []string{"hey", "subcmd", "-p", "blah"},
			command:     "hey",
			subcommand:  "subcmd",
			fullCommand: "hey subcmd",
		},
		{
			name:        "command with no subcommand",
			args:        []string{"issue", "-l", "bug"},
			command:     "issue",
			subcommand:  "",
			fullCommand: "issue",
		},
		{
			name:        "command with multiple subcommands",
			args:        []string{"mr", "create", "new", "--title", "Test"},
			command:     "mr",
			subcommand:  "create new",
			fullCommand: "mr create new",
		},
		{
			name:        "command with no flags",
			args:        []string{"ci", "status"},
			command:     "ci",
			subcommand:  "status",
			fullCommand: "ci status",
		},
		{
			name:        "single command only",
			args:        []string{"version"},
			command:     "version",
			subcommand:  "",
			fullCommand: "version",
		},
		{
			name:        "command with flags having values",
			args:        []string{"repo", "view", "--web", "--token", "abc123"},
			command:     "repo",
			subcommand:  "view",
			fullCommand: "repo view",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := gitlab_testing.NewTestClient(t)
			ios, _, _, _ := cmdtest.InitIOStreams(true, "")

			f := &cmdutils.Factory{
				IO: ios,
				HttpClient: func() (*gitlab.Client, error) {
					return tc.Client, nil
				},
				BaseRepo: func() (glrepo.Interface, error) {
					return glrepo.New("OWNER", "REPO"), nil
				},
			}

			project := gitlab.Project{
				ID:        123,
				Namespace: &gitlab.ProjectNamespace{ID: 123},
			}

			tc.MockProjects.EXPECT().
				GetProject(gomock.Any(), gomock.Any(), gomock.Any()).
				Return(&project, &gitlab.Response{}, nil)

			tc.MockUsageData.EXPECT().
				TrackEvent(&gitlab.TrackEventOptions{
					Event:          "gitlab_cli_command_used",
					NamespaceID:    gitlab.Ptr(project.Namespace.ID),
					ProjectID:      gitlab.Ptr(project.ID),
					SendToSnowplow: gitlab.Ptr(true),
					AdditionalProperties: map[string]string{
						"label":                  tt.command,
						"property":               tt.subcommand,
						"command_and_subcommand": tt.fullCommand,
					},
				})

			sendTelemetryData(f, tt.args)
		})
	}
}

func Test_parseCommand(t *testing.T) {
	tests := []struct {
		name        string
		cmdString   []string
		command     string
		subcommand  string
		fullCommand string
		flags       string
	}{
		{
			name:        "basic command",
			cmdString:   []string{"mr", "list", "--web", "-R", "blah", "-p", "3"},
			command:     "mr",
			subcommand:  "list",
			fullCommand: "mr list",
			flags:       "--web -R blah -p 3",
		},
		{
			name:        "multiple subcommands",
			cmdString:   []string{"mr", "list", "thing", "--web", "-R", "blah", "-p", "3"},
			command:     "mr",
			subcommand:  "list thing",
			fullCommand: "mr list thing",
			flags:       "--web -R blah -p 3",
		},
		{
			name:        "strip out parameters",
			cmdString:   []string{"mr", "list", "thing", "\"also\"", "--web", "-R", "blah", "-p", "3"},
			command:     "mr",
			subcommand:  "list thing",
			fullCommand: "mr list thing",
			flags:       "\"also\" --web -R blah -p 3",
		},
		{
			name:        "strip out single quote parameters",
			cmdString:   []string{"mr", "list", "thing", "'also'"},
			command:     "mr",
			subcommand:  "list thing",
			fullCommand: "mr list thing",
			flags:       "'also'",
		},
		{
			name:        "no subcommand",
			cmdString:   []string{"mr", "--web", "-R", "blah", "-p", "3"},
			command:     "mr",
			subcommand:  "",
			fullCommand: "mr",
			flags:       "--web -R blah -p 3",
		},
		{
			name:        "no flags",
			cmdString:   []string{"mr", "list", "thing"},
			command:     "mr",
			subcommand:  "list thing",
			fullCommand: "mr list thing",
			flags:       "",
		},
		{
			name:        "too short of a command",
			cmdString:   []string{},
			command:     "",
			subcommand:  "",
			fullCommand: "",
			flags:       "",
		},
		{
			name:        "many subcommands",
			cmdString:   []string{"mr", "list", "thing", "a", "b", "c", "--flag"},
			command:     "mr",
			subcommand:  "list thing a b c",
			fullCommand: "mr list thing a b c",
			flags:       "--flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			command, subcommand, fullCommand, flags := parseCommand(tt.cmdString)

			require := require.New(t)
			require.Equal(tt.command, command)
			require.Equal(tt.subcommand, subcommand)
			require.Equal(tt.fullCommand, fullCommand)
			require.Equal(tt.flags, flags)
		})
	}
}

func TestIsTelemetryEnabled(t *testing.T) {
	tests := []struct {
		name           string
		configYaml     string
		expectedResult bool
	}{
		{
			name:           "enabled with 'true' value",
			configYaml:     "telemetry: true",
			expectedResult: true,
		},
		{
			name:           "enabled with '1' value",
			configYaml:     "telemetry: '1'",
			expectedResult: true,
		},
		{
			name:           "disabled with 'false' value",
			configYaml:     "telemetry: false",
			expectedResult: false,
		},
		{
			name:           "disabled with '0' value",
			configYaml:     "telemetry: '0'",
			expectedResult: false,
		},
		{
			name:           "enabled with empty value",
			configYaml:     "telemetry: ''",
			expectedResult: true,
		},
		{
			name:           "enabled with other value",
			configYaml:     "telemetry: something",
			expectedResult: true,
		},
		{
			name:           "no config value set",
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			restore := config.StubConfig(tt.configYaml, "")
			defer restore()

			cfg, err := config.ParseConfig("config.yml")
			if tt.configYaml == "" {
				cfg = config.NewBlankConfig()
			} else {
				require.NoError(t, err)
			}

			result := IsTelemetryEnabled(cfg)

			require.Equal(t, tt.expectedResult, result)
		})
	}
}
