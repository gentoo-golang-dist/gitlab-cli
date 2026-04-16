package cmdutils

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// groupViper is a dedicated viper instance for resolving the GITLAB_GROUP
// environment variable. Using a private instance instead of the global viper
// avoids concurrent writes to shared state when GroupOverride is called from
// parallel goroutines, and ensures the setup is unaffected by any call to
// viper.Reset() elsewhere.
var groupViper = func() *viper.Viper {
	inst := viper.New()
	inst.SetEnvPrefix("GITLAB")
	inst.MustBindEnv("group")
	return inst
}()

func GroupOverride(cmd *cobra.Command) (string, error) {
	// Get group from env
	groupFromEnv := groupViper.GetString("group")

	// Get group/repo flags
	group, err := cmd.Flags().GetString("group")
	if err != nil {
		return "", err
	}
	repo, err := cmd.Flags().GetString("repo")
	if err != nil {
		return "", err
	}

	// Determine which group to use based on env and repo/group flags
	switch {
	case repo != "":
		// Ignore any groups if the repo flag is set
		return "", nil
	case group != "":
		// Use the group flag if set and repo flag is not set
		return group, nil
	default:
		// Consider environment if no repo or group flags are set
		return groupFromEnv, nil
	}
}
