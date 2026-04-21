package cmdutils

import (
	"os"

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

	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		repoOverride, err := cmd.Flags().GetString("repo")
		if err != nil {
			return err
		}
		if repoFromEnv := os.Getenv("GITLAB_REPO"); repoOverride == "" && repoFromEnv != "" {
			repoOverride = repoFromEnv
		}
		if repoOverride != "" {
			return f.RepoOverride(repoOverride)
		}
		return nil
	}
}
