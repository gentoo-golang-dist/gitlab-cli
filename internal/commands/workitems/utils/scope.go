package utils

import (
	"fmt"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/workitems/api"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
)

// DetectScope determines the appropriate scope (group or project) for work items queries.
func DetectScope(groupFlag string, baseRepo func() (glrepo.Interface, error)) (*api.ScopeInfo, error) {
	// Priority 1: Explicit --group flag
	if groupFlag != "" {
		return &api.ScopeInfo{
			Type: api.ScopeTypeGroup,
			Path: groupFlag,
		}, nil
	}

	// Priority 2: Current repository context
	repo, err := baseRepo()
	if err == nil {
		return &api.ScopeInfo{
			Type: api.ScopeTypeProject,
			Path: repo.FullName(),
		}, nil
	}

	// Priority 3: No context available
	return nil, cmdutils.FlagError{
		Err: fmt.Errorf("unable to determine scope: run from within a project, specify --group, or use -R/ --repo flag"),
	}
}
