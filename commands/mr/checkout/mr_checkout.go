package checkout

import (
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/api"
	"gitlab.com/gitlab-org/cli/commands/cmdutils"
	"gitlab.com/gitlab-org/cli/commands/mr/mrutils"
	"gitlab.com/gitlab-org/cli/pkg/git"
)

type mrCheckoutConfig struct {
	branch   string
	track    bool
	upstream string
}

var mrCheckoutCfg mrCheckoutConfig

func NewCmdCheckout(f *cmdutils.Factory) *cobra.Command {
	mrCheckoutCmd := &cobra.Command{
		Use:   "checkout [<id> | <branch>]",
		Short: "Check out an open merge request.",
		Long:  ``,
		Example: heredoc.Doc(`
			- glab mr checkout 1
			- glab mr checkout branch
			- glab mr checkout 12 --branch todo-fix
			- glab mr checkout new-feature --set-upstream-to=upstream/main

			Uses the checked-out branch
			- glab mr checkout
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Validate upstream if provided
			if err := validateUpstream(f, mrCheckoutCfg.upstream); err != nil {
				return err
			}

			apiClient, err := f.HttpClient()
			if err != nil {
				return err
			}

			mr, _, err := mrutils.MRFromArgs(f, args, "any")
			if err != nil {
				return err
			}

			if mrCheckoutCfg.branch == "" {
				mrCheckoutCfg.branch = mr.SourceBranch
			}

			mrRef, remoteURL, err := getMRRef(apiClient, mr)
			if err != nil {
				return err
			}

			// Handle branch comparison and update (returns current branch info)
			currentBranch, err := handleBranchUpdate(cmd, mrRef, remoteURL, args)
			if err != nil {
				return err
			}

			// Setup MR branch configuration
			if err := setupMRBranchConfig(mrCheckoutCfg.branch, remoteURL, mrRef, mr.AllowCollaboration); err != nil {
				return err
			}

			var gr git.StandardGitCommand
			if currentBranch != mrCheckoutCfg.branch {
				err := git.CheckoutBranch(mrCheckoutCfg.branch, gr)
				if err != nil {
					return err
				}
			}

			if mrCheckoutCfg.upstream != "" {
				if err := git.RunCmd([]string{"branch", "--set-upstream-to", mrCheckoutCfg.upstream}); err != nil {
					return err
				}
			}
			return nil
		},
	}
	mrCheckoutCmd.Flags().StringVarP(&mrCheckoutCfg.branch, "branch", "b", "", "Check out merge request with name <branch>.")
	mrCheckoutCmd.Flags().BoolVarP(&mrCheckoutCfg.track, "track", "t", true, "Set checked out branch to track the remote branch.")
	_ = mrCheckoutCmd.Flags().MarkDeprecated("track", "Now enabled by default")
	mrCheckoutCmd.Flags().StringVarP(&mrCheckoutCfg.upstream, "set-upstream-to", "u", "", "Set tracking of checked-out branch to [REMOTE/]BRANCH.")
	return mrCheckoutCmd
}

// validateUpstream validates the upstream configuration
func validateUpstream(f *cmdutils.Factory, upstream string) error {
	if upstream == "" {
		return nil
	}

	if val := strings.Split(upstream, "/"); len(val) > 1 {
		// Verify that we have the remote set
		repo, err := f.Remotes()
		if err != nil {
			return err
		}
		_, err = repo.FindByName(val[0])
		if err != nil {
			return err
		}
	}
	return nil
}

// getMRRef gets ref and remoteURL for the merge request
func getMRRef(apiClient *gitlab.Client, mr *gitlab.MergeRequest) (string, string, error) {
	var mrRef string
	var mrProject *gitlab.Project

	mrProject, err := api.GetProject(apiClient, mr.SourceProjectID)
	if err != nil {
		// If we don't have access to the source project, try the target project
		mrProject, err = api.GetProject(apiClient, mr.TargetProjectID)
		if err != nil {
			return "", "", err
		}
		// Use merge request ref for target project
		mrRef = fmt.Sprintf("refs/merge-requests/%d/head", mr.IID)
	} else {
		mrRef = fmt.Sprintf("refs/heads/%s", mr.SourceBranch)
	}

	// Get the preferred remote URL format based on current branch config
	remoteURL, err := getPreferredRemoteURL(mrProject)
	if err != nil {
		return "", "", fmt.Errorf("failed to determine remote URL: %w", err)
	}

	return mrRef, remoteURL, nil
}

func getPreferredRemoteURL(mrProject *gitlab.Project) (string, error) {
	currentBranch, err := git.CurrentBranch()
	if err != nil {
		// Can't determine current branch, default to SSH
		return mrProject.SSHURLToRepo, nil
	}

	branchConfig := git.ReadBranchConfig(currentBranch)

	// Check if branch has direct remote URL configured (rare case)
	if branchConfig.RemoteURL != nil {
		return matchingProtocolURL(branchConfig.RemoteURL.String(), mrProject), nil
	}

	// Check if branch uses named remote (common case)
	if branchConfig.RemoteName != "" {
		remoteURL, err := git.GetRemoteURL(branchConfig.RemoteName)
		if err == nil {
			return matchingProtocolURL(remoteURL, mrProject), nil
		}
	}

	// No branch config found, default to SSH
	return mrProject.SSHURLToRepo, nil
}

// matchingProtocolURL returns the GitLab project URL that matches the existing URL's protocol
func matchingProtocolURL(existingURL string, mrProject *gitlab.Project) string {
	if strings.HasPrefix(existingURL, "git@") || strings.HasPrefix(existingURL, "ssh://") {
		return mrProject.SSHURLToRepo
	}
	return mrProject.HTTPURLToRepo
}

func handleBranchUpdate(cmd *cobra.Command, mrRef, remoteURL string, args []string) (string, error) {
	branchExists := git.HasLocalBranch(mrCheckoutCfg.branch)
	currentBranch, _ := git.CurrentBranch()
	isOnTargetBranch := currentBranch == mrCheckoutCfg.branch

	if branchExists {
		tempRef := fmt.Sprintf("refs/remotes/mr-checkout-temp/%s", mrCheckoutCfg.branch)

		if err := git.FetchRefToTempRef(remoteURL, mrRef, tempRef); err != nil {
			return currentBranch, fmt.Errorf("failed to fetch merge request updates: %w", err)
		}

		comparison, err := git.CompareBranches(mrCheckoutCfg.branch, tempRef)

		defer func() {
			_ = git.DeleteRef(tempRef) // Ignore cleanup errors
		}()

		if comparison.IsUpToDate {
			if isOnTargetBranch {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ Already on branch '%s' which is up to date with the merge request.\n", mrCheckoutCfg.branch)
				return currentBranch, nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ Branch '%s' is up to date with the merge request.\n", mrCheckoutCfg.branch)
		} else if comparison.HasDiverged {
			return currentBranch, handleDivergedBranch(cmd, comparison, mrCheckoutCfg.branch, args)
		} else if comparison.IsAhead {
			return currentBranch, handleAheadBranch(cmd, comparison, mrCheckoutCfg.branch)
		} else if comparison.IsBehind {
			if err := handleBehindBranch(cmd, mrCheckoutCfg.branch, comparison.BehindCount); err != nil {
				return currentBranch, err
			}
		}

		err = git.UpdateBranchToRef(mrCheckoutCfg.branch, remoteURL, mrRef, currentBranch, branchExists)
		return currentBranch, err
	}

	err := git.UpdateBranchToRef(mrCheckoutCfg.branch, remoteURL, mrRef, currentBranch, branchExists)
	return currentBranch, err
}

// setupMRBranchConfig sets up git config for a merge request branch
func setupMRBranchConfig(branchName, remoteURL, mergeRef string, allowCollaboration bool) error {
	// Set remote for git pull
	if err := git.RunCmd([]string{"config", fmt.Sprintf("branch.%s.remote", branchName), remoteURL}); err != nil {
		return fmt.Errorf("failed to set branch remote: %w", err)
	}

	// Set push remote if collaboration is allowed
	if allowCollaboration {
		if err := git.RunCmd([]string{"config", fmt.Sprintf("branch.%s.pushRemote", branchName), remoteURL}); err != nil {
			return fmt.Errorf("failed to set branch push remote: %w", err)
		}
	}

	// Set merge ref
	if err := git.RunCmd([]string{"config", fmt.Sprintf("branch.%s.merge", branchName), mergeRef}); err != nil {
		return fmt.Errorf("failed to set branch merge ref: %w", err)
	}

	return nil
}

// handleDivergedBranch handles when a branch has diverged from the MR
func handleDivergedBranch(cmd *cobra.Command, comparison *git.BranchComparison, branchName string, args []string) error {
	fmt.Fprintf(cmd.OutOrStdout(), "\n⚠️  Your local branch '%s' has diverged from the merge request:\n", branchName)
	fmt.Fprintf(cmd.OutOrStdout(), "   - %s commit(s) ahead (local commits not in MR)\n", comparison.AheadCount)
	fmt.Fprintf(cmd.OutOrStdout(), "   - %s commit(s) behind (MR commits not in local)\n", comparison.BehindCount)
	fmt.Fprintf(cmd.OutOrStdout(), "\n❌ Cannot update branch: You have local commits that would be lost.\n")
	return fmt.Errorf("branch has diverged from merge request")
}

// handleAheadBranch handles when a local branch is ahead of the MR
func handleAheadBranch(cmd *cobra.Command, comparison *git.BranchComparison, branchName string) error {
	fmt.Fprintf(cmd.OutOrStdout(), "\n⚠️  Your local branch '%s' is %s commit(s) ahead of the merge request.\n", branchName, comparison.AheadCount)
	fmt.Fprintf(cmd.OutOrStdout(), "\n❌ Cannot update branch: You have local commits that would be lost.\n")
	return fmt.Errorf("local commits would be lost")
}

// handleBehindBranch handles when a local branch is behind the MR
func handleBehindBranch(cmd *cobra.Command, branchName, behindCount string) error {
	fmt.Fprintf(cmd.OutOrStdout(), "\n⚠️  Your local branch '%s' is %s commit(s) behind the merge request.\n", branchName, behindCount)

	// Check for uncommitted changes
	hasUncommitted, changeCount, err := git.HasUncommittedChanges()
	if err != nil {
		return fmt.Errorf("failed to check for uncommitted changes: %w", err)
	}

	if hasUncommitted {
		fmt.Fprintf(cmd.OutOrStdout(), "\n❌ Cannot update branch: You have %d uncommitted change(s) that would be lost.\n", changeCount)
		return fmt.Errorf("uncommitted changes would be lost")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\nUpdating branch to match the merge request...\n")
	return nil
}
