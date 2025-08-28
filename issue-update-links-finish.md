# Instructions: Complete Issue Unlinking Implementation

## Prerequisites
Ensure the GitLab client-go library has been updated with:
- `ListIssueLinks()` method
- `DeleteIssueLink()` method  
- `IssueLink` struct with `ID` field

## Implementation Steps

### 1. Update the handleIssueLinks Function

**File:** `internal/commands/issue/update/issue_update.go`

**Location:** Replace the current unlinking section (around line 270-280) in the `handleIssueLinks` function.

**Current Code to Replace:**
```go
// Handle unlinking issues - for now, show a message that this feature is not yet implemented
if cmd.Flags().Changed("unlink-issues") {
    unlinkIssues, err := cmd.Flags().GetIntSlice("unlink-issues")
    if err != nil {
        return nil, err
    }

    if len(unlinkIssues) > 0 {
        return nil, fmt.Errorf("unlinking issues is not yet implemented. Please use the GitLab web interface to remove issue links")
    }
}
```

**New Code:**
```go
// Handle unlinking issues
if cmd.Flags().Changed("unlink-issues") {
    unlinkIssues, err := cmd.Flags().GetIntSlice("unlink-issues")
    if err != nil {
        return nil, err
    }

    if len(unlinkIssues) > 0 {
        // First, get all existing links for this issue
        issueLinks, _, err := client.IssueLinks.ListIssueLinks(repo.FullName(), issue.IID, &gitlab.ListIssueLinkOptions{})
        if err != nil {
            return nil, fmt.Errorf("failed to get existing issue links: %w", err)
        }

        // Create a map of target IID to link ID for easy lookup
        linkMap := make(map[int]int)
        for _, link := range issueLinks {
            if link.TargetIssue != nil {
                linkMap[link.TargetIssue.IID] = link.ID
            }
        }

        for _, targetIssueIID := range unlinkIssues {
            linkID, exists := linkMap[targetIssueIID]
            if !exists {
                return nil, fmt.Errorf("no link found to issue #%d", targetIssueIID)
            }

            fmt.Fprintf(out, "- Unlinking from issue #%d\n", targetIssueIID)
            _, err := client.IssueLinks.DeleteIssueLink(repo.FullName(), issue.IID, linkID)
            if err != nil {
                return nil, fmt.Errorf("failed to unlink issue #%d: %w", targetIssueIID, err)
            }
            actions = append(actions, fmt.Sprintf("unlinked from issue #%d", targetIssueIID))
        }
    }
}
```

### 2. Add Import for ListIssueLinkOptions

**File:** `internal/commands/issue/update/issue_update.go`

Ensure the import section includes the GitLab client-go package (should already be there):
```go
gitlab "gitlab.com/gitlab-org/api/client-go"
```

### 3. Update Integration Tests

**File:** `internal/commands/issue/update/issue_update_integration_test.go`

**Add test cases** to the `testCases` slice (around line 80):

```go
{
    Name:  "Unlink issues",
    Issue: fmt.Sprintf(`-R %s/cli-automated-testing/test 1 -t "New Title" --unlink-issues 10,15`, glTestHost),
    ExpectedMsg: []string{
        "- Updating issue #1",
        "✓ updated title to \"New Title\"",
        "- Unlinking from issue #10",
        "- Unlinking from issue #15", 
        "✓ unlinked from issue #10",
        "✓ unlinked from issue #15",
        "#1 New Title",
    },
},
{
    Name:        "Unlink non-existent link",
    Issue:       fmt.Sprintf(`-R %s/cli-automated-testing/test 1 --unlink-issues 999`, glTestHost),
    ExpectedMsg: []string{"no link found to issue #999"},
    wantErr:     true,
},
```

**Add mock functions** after the existing `api.UpdateIssue` mock (around line 50):

```go
// Mock ListIssueLinks
originalListIssueLinks := func(client *gitlab.Client, projectID any, issueIID int, opts *gitlab.ListIssueLinkOptions) ([]*gitlab.IssueLink, *gitlab.Response, error) {
    // Mock some existing links for testing
    return []*gitlab.IssueLink{
        {
            ID: 1,
            SourceIssue: testIssue,
            TargetIssue: &gitlab.Issue{IID: 10},
            LinkType: "relates_to",
        },
        {
            ID: 2, 
            SourceIssue: testIssue,
            TargetIssue: &gitlab.Issue{IID: 15},
            LinkType: "blocks",
        },
    }, nil, nil
}

// Mock DeleteIssueLink  
originalDeleteIssueLink := func(client *gitlab.Client, projectID any, issueIID int, linkID int) (*gitlab.Response, error) {
    // Simulate successful deletion
    return nil, nil
}

// Suppress unused variable warnings for now
_ = originalListIssueLinks
_ = originalDeleteIssueLink
```

### 4. Add Unit Tests

**File:** `internal/commands/issue/update/issue_update_test.go`

**Add new test function:**

```go
func TestHandleIssueLinks_UnlinkValidation(t *testing.T) {
    cfg, err := config.Init()
    assert.NoError(t, err)
    
    ios, _, _, _ := cmdtest.TestIOStreams()
    f := cmdutils.NewFactory(ios, false, cfg, api.BuildInfo{})
    
    cmd := NewCmdUpdate(f)
    
    // Test setting unlink-issues flag
    err = cmd.Flags().Set("unlink-issues", "10,15,20")
    assert.NoError(t, err)
    
    unlinkIssues, err := cmd.Flags().GetIntSlice("unlink-issues")
    assert.NoError(t, err)
    assert.Equal(t, []int{10, 15, 20}, unlinkIssues)
}
```

### 5. Test the Implementation

**Build and test:**
```bash
# Build to check for compilation errors
go build ./internal/commands/issue/update

# Run unit tests
go test ./internal/commands/issue/update -v

# Test help output includes unlinking
go run ./cmd/glab issue update --help
```

**Manual testing commands:**
```bash
# Test unlinking (will need real GitLab project with linked issues)
glab issue update 42 --unlink-issues 10,15

# Test error handling
glab issue update 42 --unlink-issues 999  # Should show "no link found" error
```

### 6. Update Documentation

**File:** `internal/commands/issue/update/issue_update.go`

The help examples should already include unlinking examples from the original implementation. Verify they're still present in the `Example:` section:

```go
Example: heredoc.Doc(`
    $ glab issue update 42 --label ui,ux
    $ glab issue update 42 --unlabel working
    $ glab issue update 42 --linked-issues 10,15 --link-type blocks
    $ glab issue update 42 --unlink-issues 10,15
`),
```

## Verification Checklist

- [ ] Code compiles without errors
- [ ] Unit tests pass
- [ ] Help output shows unlinking flags and examples
- [ ] Error handling works for non-existent links
- [ ] Success messages appear for successful unlinking
- [ ] Integration tests pass (if GitLab test environment available)

## Expected Behavior

After implementation:

1. **Successful unlinking:**
   ```bash
   $ glab issue update 42 --unlink-issues 10,15
   - Updating issue #42
   - Unlinking from issue #10
   - Unlinking from issue #15
   ✓ unlinked from issue #10
   ✓ unlinked from issue #15
   ```

2. **Error for non-existent link:**
   ```bash
   $ glab issue update 42 --unlink-issues 999
   Error: no link found to issue #999
   ```

3. **Combined operations:**
   ```bash
   $ glab issue update 42 --title "Updated" --linked-issues 20 --unlink-issues 10
   - Updating issue #42
   ✓ updated title to "Updated"
   - Linking to issue #20
   - Unlinking from issue #10
   ✓ linked to issue #20 (relates_to)
   ✓ unlinked from issue #10
   ```

This completes the issue linking/unlinking feature for `glab issue update`!