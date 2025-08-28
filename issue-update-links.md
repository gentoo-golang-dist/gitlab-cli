# Plan: Add Issue Linking Support to `glab issue update`

## Analysis Summary
Currently, `glab issue update` only supports basic issue properties but not linking. Issue linking is only available during `glab issue create` using the GitLab IssueLinks API.

## Required Changes

### 1. Update CLI Flags (`issue_update.go:198-212`)
- Add `--linked-issues` flag (comma-separated list of IIDs)  
- Add `--link-type` flag (defaults to "relates_to")
- Add `--unlink-issues` flag (comma-separated list of IIDs to remove)

### 2. Update Command Logic (`issue_update.go:69-181`)
- Parse new linking flags
- Add linking logic after the main `UpdateIssue` call
- Use `apiClient.IssueLinks.CreateIssueLink()` for new links
- Use `apiClient.IssueLinks.DeleteIssueLink()` for unlinking
- Add appropriate action messages for user feedback

### 3. Add Helper Functions
- Link management function similar to `postCreateActions` in create command
- Error handling for invalid IIDs and link operations
- Support for different link types: `relates_to`, `blocks`, `blocked_by`

### 4. Update Tests (`issue_update_integration_test.go`)
- Add test cases for linking/unlinking scenarios
- Mock the IssueLinks API calls
- Test error conditions (invalid IIDs, API failures)

### 5. Update Help Documentation
- Add examples showing linking usage
- Document available link types
- Show unlinking examples

## Technical Implementation Details

The implementation will follow the same pattern as the existing `issue create` command:
1. Parse flags during command execution
2. Perform the main issue update via `gitlab.UpdateIssueOptions` 
3. Handle linking as a separate post-update operation using `gitlab.IssueLinks` API
4. Provide user feedback for each linking action

## Files to Modify
- `internal/commands/issue/update/issue_update.go` (main implementation)
- `internal/commands/issue/update/issue_update_integration_test.go` (tests)

This approach maintains consistency with existing glab patterns and leverages the same GitLab API calls already used in issue creation.

## Example Usage (Proposed)

```bash
# Link issue #42 to issues #10 and #15 with "blocks" relationship
glab issue update 42 --linked-issues 10,15 --link-type blocks

# Remove links to issues #10 and #15 from issue #42
glab issue update 42 --unlink-issues 10,15

# Add a "relates_to" link (default type)
glab issue update 42 --linked-issues 20
```

## API Reference

Based on the existing `issue create` implementation, the GitLab API calls used will be:

- `apiClient.IssueLinks.CreateIssueLink(repo, issueIID, options)` 
- `apiClient.IssueLinks.DeleteIssueLink(repo, issueIID, linkID)` (may need to list links first)

Where `CreateIssueLinkOptions` includes:
- `TargetIssueIID`: The IID of the issue to link to
- `LinkType`: The type of relationship ("relates_to", "blocks", "blocked_by")