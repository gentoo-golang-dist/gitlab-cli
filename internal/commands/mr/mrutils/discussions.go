package mrutils

import (
	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
)

// ListAllDiscussions fetches all discussions for a merge request, paginating automatically.
var ListAllDiscussions = func(client *gitlab.Client, projectID any, mrIID int64, opts *gitlab.ListMergeRequestDiscussionsOptions) ([]*gitlab.Discussion, error) {
	if opts == nil {
		opts = &gitlab.ListMergeRequestDiscussionsOptions{}
	}
	if opts.PerPage == 0 {
		opts.PerPage = api.DefaultListLimit
	}

	var allDiscussions []*gitlab.Discussion
	page := opts.Page
	if page == 0 {
		page = 1
	}

	for {
		opts.Page = page
		discussions, resp, err := client.Discussions.ListMergeRequestDiscussions(projectID, mrIID, opts)
		if err != nil {
			return nil, err
		}

		allDiscussions = append(allDiscussions, discussions...)

		if resp == nil || resp.NextPage == 0 {
			break
		}
		page = resp.NextPage
	}

	return allDiscussions, nil
}

// FilterOpts specifies how to filter discussions.
type FilterOpts struct {
	// State filters by resolution status: "resolved", "unresolved", "resolvable", or "" for all.
	State string
	// Type filters by discussion type: "general", "diff", "system", or "" for all.
	Type string
	// FilePath filters to discussions on a specific file path.
	FilePath string
}

// FilterDiscussions filters discussions based on the provided options.
func FilterDiscussions(discussions []*gitlab.Discussion, opts FilterOpts) []*gitlab.Discussion {
	if opts.State == "" && opts.Type == "" && opts.FilePath == "" {
		return discussions
	}

	filtered := []*gitlab.Discussion{}

	for _, discussion := range discussions {
		if len(discussion.Notes) == 0 {
			continue
		}

		if !matchesState(discussion, opts.State) {
			continue
		}

		if !matchesType(discussion, opts.Type) {
			continue
		}

		if !matchesFilePath(discussion, opts.FilePath) {
			continue
		}

		filtered = append(filtered, discussion)
	}

	return filtered
}

// matchesState checks if a discussion matches the requested resolution state.
// Supported states: "resolved", "unresolved", "resolvable", or "" for all.
func matchesState(discussion *gitlab.Discussion, state string) bool {
	if state == "" {
		return true
	}

	hasResolvableNotes := false
	allResolved := true

	for _, note := range discussion.Notes {
		if note.Resolvable {
			hasResolvableNotes = true
			if !note.Resolved {
				allResolved = false
			}
		}
	}

	// Non-resolvable discussions don't match any resolution filter
	if !hasResolvableNotes {
		return false
	}

	switch state {
	case "resolved":
		return allResolved
	case "unresolved":
		return !allResolved
	case "resolvable":
		return true // already confirmed hasResolvableNotes above
	default:
		return true
	}
}

// matchesType checks if a discussion matches the requested type.
func matchesType(discussion *gitlab.Discussion, typ string) bool {
	if typ == "" {
		return true
	}

	firstNote := discussion.Notes[0]

	switch typ {
	case "system":
		return firstNote.System
	case "diff":
		return !firstNote.System && firstNote.Position != nil
	case "general":
		return !firstNote.System && firstNote.Position == nil
	default:
		return true
	}
}

// matchesFilePath checks if a discussion is on the specified file path.
func matchesFilePath(discussion *gitlab.Discussion, filePath string) bool {
	if filePath == "" {
		return true
	}

	firstNote := discussion.Notes[0]
	if firstNote.Position == nil {
		return false
	}

	return firstNote.Position.NewPath == filePath || firstNote.Position.OldPath == filePath
}
