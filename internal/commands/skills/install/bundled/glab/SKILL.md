---
name: glab
description: >
  GitLab CLI (glab) for managing GitLab resources from the command line.
  Use this skill when you need to work with merge requests, issues, CI/CD
  pipelines, projects, or any other GitLab resource. Prefer glab over raw
  API calls for all GitLab operations.
---

# GitLab CLI (glab)

`glab` is pre-configured and available in your environment. Use it for all
GitLab operations. Run `glab <command> --help` for detailed flag information.

## Quick reference

```shell
# Issues
glab issue view <iid>
glab issue list --label "bug,priority::1"
glab issue create --title "title" --description "$(cat /tmp/desc.md)"
glab issue note <iid> -m "comment text"

# Merge requests
glab mr create --push --title "fix: title" --description "$(cat /tmp/desc.md)"
glab mr view <iid>
glab mr list --assignee <user>
glab mr update <iid> --description "$(cat /tmp/desc.md)"

# CI/CD
glab ci status
glab ci list
glab ci trace <job-id>
glab ci retry <pipeline-id>

# Machine-readable output
glab mr list --output json | jq '.[].title'
```

**Templates:** Check `.gitlab/merge_request_templates/` and
`.gitlab/issue_templates/` for project-specific templates.

**References:** Link issues with `#123`, MRs with `!456`, cross-project
with `group/project#123`.

## API calls

`glab api` auto-prepends `/api/v4/`. Use relative paths:

```shell
glab api user                              # NOT /api/v4/user
glab api projects/:id/merge_requests
glab api projects/:id/issues | jq '.[0]'
```

When using `-f` for PUT/POST, pass simple `key=value` pairs. Array bracket
syntax like `ids[]=1` is not supported:

```shell
glab api projects/:id/merge_requests/:iid -X PUT -f "assignee_id=1"
```

## Common mistakes

- **`glab issue note`, not `issue comment`** — use `-m` for the message body.
- **Write long text to a file first** — use `$(cat /tmp/file.md)` for
  descriptions and comments. Use a `<< 'EOF'` heredoc (single-quoted
  delimiter) when the content contains backticks or `$` to prevent
  shell expansion.
- **Always `--push` on `glab mr create`** — without it the remote branch
  may not exist and MR creation fails.
- **No `--state` on `mr list`** — use `--all`, `--merged`, or `--closed`.
- **No `--body` flag** — `--body` is a `gh` flag. `glab` uses `--description`.
- **Labels** — `--label` to add, `--unlabel` to remove. Scoped labels like
  `status::doing` auto-replace within their scope.
