package config

import (
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	compileCmd "gitlab.com/gitlab-org/cli/internal/commands/ci/config/compile"
)

func NewDeprecatedConfigCompileCmd(f cmdutils.Factory) *cobra.Command {
	configCmd := &cobra.Command{
		Use:    "config",
		Hidden: true,
	}

	configCmd.AddCommand(compileCmd.NewCmdConfigCompile(f))
	return configCmd
}
