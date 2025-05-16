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
func parseCommand(parts []string) (command, subcommand, fullCommand, flags string) {
	if len(parts) < 1 {
		return "", "", "", ""
	}

	command = parts[0] // command is the first part

	// where do the flags/parameters start?
	flagStartIndex := len(parts)
	for i := 1; i < len(parts); i++ {
		// check for things in "quotes" or -flags
		if strings.HasPrefix(parts[i], "-") || strings.Contains(parts[i], "\"") {
			flagStartIndex = i
			break
		}
	}

	if flagStartIndex > 1 {
		subcommandParts := parts[1:flagStartIndex]
		subcommand = strings.Join(subcommandParts, " ")
		// everything after this is presumably flags/parameters
	}

	fullCommand = command
	if subcommand != "" {
		fullCommand += " " + subcommand
	}

	if flagStartIndex < len(parts) {
		flags = strings.Join(parts[flagStartIndex:], " ")
	}

	return command, subcommand, fullCommand, flags
}

func sendTelemetryData(f *cmdutils.Factory, args []string) {
	var projectID int
	var namespaceID int

	command, subcommand, fullCommand, _ := parseCommand(args)

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
