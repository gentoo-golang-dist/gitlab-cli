package searchutils

import (
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

// DetectScope determines the appropriate search scope based on explicit flags
// and the git repository context.
//
// Resolution order:
//  1. If group is non-empty → ScopeGroup (repo flag was already handled by cmdutils)
//  2. If baseRepo returns a valid repo → ScopeProject
//  3. Otherwise → ScopeInstance
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

	// No project context — fall back to instance-level search.
	return ScopeInstance, "", nil, nil
}
