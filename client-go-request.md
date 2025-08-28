# GitLab Client-Go Enhancement Request: Issue Links Management

## Current State

The GitLab client-go library currently provides limited support for issue links management:

### What Works Now
- **CreateIssueLink**: `client.IssueLinks.CreateIssueLink(projectID, issueIID, options)` 
  - Creates a new link between issues
  - Takes `CreateIssueLinkOptions` with `TargetIssueIID` and `LinkType`
  - Returns `*gitlab.IssueLink` containing source and target issue information

### What's Missing
The library lacks methods to:
1. **List existing issue links** for a given issue
2. **Delete specific issue links** by link ID

## Required Enhancements

### 1. List Issue Links Method

**Method Signature Needed:**
```go
func (s *IssueLinksService) ListIssueLinks(pid interface{}, issue int, opt *ListIssueLinkOptions, options ...RequestOptionFunc) ([]*IssueLink, *Response, error)
```

**Options Structure:**
```go
type ListIssueLinkOptions struct {
    ListOptions
}
```

**What it should do:**
- Retrieve all issue links for a specific issue
- Return an array of `IssueLink` objects
- Each `IssueLink` should contain:
  - `ID`: The unique identifier of the link (needed for deletion)
  - `SourceIssue`: The issue that contains the link
  - `TargetIssue`: The issue being linked to
  - `LinkType`: The relationship type ("relates_to", "blocks", "blocked_by")

**GitLab API Endpoint:**
- `GET /projects/:id/issues/:issue_iid/links`
- Documentation: https://docs.gitlab.com/ee/api/issue_links.html#list-issue-links

### 2. Delete Issue Link Method

**Method Signature Needed:**
```go
func (s *IssueLinksService) DeleteIssueLink(pid interface{}, issue int, issueLinkID int, options ...RequestOptionFunc) (*Response, error)
```

**What it should do:**
- Delete a specific issue link by its ID
- Take the project ID, source issue IID, and the link ID
- Return only a response (no data payload needed)

**GitLab API Endpoint:**
- `DELETE /projects/:id/issues/:issue_iid/links/:issue_link_id`
- Documentation: https://docs.gitlab.com/ee/api/issue_links.html#delete-an-issue-link

## Data Structures

### Current IssueLink Structure
The existing `IssueLink` struct should include (if not already present):

```go
type IssueLink struct {
    ID          int    `json:"id"`
    SourceIssue *Issue `json:"source_issue"`
    TargetIssue *Issue `json:"target_issue"`
    LinkType    string `json:"link_type"`
}
```

**Critical Field:** The `ID` field is essential for the delete operation.

## Implementation Requirements

### Service Interface
The `IssueLinksServiceInterface` should be updated to include:

```go
type IssueLinksServiceInterface interface {
    CreateIssueLink(pid interface{}, issue int, opt *CreateIssueLinkOptions, options ...RequestOptionFunc) (*IssueLink, *Response, error)
    ListIssueLinks(pid interface{}, issue int, opt *ListIssueLinkOptions, options ...RequestOptionFunc) ([]*IssueLink, *Response, error)
    DeleteIssueLink(pid interface{}, issue int, issueLinkID int, options ...RequestOptionFunc) (*Response, error)
}
```

## Use Case Context

This enhancement is needed for the `glab issue update --unlink-issues` functionality, which requires:

1. **Listing existing links** to find the link IDs for issues that need to be unlinked
2. **Deleting specific links** by their IDs

The workflow would be:
1. User runs: `glab issue update 42 --unlink-issues 10,15`
2. Code calls `ListIssueLinks(project, 42, options)` to get all existing links
3. Code finds links where `TargetIssue.IID` matches 10 or 15
4. Code calls `DeleteIssueLink(project, 42, linkID)` for each matching link

## Priority

**High Priority** - This blocks the completion of a user-requested feature that provides parity with the `glab issue create` command's linking capabilities.