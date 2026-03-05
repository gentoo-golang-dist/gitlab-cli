package cmdutils

import (
	"github.com/spf13/cobra"
)

// EnableJSONOutput adds the --output/-F flag to a command for JSON output support.
// By default, it uses a standard description. Pass a custom description to override.
func EnableJSONOutput(cmd *cobra.Command, outputFormat *string, customDescription ...string) {
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
}
