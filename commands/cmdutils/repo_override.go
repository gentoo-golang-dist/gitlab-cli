package cmdutils

import (
	"github.com/spf13/cobra"
)

func EnableRepoOverride(cmd *cobra.Command, f Factory) {
	// if the flag doesn't exist yet, create it
	if cmd.PersistentFlags().Lookup("repo") == nil {
		cmd.PersistentFlags().StringP("repo", "R", "", "Select another repository. Can use either `OWNER/REPO` or `GROUP/NAMESPACE/REPO` format. Also accepts full URL or Git URL.")
	}

	// unhide the flag for this command and its children
	if flag := cmd.PersistentFlags().Lookup("repo"); flag != nil {
		flag.Hidden = false
	}
}

// AddGlobalRepoOverride adds the -R flag globally but keeps it hidden
func AddGlobalRepoOverride(cmd *cobra.Command, f Factory) {
	cmd.PersistentFlags().StringP("repo", "R", "", "Select another repository. Can use either `OWNER/REPO` or `GROUP/NAMESPACE/REPO` format. Also accepts full URL or Git URL.")
	_ = cmd.PersistentFlags().MarkHidden("repo")
}
