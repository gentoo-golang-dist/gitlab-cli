package searchutils

import (
	"strings"

	"gitlab.com/gitlab-org/cli/internal/glrepo"
)

// SearchScope represents the level at which a search is performed.
type SearchScope int

const (
	// ScopeInstance searches across the whole GitLab instance.
	ScopeInstance SearchScope = iota
	// ScopeGroup searches within a specific group.
	ScopeGroup
	// ScopeProject searches within a specific project.
	ScopeProject
)

// isNoRepoContextError reports whether err indicates that no git repository
// or remote context was found (as opposed to, e.g., an auth or network error).
// We check for the known "no git remotes found" message produced by the remote
// resolver, as well as common "not in a git repo" patterns used in tests and
// other command contexts. Errors that don't match these patterns are treated as
// real failures and propagated to the caller.
func isNoRepoContextError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "no git remotes") ||
		strings.Contains(msg, "not a git repo") ||
		strings.Contains(msg, "not in a git repo") ||
		strings.Contains(msg, "could not find default branch")
}

// DetectScope determines the appropriate search scope based on explicit flags
// and the git repository context.
//
// Resolution order:
//  1. If group is non-empty → ScopeGroup (repo flag was already handled by cmdutils)
//  2. If baseRepo returns a valid repo → ScopeProject
//  3. If baseRepo returns a "no repo context" error → ScopeInstance
//  4. If baseRepo returns any other error → propagate the error
//
// Note: when both -g and -R are set, group takes precedence.
// This matches the behavior of other glab commands and can be revisited.
func DetectScope(group string, baseRepo func() (glrepo.Interface, error)) (SearchScope, string, glrepo.Interface, error) {
	if group != "" {
		return ScopeGroup, group, nil, nil
	}

	repo, err := baseRepo()
	if err == nil && repo != nil {
		return ScopeProject, "", repo, nil
	}

	if err != nil && !isNoRepoContextError(err) {
		// Propagate unexpected errors (auth failure, network issues, etc.)
		return ScopeInstance, "", nil, err
	}

	// No project context — fall back to instance-level search.
	return ScopeInstance, "", nil, nil
}
