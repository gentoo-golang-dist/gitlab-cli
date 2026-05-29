package checkout

import (
	"context"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
	"gitlab.com/gitlab-org/cli/internal/git"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type mrCheckoutConfig struct {
	branch   string
	track    bool
	upstream string
	force    bool
}

var mrCheckoutCfg mrCheckoutConfig

func NewCmdCheckout(f cmdutils.Factory) *cobra.Command {
	mrCheckoutCmd := &cobra.Command{
		Use:   "checkout [<id> | <branch> | <url>]",
		Short: "Check out an open merge request.",
		Long: heredoc.Docf(`
			Defaults to the currently checked-out branch. Use %[1]s--branch%[1]s to
			override the local branch name used for the checkout.
		`, "`"),
		Example: heredoc.Doc(`
			glab mr checkout 1
			glab mr checkout branch
			glab mr checkout 12 --branch todo-fix
			glab mr checkout new-feature --set-upstream-to=upstream/main
			glab mr checkout https://gitlab.com/gitlab-org/cli/-/merge_requests/1234

			# Uses the checked-out branch
			glab mr checkout`),
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			gr := f.GitRunner()

			var err error
			var upstream string

			if mrCheckoutCfg.upstream != "" {
				upstream = mrCheckoutCfg.upstream

				if val := strings.Split(mrCheckoutCfg.upstream, "/"); len(val) > 1 {
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
			}

			client, err := f.GitLabClient()
			if err != nil {
				return err
			}

			mr, _, err := mrutils.MRFromArgs(cmd.Context(), f, args, "any")
			if err != nil {
				return err
			}

			if mrCheckoutCfg.branch == "" {
				mrCheckoutCfg.branch = mr.SourceBranch
			}

			var mrRef string
			var mrProject *gitlab.Project

			mrProject, err = api.GetProject(client, mr.SourceProjectID)
			if err != nil {
				// If we don't have access to the source project, let's try the target project
				mrProject, err = api.GetProject(client, mr.TargetProjectID)
				if err != nil {
					return err
				} else {
					// We found the target project, let's find the ref another way
					mrRef = fmt.Sprintf("refs/merge-requests/%d/head", mr.IID)
				}

			} else {
				mrRef = fmt.Sprintf("refs/heads/%s", mr.SourceBranch)
			}

			// Get the appropriate remote URL based on user's git_protocol preference
			baseRepo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			cfg := f.Config()
			gitProtocol, err := cfg.Get(baseRepo.RepoHost(), "git_protocol")
			if err != nil {
				return err
			}
			repoURL := glrepo.RemoteURL(mrProject, gitProtocol)

			io := f.IO()
			ctx := cmd.Context()
			fetchRefSpec := fmt.Sprintf("%s:%s", mrRef, mrCheckoutCfg.branch)
			if err := gr.GitWithIO(io.StdOut, io.StdErr, "fetch", repoURL, fetchRefSpec); err != nil {
				// Remote diverged from local. Fall back to fetching just the ref (FETCH_HEAD only).
				if err := gr.GitWithIO(io.StdOut, io.StdErr, "fetch", repoURL, mrRef); err != nil {
					return err
				}
				if err := resolveDivergence(ctx, io, gr, mrCheckoutCfg.branch, mrCheckoutCfg.force); err != nil {
					return err
				}
			}

			// .remote is needed for `git pull` to work
			// .pushRemote is needed for `git push` to work, if user has set `remote.pushDefault`.
			// see https://git-scm.com/docs/git-config#Documentation/git-config.txt-branchltnamegtremote
			if _, err := gr.Git("config", fmt.Sprintf("branch.%s.remote", mrCheckoutCfg.branch), repoURL); err != nil {
				return err
			}
			if mr.AllowCollaboration {
				if _, err := gr.Git("config", fmt.Sprintf("branch.%s.pushRemote", mrCheckoutCfg.branch), repoURL); err != nil {
					return err
				}
			}
			if _, err := gr.Git("config", fmt.Sprintf("branch.%s.merge", mrCheckoutCfg.branch), mrRef); err != nil {
				return err
			}

			if err := gr.GitWithIO(io.StdOut, io.StdErr, "checkout", mrCheckoutCfg.branch); err != nil {
				return fmt.Errorf("could not checkout branch %q: %w", mrCheckoutCfg.branch, err)
			}

			if upstream != "" {
				if _, err := gr.Git("branch", "--set-upstream-to", upstream); err != nil {
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
	mrCheckoutCmd.Flags().BoolVarP(&mrCheckoutCfg.force, "force", "f", false, "Reset local branch to remote when they have diverged. Refuses if working tree has changes that would be lost.")
	return mrCheckoutCmd
}

// resolveDivergence is called after the fallback fetch wrote FETCH_HEAD but the
// local branch ref was not updated. Decides whether to reset, ask the user, or
// abort. The force flag bypasses the user prompt; otherwise an interactive
// prompt confirms (TTY) or a FlagError is returned (non-TTY).
func resolveDivergence(ctx context.Context, io *iostreams.IOStreams, gr git.GitRunner, branch string, force bool) error {
	diverged, err := localDivergesFromFetchHead(gr, branch)
	if err != nil {
		return err
	}
	if !diverged {
		return nil
	}

	onTarget, err := guardReset(gr, branch)
	if err != nil {
		return err
	}

	if !force {
		if !io.PromptEnabled() {
			return cmdutils.FlagError{Err: fmt.Errorf("local branch %q has diverged from remote. Pass --force to reset it (discards local commits on this branch)", branch)}
		}
		var confirmed bool
		if err := io.Confirm(ctx, &confirmed, fmt.Sprintf("Local branch %q has diverged from remote. Reset to remote (discards local commits on this branch)?", branch)); err != nil {
			return err
		}
		if !confirmed {
			return cmdutils.CancelError()
		}
	}
	return applyReset(io, gr, branch, onTarget)
}

// localDivergesFromFetchHead reports whether the local branch ref points at a
// different commit than FETCH_HEAD. A missing local branch is treated as "no
// divergence" so downstream code surfaces any error.
func localDivergesFromFetchHead(gr git.GitRunner, branch string) (bool, error) {
	localSHA, err := gr.Git("rev-parse", "--verify", "refs/heads/"+branch)
	if err != nil {
		return false, nil
	}
	fetchSHA, err := gr.Git("rev-parse", "FETCH_HEAD^{commit}")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(localSHA) != strings.TrimSpace(fetchSHA), nil
}

// guardReset checks whether `git reset --hard FETCH_HEAD` is safe to run for
// the named branch. Returns whether HEAD is currently the target branch, or an
// error describing why the reset would destroy uncommitted work.
func guardReset(gr git.GitRunner, branch string) (bool, error) {
	currentBranch, symErr := gr.Git("symbolic-ref", "--quiet", "--short", "HEAD")
	onTarget := symErr == nil && strings.TrimSpace(currentBranch) == branch
	if !onTarget {
		return false, nil
	}
	unsafe, err := unsafeToReset(gr)
	if err != nil {
		return false, err
	}
	if unsafe {
		return false, fmt.Errorf("working tree has changes that would be lost by reset. Commit, stash, or remove them before resetting branch %q", branch)
	}
	return true, nil
}

// applyReset mutates the local ref to match FETCH_HEAD. Callers must invoke
// guardReset first and use its returned onTarget value.
func applyReset(io *iostreams.IOStreams, gr git.GitRunner, branch string, onTarget bool) error {
	if onTarget {
		return gr.GitWithIO(io.StdOut, io.StdErr, "reset", "--hard", "FETCH_HEAD")
	}
	return gr.GitWithIO(io.StdOut, io.StdErr, "branch", "-f", branch, "FETCH_HEAD")
}

// unsafeToReset reports whether `git reset --hard FETCH_HEAD` would
// silently destroy uncommitted work. Two conditions trigger it:
//  1. Tracked files with modified or staged changes (reset --hard discards them).
//  2. Untracked files whose paths appear in FETCH_HEAD's tree (reset --hard
//     overwrites them silently — unlike `git checkout`, which refuses).
//
// Unrelated untracked files (local notes, build artifacts) are NOT a blocker
// since reset --hard leaves them alone.
func unsafeToReset(gr git.GitRunner) (bool, error) {
	tracked, err := gr.Git("diff", "--name-only", "HEAD")
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(tracked) != "" {
		return true, nil
	}
	untracked, err := gr.Git("ls-files", "--others", "--exclude-standard")
	if err != nil {
		return false, err
	}
	if strings.TrimSpace(untracked) == "" {
		return false, nil
	}
	incoming, err := gr.Git("ls-tree", "-r", "--name-only", "FETCH_HEAD")
	if err != nil {
		return false, err
	}
	incomingPaths := map[string]struct{}{}
	for line := range strings.SplitSeq(incoming, "\n") {
		if line != "" {
			incomingPaths[line] = struct{}{}
		}
	}
	for line := range strings.SplitSeq(untracked, "\n") {
		if line == "" {
			continue
		}
		if _, conflict := incomingPaths[line]; conflict {
			return true, nil
		}
	}
	return false, nil
}
