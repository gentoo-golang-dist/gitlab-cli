package hooks

import (
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/commands/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
)

func AddTelemetryHook(f *cmdutils.Factory, args string) func() {
	arrayArgs := strings.Split(args, " ")

	return func() {
		go sendTelemetryData(f, arrayArgs)
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

	// "glab" will always be the first value
	// the command is the first part
	command = parts[1]

	subcommandParts := parts[2:]
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
