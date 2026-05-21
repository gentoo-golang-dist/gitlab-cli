package mrutils

import (
	"fmt"
	"strconv"
	"strings"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
)

// MRRef is a parsed merge request reference. Repo is nil when the reference
// targets the caller's default project (e.g. a bare IID like "123"); when the
// reference is a full GitLab URL, Repo carries the project and host parsed
// from the URL so callers can route the API call to the right place.
type MRRef struct {
	Repo glrepo.Interface
	IID  int
}

// ParseMRRef parses a single merge request reference. Accepted forms:
//
//	123                                                  // bare IID in the default project
//	https://gitlab.com/group/project/-/merge_requests/7  // any project, any host
//
// The `!7` and `group/project!7` sigil forms are intentionally not supported:
// other glab MR commands don't expose them either, and we don't want to
// establish that convention here.
func ParseMRRef(ref, defaultHostname string) (MRRef, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return MRRef{}, fmt.Errorf("empty merge request reference")
	}

	if iid, repo := cmdutils.ParseMergeRequestFromURL(ref, defaultHostname); iid != 0 {
		return MRRef{Repo: repo, IID: iid}, nil
	}

	iid, err := strconv.Atoi(ref)
	if err != nil || iid <= 0 {
		return MRRef{}, fmt.Errorf("invalid merge request reference %q: expected an IID (e.g. 123) or a merge request URL", ref)
	}
	return MRRef{IID: iid}, nil
}

// ParseMRRefs parses a list of merge request references, returning the first
// parse error so callers can fail fast before any API round-trip.
func ParseMRRefs(refs []string, defaultHostname string) ([]MRRef, error) {
	out := make([]MRRef, 0, len(refs))
	for _, r := range refs {
		parsed, err := ParseMRRef(r, defaultHostname)
		if err != nil {
			return nil, err
		}
		out = append(out, parsed)
	}
	return out, nil
}
