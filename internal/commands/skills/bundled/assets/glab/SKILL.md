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
glab issue create --title "title" --description "$(cat << 'EOF'
Your **markdown** description here.
Code blocks and `inline code` are safe.
EOF
)"
glab issue note <iid> -m "comment text"

# Merge requests
glab mr create --push --title "fix: title" --description "$(cat << 'EOF'
Your **markdown** description here.
Code blocks and `inline code` are safe.
EOF
)"
glab mr view <iid>
glab mr list --assignee <user>
glab mr update <iid> --description "$(cat << 'EOF'
Your **markdown** description here.
Code blocks and `inline code` are safe.
EOF
)"

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

## Comments and discussions

Always pass `-m` — without it, the next argument is treated as a filename.

```shell
# Root-level comments (issue / mr / incident)
glab issue note    <iid> -m "comment text"
glab mr note       <iid> -m "comment text"
glab incident note <iid> -m "comment text"

# Long/markdown body — single-quoted heredoc keeps backticks, $, \ literal
glab mr note <iid> -m "$(cat << 'EOF'
Your **markdown** comment.
Code blocks and `inline code` are safe.
EOF
)"

# Cross-project
glab mr note <iid> -m "..." --repo group/project
```

### Threaded replies

`glab mr note create` supports `--reply <discussion-id>` (full ID or 8+ char
prefix) for replying inside an MR thread, plus diff comments and thread state
management:

```shell
glab mr note create  <iid> --reply abc12345 -m "I agree!"
glab mr note create  <iid> --file main.go --line 42 -m "Needs refactoring"
glab mr note resolve <iid> <discussion-id>
glab mr note reopen  <iid> <discussion-id>
```

For issues, incidents, and work items addressed by IID, the CLI does not wrap
threaded replies. Fall back to `glab api` with the REST discussions endpoint:

```shell
glab api projects/:id/issues/<iid>/discussions/<discussion-id>/notes \
  -f body="reply text"
```

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

### Content-type guidance

Know the difference between `-f` and `-F` when POSTing with `glab api`:

```shell
# -f (--raw-field) — value is always a literal string
glab api projects/:id/issues/:iid/notes -f body="comment text"

# -F (--field) — @file reads from file, auto-infers types
glab api projects/:id/issues/:iid/notes -F body=@/tmp/comment.md

# ⚠️ --input sends raw bytes — requires explicit Content-Type header
glab api projects/:id/issues/:iid/notes --input file.json -H "Content-Type: application/json"
```

## Common mistakes

- **`glab issue note`, not `issue comment`** — use `-m` for the message body.
- **`glab issue note` requires `-m`** — without it, the next argument is
  treated as a filename, not the comment text.
- **`glab issue note` and `glab incident note` only post root-level
  comments** — use `glab mr note create --reply` for MRs, or
  `glab api .../discussions/<id>/notes -f body=...` for issues/incidents.
- **Prefer heredoc for Markdown bodies** — use `$(cat << 'EOF' ... EOF)` for
  descriptions and comments. The single-quoted delimiter prevents shell
  expansion of backticks, `$`, and backslashes. Fall back to
  `$(cat /tmp/file.md)` only for very long or reusable content.
- **`--input` requires an explicit `Content-Type` header** — `glab api --input
  file.json` sends raw bytes without setting `Content-Type: application/json`,
  causing HTTP 415. Add `-H "Content-Type: application/json"` or use `-f`/`-F`
  instead.
- **Always `--push` on `glab mr create`** — without it the remote branch
  may not exist and MR creation fails.
- **No `--state` on `mr list`** — use `--all`, `--merged`, or `--closed`.
- **No `--body` flag** — `--body` is a `gh` flag. `glab` uses `--description`.
- **Labels** — `--label` to add, `--unlabel` to remove. Scoped labels like
  `status::doing` auto-replace within their scope.
