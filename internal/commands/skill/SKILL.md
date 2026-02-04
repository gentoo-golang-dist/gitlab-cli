---
name: glab
description: Work with GitLab from the command line using glab. Use when creating, viewing, or managing GitLab issues, epics, work-items, merge requests, CI/CD pipelines, releases, and repositories. Enables automation of GitLab workflows without leaving the terminal.
---

# glab

glab is the official GitLab CLI tool for working with issues, merge requests, pipelines, and more from the terminal.

## Context Detection

glab automatically detects the GitLab project from the current Git repository's remote URL. Most commands work without specifying a project when run inside a Git repo.

To target a different project, use the `-R` or `--repo` flag:

```bash
glab issue list -R gitlab-org/gitlab
glab mr view 123 -R owner/project
```

## Authentication

The user should already be logged in to glab. On authentication errors, STOP and ask the user to check their login status with `glab auth status` or log in with `glab auth login`.

## Commands

glab has many commands. Run `glab <command> --help` for detailed usage. Available commands:

<% DYNAMIC_COMMAND_LIST %>

## Example Workflows

### Understanding an issue before implementation

Fetch issue details to understand requirements before starting work:

```bash
glab issue view 456
glab issue view 456 --comments   # Include discussion thread
glab issue --help                # Full issue command reference
```

### Fixing a failed pipeline

When a pipeline fails, diagnose the issue:

```bash
glab ci status                   # Quick status of current branch
glab ci view                     # See all jobs and their status
glab ci trace                    # Stream logs from the failed job
glab ci --help                   # Full ci command reference
```

### Creating an MR from the current branch

After committing changes, create a merge request:

```bash
glab mr create --fill            # Auto-fill title/description from commits
glab mr create --fill --draft    # Create as draft if not ready for review
glab mr create --fill --web      # Create and open in browser
glab mr create --help            # Full MR creation command reference
```

For longer descriptions, write to a file first to avoid shell escaping issues:

```bash
glab mr create --title "feat: add feature" --description "$(cat /tmp/mr-description.md)"
```

### Checking assigned work

List issues and MRs assigned to the user:

```bash
glab issue list --assignee=@me   # Issues assigned to the user
glab mr list --reviewer=@me      # MRs awaiting the user's review
glab mr list --author=@me        # The user's open MRs
```

## GitLab References

GitLab uses special syntax for cross-referencing items:

- `#123` - References issue 123
- `!456` - References merge request 456
- `Closes #123` - In MR description or commit, auto-closes issue when merged
- `Related to #123` - Creates a link without auto-closing

## Tips

- **Get help**: `glab <command> --help` for detailed options
- **JSON output**: `--output json` for machine-readable output (where supported)
- **Filtering**: Use `--label`, `--state`, `--assignee`, `--author` to filter lists
- **Templates**: Check `.gitlab/merge_request_templates/` and `.gitlab/issue_templates/` for project-specific templates when creating MRs or issues
