package hooks

import (
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/commands/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
)

func AddTelemetryHook(f *cmdutils.Factory, args []string) func() {
	return func() {
		go sendTelemetryData(f, args)
	}
}

// IsTelemetryEnabled checks if usage data is disabled via config or env var
func IsTelemetryEnabled(cfg config.Config) bool {
	disableTelemetry, _ := cfg.Get("", "telemetry")
	if disableTelemetry == "false" || disableTelemetry == "0" {
		return false
	}

	return true
}

// parseCommand parses a command string and returns components
func parseCommand(parts []string) (command, subcommand, fullCommand string) {
	if len(parts) < 1 {
		return "", "", ""
	}

	// the command is the first value
	command = parts[0]

	subcommandParts := parts[1:]
	subcommand = strings.Join(subcommandParts, " ")

	fullCommand = command
	if subcommand != "" {
		fullCommand += " " + subcommand
	}

	return command, subcommand, fullCommand
}

func sendTelemetryData(f *cmdutils.Factory, args []string) {
	var projectID int
	var namespaceID int

	command, subcommand, fullCommand := parseCommand(args)

	client, _ := f.HttpClient()

	repo, _ := f.BaseRepo()

	project, err := repo.Project(client)
	if err == nil {
		projectID = project.ID
		namespaceID = project.Namespace.ID
	}

	if client != nil {
		_, _ = client.UsageData.TrackEvent(&gitlab.TrackEventOptions{
			Event:          "gitlab_cli_command_used",
			NamespaceID:    gitlab.Ptr(namespaceID),
			ProjectID:      gitlab.Ptr(projectID),
			SendToSnowplow: gitlab.Ptr(true),
			AdditionalProperties: map[string]string{
				"label":                  command,
				"property":               subcommand,
				"command_and_subcommand": fullCommand,
			},
		})
	}
}
