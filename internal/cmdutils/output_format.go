package cmdutils

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/iostreams"
)

// EnableJSONOutput adds the --output/-F flag to a command for JSON output
// support, and also registers --jq via AddJQFlag so callers do not have to
// invoke both helpers.
//
// By default it uses a standard description. Pass a custom description to
// override.
func EnableJSONOutput(cmd *cobra.Command, io *iostreams.IOStreams, outputFormat *string, customDescription ...string) {
	description := "Format output as: text, json."
	if len(customDescription) > 0 && customDescription[0] != "" {
		description = customDescription[0]
	}

	cmd.Flags().VarP(
		NewEnumValue([]string{"text", "json"}, "text", outputFormat),
		"output",
		"F",
		description,
	)
	AddJQFlag(cmd, io)
}
