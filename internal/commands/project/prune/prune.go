package prune

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/git"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	dryRun          bool
	yes             bool
	excludePatterns []string
	includeCurrent  bool
	useMergedFlag   bool

	io           *iostreams.IOStreams
	gitLabClient func() (*gitlab.Client, error)
	baseRepo     func() (glrepo.Interface, error)
	branch       func() (string, error)
	gitRunner    git.GitRunner
}

type candidate struct {
	branch       string
	mrIID        int64
	targetBranch string
}

func NewCmdPrune(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		gitLabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,
		branch:       f.Branch,
		gitRunner:    f.GitRunner(),
	}

	cmd := &cobra.Command{
		Use:   "prune [flags]",
		Short: "Delete local Git branches whose merge request has been merged.",
		Long: heredoc.Docf(`
			Delete local Git branches whose merge request has been merged on GitLab.

			By default, the command queries GitLab for each local branch and only
			deletes branches that have at least one merged merge request and no
			merge requests still open from the same source branch.

			Protected branches, the default branch, and the currently checked-out
			branch are never deleted.

			This command only affects your local Git repository. Remote branches
			on GitLab are not touched.

			Use --merged to skip the GitLab API check and prune branches based on
			%[1]sgit branch --merged%[1]s instead. This is faster but only detects
			fast-forward merges — squash and rebase merges look like distinct
			commits to Git and will not be reported as merged.
		`, "`"),
		Example: heredoc.Doc(`
			# Preview branches that would be deleted
			glab repo prune --dry-run

			# Delete branches with merged MRs (after confirmation)
			glab repo prune

			# Delete without confirmation
			glab repo prune --yes

			# Exclude additional branches by name or glob pattern
			glab repo prune --exclude wip-*,demo-branch

			# Use Git's local view of merged branches instead of querying GitLab
			glab repo prune --merged
		`),
		Args: cobra.NoArgs,
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return opts.run(cmd.Context())
		},
	}

	fl := cmd.Flags()
	fl.BoolVar(&opts.dryRun, "dry-run", false, "Preview branches that would be deleted without deleting them. (default false)")
	fl.BoolVarP(&opts.yes, "yes", "y", false, "Skip the confirmation prompt. (default false)")
	fl.StringSliceVarP(&opts.excludePatterns, "exclude", "e", nil, "Branch name or glob pattern to exclude. Comma-separated or repeated.")
	fl.BoolVar(&opts.includeCurrent, "include-current", false, "Allow pruning the currently checked-out branch. (default false)")
	fl.BoolVar(&opts.useMergedFlag, "merged", false, "Use 'git branch --merged' instead of querying GitLab. Detects fast-forward merges only. (default false)")

	return cmd
}

func (o *options) run(ctx context.Context) error {
	if !o.yes && !o.io.PromptEnabled() && !o.dryRun {
		return &cmdutils.FlagError{Err: errors.New("--yes or -y is required when not running interactively.")}
	}

	repo, err := o.baseRepo()
	if err != nil {
		return err
	}

	currentBranch, err := o.branch()
	if err != nil && !errors.Is(err, git.ErrNotOnAnyBranch) {
		return err
	}

	apiClient, err := o.gitLabClient()
	if err != nil {
		return err
	}

	project, err := api.GetProject(apiClient, repo.FullName())
	if err != nil {
		return fmt.Errorf("could not fetch project from GitLab: %w", err)
	}
	defaultBranch := project.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = git.DefaultBranchName
	}

	protectedPatterns, err := listProtectedBranchNames(apiClient, repo.FullName())
	if err != nil {
		return fmt.Errorf("could not fetch protected branches from GitLab: %w", err)
	}

	allBranches, err := git.ListLocalBranches(o.gitRunner)
	if err != nil {
		return err
	}

	excluded := buildMatcher(defaultBranch, currentBranch, o.includeCurrent, protectedPatterns, o.excludePatterns)

	candidateBranches := make([]string, 0, len(allBranches))
	for _, b := range allBranches {
		if !excluded(b) {
			candidateBranches = append(candidateBranches, b)
		}
	}

	var candidates []candidate
	if o.useMergedFlag {
		candidates, err = collectMergedLocally(o.gitRunner, defaultBranch, candidateBranches)
	} else {
		candidates, err = collectMergedViaAPI(o.io, apiClient, repo.FullName(), candidateBranches)
	}
	if err != nil {
		return err
	}

	c := o.io.Color()
	out := o.io.StdOut

	if len(candidates) == 0 {
		fmt.Fprintf(out, "%s No local branches found with merged merge requests.\n", c.GreenCheck())
		return nil
	}

	fmt.Fprintf(out, "Found %d branch(es) with merged merge requests:\n", len(candidates))
	for _, cand := range candidates {
		switch {
		case cand.mrIID != 0:
			fmt.Fprintf(out, "  %s %s (MR !%d → %s)\n", c.GreenCheck(), cand.branch, cand.mrIID, cand.targetBranch)
		default:
			fmt.Fprintf(out, "  %s %s (merged into %s)\n", c.GreenCheck(), cand.branch, cand.targetBranch)
		}
	}

	if o.dryRun {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Dry run: no branches were deleted.")
		return nil
	}

	if !o.yes {
		fmt.Fprintln(o.io.StdErr)
		confirmed := false
		if err := o.io.Confirm(ctx, &confirmed, fmt.Sprintf("Delete these %d local branch(es)?", len(candidates))); err != nil {
			return err
		}
		if !confirmed {
			fmt.Fprintln(o.io.StdErr, "aborted by user")
			return nil
		}
	}

	fmt.Fprintln(out)
	deleted := 0
	for _, cand := range candidates {
		if err := git.DeleteLocalBranch(cand.branch, o.gitRunner); err != nil {
			fmt.Fprintf(out, "  %s %s: %s\n", c.FailedIcon(), cand.branch, err)
			continue
		}
		fmt.Fprintf(out, "  %s deleted %s\n", c.GreenCheck(), cand.branch)
		deleted++
	}
	fmt.Fprintf(out, "\n%d branch(es) deleted.\n", deleted)
	return nil
}

func collectMergedViaAPI(io *iostreams.IOStreams, client *gitlab.Client, projectID string, branches []string) ([]candidate, error) {
	if len(branches) == 0 {
		return nil, nil
	}

	io.StartSpinner("Checking %d local branch(es) against GitLab...", len(branches))
	defer io.StopSpinner("")

	var candidates []candidate
	for _, b := range branches {
		mrs, err := api.ListMRs(client, projectID, &gitlab.ListProjectMergeRequestsOptions{
			SourceBranch: new(b),
		})
		if err != nil {
			return nil, fmt.Errorf("listing merge requests for branch %q: %w", b, err)
		}

		var merged *gitlab.BasicMergeRequest
		hasOpen := false
		for _, mr := range mrs {
			switch mr.State {
			case "merged":
				if merged == nil {
					merged = mr
				}
			case "opened", "locked":
				hasOpen = true
			}
		}
		if merged != nil && !hasOpen {
			candidates = append(candidates, candidate{
				branch:       b,
				mrIID:        merged.IID,
				targetBranch: merged.TargetBranch,
			})
		}
	}
	return candidates, nil
}

func collectMergedLocally(gr git.GitRunner, target string, branches []string) ([]candidate, error) {
	if len(branches) == 0 {
		return nil, nil
	}

	merged, err := git.ListMergedBranches(target, gr)
	if err != nil {
		return nil, err
	}
	mergedSet := make(map[string]struct{}, len(merged))
	for _, b := range merged {
		mergedSet[b] = struct{}{}
	}

	var candidates []candidate
	for _, b := range branches {
		if _, ok := mergedSet[b]; ok {
			candidates = append(candidates, candidate{branch: b, targetBranch: target})
		}
	}
	return candidates, nil
}

func listProtectedBranchNames(client *gitlab.Client, projectID string) ([]string, error) {
	opts := &gitlab.ListProtectedBranchesOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}
	var names []string
	for {
		protected, resp, err := client.ProtectedBranches.ListProtectedBranches(projectID, opts)
		if err != nil {
			return nil, err
		}
		for _, p := range protected {
			if p != nil && p.Name != "" {
				names = append(names, p.Name)
			}
		}
		if resp == nil || resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return names, nil
}

// buildMatcher returns a predicate that reports whether a branch should be
// excluded from pruning.
func buildMatcher(defaultBranch, currentBranch string, includeCurrent bool, protected, userPatterns []string) func(string) bool {
	var patterns []string
	if defaultBranch != "" {
		patterns = append(patterns, defaultBranch)
	}
	if currentBranch != "" && !includeCurrent {
		patterns = append(patterns, currentBranch)
	}
	patterns = append(patterns, protected...)
	for _, raw := range userPatterns {
		for p := range strings.SplitSeq(raw, ",") {
			if p = strings.TrimSpace(p); p != "" {
				patterns = append(patterns, p)
			}
		}
	}

	return func(branch string) bool {
		for _, p := range patterns {
			if branchMatches(p, branch) {
				return true
			}
		}
		return false
	}
}

func branchMatches(pattern, branch string) bool {
	if pattern == branch {
		return true
	}
	if strings.ContainsAny(pattern, "*?[") {
		if ok, err := filepath.Match(pattern, branch); err == nil && ok {
			return true
		}
	}
	return false
}
