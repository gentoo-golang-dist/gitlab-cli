package mrutils

import (
	"crypto/sha1"
	"fmt"
	"strconv"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/diff"
)

// GetLatestDiffVersion fetches MR diff versions and returns the latest one
// (with diffs included).
var GetLatestDiffVersion = func(client *gitlab.Client, project string, mrIID int64) (*gitlab.MergeRequestDiffVersion, error) {
	versions, _, err := client.MergeRequests.GetMergeRequestDiffVersions(project, mrIID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list MR diff versions: %w", err)
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("no diff versions found for MR !%d", mrIID)
	}
	// First version in the list is the latest
	latest := versions[0]
	full, _, err := client.MergeRequests.GetSingleMergeRequestDiffVersion(project, mrIID, latest.ID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch diff version %d: %w", latest.ID, err)
	}
	return full, nil
}

// FindFileDiff finds a file's diff in the given version by matching NewPath or OldPath.
func FindFileDiff(version *gitlab.MergeRequestDiffVersion, filePath string) (*gitlab.Diff, error) {
	for _, d := range version.Diffs {
		if d.NewPath == filePath || d.OldPath == filePath {
			return d, nil
		}
	}
	return nil, fmt.Errorf("file %q not found in MR diff", filePath)
}

// BuildDiffPosition builds a PositionOptions for a diff comment.
// lineStart/lineEnd refer to new-side lines (lineEnd > lineStart for multiline).
// oldLine refers to an old-side (removed) line.
// For file-level comments, pass lineStart=0 and oldLine=0.
func BuildDiffPosition(version *gitlab.MergeRequestDiffVersion, fileDiff *gitlab.Diff, lineStart, lineEnd, oldLine int) (*gitlab.PositionOptions, error) {
	pos := &gitlab.PositionOptions{
		BaseSHA:      gitlab.Ptr(version.BaseCommitSHA),
		HeadSHA:      gitlab.Ptr(version.HeadCommitSHA),
		StartSHA:     gitlab.Ptr(version.StartCommitSHA),
		NewPath:      gitlab.Ptr(fileDiff.NewPath),
		OldPath:      gitlab.Ptr(fileDiff.OldPath),
		PositionType: gitlab.Ptr("text"),
	}

	lines := diff.Parse(fileDiff.Diff)

	switch {
	case lineStart == 0 && oldLine == 0:
		// File-level comment: target first changed or context line
		for _, l := range lines {
			if l.Type == diff.Added {
				pos.NewLine = gitlab.Ptr(int64(l.NewLine))
				break
			}
			if l.Type == diff.Unchanged && l.NewLine > 0 {
				pos.NewLine = gitlab.Ptr(int64(l.NewLine))
				pos.OldLine = gitlab.Ptr(int64(l.OldLine))
				break
			}
		}

	case oldLine > 0:
		// Targeting an old-side (removed) line
		_, lt, err := diff.FindOldLine(lines, oldLine)
		if err != nil {
			return nil, fmt.Errorf("old line %d not found in diff for %s", oldLine, fileDiff.OldPath)
		}
		pos.OldLine = gitlab.Ptr(int64(oldLine))
		if lt == diff.Unchanged {
			newLine, _, _ := diff.FindOldLine(lines, oldLine)
			pos.NewLine = gitlab.Ptr(int64(newLine))
		}

	default:
		// Targeting a new-side line (possibly a range)
		oldLineNum, lt, err := diff.FindNewLine(lines, lineStart)
		if err != nil {
			return nil, fmt.Errorf("line %d not found in diff for %s", lineStart, fileDiff.NewPath)
		}
		pos.NewLine = gitlab.Ptr(int64(lineStart))
		if lt == diff.Unchanged {
			pos.OldLine = gitlab.Ptr(int64(oldLineNum))
		}

		// Multiline range
		if lineEnd > lineStart {
			pos.LineRange = &gitlab.LineRangeOptions{
				Start: &gitlab.LinePositionOptions{
					LineCode: gitlab.Ptr(lineCode(fileDiff.NewPath, lineStart)),
					Type:     gitlab.Ptr("new"),
				},
				End: &gitlab.LinePositionOptions{
					LineCode: gitlab.Ptr(lineCode(fileDiff.NewPath, lineEnd)),
					Type:     gitlab.Ptr("new"),
				},
			}
		}
	}

	return pos, nil
}

// ParseLine parses a line flag value like "42" or "10:15" into start and end line numbers.
// For a single line, start == end.
func ParseLine(s string) (start, end int, err error) {
	if s == "" {
		return 0, 0, nil
	}
	parts := strings.SplitN(s, ":", 2)
	start, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid line number %q", s)
	}
	if len(parts) == 2 {
		end, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("invalid line range %q", s)
		}
		if end < start {
			return 0, 0, fmt.Errorf("invalid line range %q: end must be >= start", s)
		}
	} else {
		end = start
	}
	return start, end, nil
}

// ResolveDiscussionID resolves a prefix (8+ chars) to a full discussion ID.
// Returns an error if the prefix is ambiguous or not found.
var ResolveDiscussionID = func(client *gitlab.Client, project string, mrIID int64, prefix string) (string, error) {
	if len(prefix) < 8 {
		return "", fmt.Errorf("discussion ID prefix must be at least 8 characters, got %d", len(prefix))
	}
	discussions, err := ListAllDiscussions(client, project, mrIID)
	if err != nil {
		return "", err
	}
	var matches []string
	for _, d := range discussions {
		if len(d.ID) >= len(prefix) && d.ID[:len(prefix)] == prefix {
			matches = append(matches, d.ID)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("no discussion found matching prefix %q", prefix)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("prefix %q matches %d discussions: %s, %s", prefix, len(matches), matches[0][:12], matches[1][:12])
	}
}

// ListAllDiscussions fetches all MR discussions with pagination.
var ListAllDiscussions = func(client *gitlab.Client, project string, mrIID int64) ([]*gitlab.Discussion, error) {
	var all []*gitlab.Discussion
	opts := &gitlab.ListMergeRequestDiscussionsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}
	for {
		discussions, resp, err := client.Discussions.ListMergeRequestDiscussions(project, mrIID, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list discussions: %w", err)
		}
		all = append(all, discussions...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

// lineCode generates a GitLab line_code for multiline ranges.
// Format: sha1(file_path)_oldline_newline
func lineCode(path string, line int) string {
	h := sha1.Sum([]byte(path))
	return fmt.Sprintf("%x_%d_%d", h, 0, line)
}
