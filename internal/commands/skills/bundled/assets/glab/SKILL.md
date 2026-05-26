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
glab issue create --title "title" --description "$(cat body.md)"
glab issue note <iid> -m "comment text"

# Merge requests
glab mr create --push --title "fix: title" --description "$(cat body.md)"
glab mr view <iid>
glab mr list --assignee <user>
glab mr update <iid> --description "$(cat body.md)"

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

## Non-interactive use (agents)

Never rely on commands that open `$EDITOR` — they block waiting for a TTY.
The interactive paths in `glab` are:

- `glab issue note <iid>` / `glab incident note <iid>` without `-m`
- `glab mr note <iid>` / `glab mr note create <iid>` without `-m` **on a TTY**
  (on a pipe, these read stdin instead — see below)
- `--description "-"` on `glab issue create`, `glab mr create`, `glab mr update`

Always pass `-m`, pipe to stdin where supported, or pass an explicit
`--description` value.

## Comments and discussions

### Short, inline bodies — pass `-m`

```shell
glab issue note    <iid> -m "comment text"
glab mr note       <iid> -m "comment text"
glab incident note <iid> -m "comment text"

# Cross-project
glab mr note <iid> -m "..." --repo group/project
```

### Long or Markdown bodies — pipe to stdin (preferred for MR notes)

`glab mr note` and `glab mr note create` read the body from stdin when their
input is a pipe. This is the safest pattern for agents: no argv-length limit,
no shell-quoting pitfalls, no escape bugs from backticks, `$`, or backslashes.

```shell
# From a file
glab mr note create <iid> < body.md

# From a variable, safely
printf '%s' "$BODY" | glab mr note create <iid>

# Inline literal multi-line body — quoted heredoc, no shell expansion inside
glab mr note create <iid> << 'EOF'
Your **markdown** comment.
Code blocks and `inline code`, $variables, and \backslashes are all literal.
EOF
```

`glab issue note` and `glab incident note` do **not** read stdin. For long
bodies on those commands, use `glab api` with `-F body=@file` (see
[Content-type guidance](#content-type-guidance)) or inline a quoted heredoc
into `-m`:

```shell
glab issue note <iid> -m "$(cat << 'EOF'
Your **markdown** comment.
Code blocks and `inline code` are safe.
EOF
)"
```

For descriptions on `glab issue create` / `glab mr create` / `glab mr update`,
the same trade-off applies: a quoted heredoc into `--description` works for
bodies up to roughly the shell's argv limit (~128 KiB on Linux); for larger
or reusable bodies, write to a file and use `--description "$(cat file.md)"`,
or post via `glab api` with `-F description=@file.md`.

### Threaded replies on merge requests

`glab mr note create` supports `--reply <discussion-id>` for replying inside
an MR thread. The value can be the full discussion ID or a unique prefix of
at least 8 characters — prefer the full ID to avoid prefix collisions.

Diff comments accept a single line (`--line 42`), a range (`--line 10:15`),
a removed line (`--old-line 7`), or no line for a file-level comment.

```shell
glab mr note create  <iid> --reply <discussion-id> -m "I agree!"
glab mr note create  <iid> --file main.go --line 42 -m "Needs refactoring"
glab mr note create  <iid> --file main.go --line 10:15 -m "Extract this block"
glab mr note create  <iid> --file main.go --old-line 7 -m "Why was this removed?"
glab mr note create  <iid> --file main.go -m "General comment on this file"
glab mr note create  <iid> -m "LGTM" --unique   # idempotent: skip if same body exists
glab mr note resolve <iid> <discussion-id>
glab mr note reopen  <iid> <discussion-id>
```

### Threaded replies on issues, incidents, and work items

The CLI does not wrap threaded replies for these. Fall back to `glab api`:

```shell
# Discover the discussion ID
glab api projects/:id/issues/<iid>/discussions \
  | jq '.[] | {id, body: .notes[0].body}'

# Reply to it
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

`glab api` has four ways to send a request body. Pick the one that matches
what the endpoint expects:

```shell
# -f / --raw-field — literal string value
glab api projects/:id/issues/:iid/notes -f body="comment text"

# -F / --field — reads @file as a string; bare values like 'true', '42',
# 'null' are coerced to typed JSON. Use -f if you need a literal string
# that happens to look like a number or boolean.
glab api projects/:id/issues/:iid/notes -F body=@comment.md

# --input — raw request body from a file (or '-' for stdin). Does NOT set
# Content-Type. Without the header, JSON endpoints return HTTP 415.
glab api projects/:id/issues/:iid/notes \
  --input body.json \
  -H "Content-Type: application/json"

# --form — multipart/form-data, required for file uploads such as wiki
# attachments. Prefix the value with @ to upload a file.
glab api projects/:fullpath/wikis/attachments \
  --form "file=@./image.png" \
  --form "branch=main"
```

## Common mistakes

- **`-m` is required on `note` commands** — without it, `glab issue note` and
  `glab incident note` open `$EDITOR` (which hangs in non-interactive
  environments). `glab mr note` and `glab mr note create` fall back to
  reading stdin on a pipe, but still open `$EDITOR` on a TTY.
- **Editor-opening flags are unsafe in agent environments** — avoid
  `--description "-"` on `issue create` / `mr create` / `mr update` and
  avoid omitting `-m` on `note` commands. Pass an explicit value or pipe
  from stdin instead.
- **Don't use `echo` for comment bodies** — `echo` interprets backslashes
  on some shells (dash, `/bin/sh`) and eats leading `-e` / `-n` flags. Use
  `printf '%s' "$body"`, a quoted heredoc, or stdin redirection from a file.
- **`glab issue note` and `glab incident note` only post root-level
  comments** — use `glab mr note create --reply` for MRs, or
  `glab api .../discussions/<id>/notes -f body=...` for issues/incidents.
- **Prefer stdin over inlined bodies** — for anything more than a short
  one-liner, pipe to `glab mr note create` (or `glab api -F body=@file`)
  rather than inlining a heredoc into `"$(...)"`. This avoids argv-length
  limits (~128 KiB on Linux) and shell-quoting traps.
- **`--input` requires an explicit `Content-Type` header** — `glab api
  --input file.json` sends raw bytes without setting Content-Type, causing
  HTTP 415. Add `-H "Content-Type: application/json"` or use `-f` / `-F`
  instead.
- **`-F` coerces bare values to typed JSON** — `-F count=42` sends a JSON
  number, `-F draft=true` sends a boolean, `-F label=null` sends null. If
  you need the literal string `"42"`, `"true"`, or `"null"`, use `-f`.
- **Always `--push` on `glab mr create`** — without it the remote branch
  may not exist and MR creation fails.
- **No `--state` on `mr list`** — use `--all`, `--merged`, or `--closed`.
- **No `--body` flag** — `--body` is a `gh` flag. `glab` uses `--description`.
- **Labels** — `--label` to add, `--unlabel` to remove. Scoped labels like
  `status::doing` auto-replace within their scope.
