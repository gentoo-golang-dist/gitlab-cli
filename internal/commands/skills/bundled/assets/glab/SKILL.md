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
glab mr note create <iid> -m "comment text"

# CI/CD
glab ci status                                   # current branch
glab ci status --output json                     # programmatic
glab ci list                                     # recent pipelines
glab ci get --merge-request <iid> --status=failed --with-job-details
glab ci retry <job-id>                           # non-interactive with ID

# Machine-readable output
glab mr list --output json | jq '.[].title'
```

**Templates:** Check `.gitlab/merge_request_templates/` and
`.gitlab/issue_templates/` for project-specific templates.

**References:** Link issues with `#123`, MRs with `!456`, cross-project
with `group/project#123`.

## Non-interactive use (agents)

Never rely on commands that open `$EDITOR` or stream output — they block
waiting for a TTY. The interactive paths in `glab` are:

- `glab issue note <iid>` / `glab incident note <iid>` without `-m`
- `glab mr note create <iid>` without `-m` **on a TTY** (on a pipe, it
  reads stdin instead — see below)
- `--description "-"` on `glab issue create`, `glab mr create`, `glab mr update`
- `glab ci trace` (streams a job log in real time) — use `glab ci get` instead
- `glab ci view` (terminal UI) — use `glab ci status` or `glab ci get` instead

Always pass `-m`, pipe to stdin where supported, or pass an explicit
`--description` value.

## Comments and discussions

`glab mr note` is the legacy entrypoint and its `--message` / `--unique` /
`--resolve` / `--unresolve` flags are deprecated in favor of the `mr note`
subcommands (`create`, `resolve`, `reopen`). Always use the subcommand form.

### Short, inline bodies — pass `-m`

```shell
glab issue note        <iid> -m "comment text"
glab mr note create    <iid> -m "comment text"
glab incident note     <iid> -m "comment text"

# Cross-project
glab mr note create <iid> -m "..." --repo group/project
```

### Long or Markdown bodies — pipe to stdin (preferred for MR notes)

`glab mr note create` reads the body from stdin when its input is a pipe.
This avoids shell-quoting pitfalls (backticks, `$`, backslashes) and is the
safest pattern for non-interactive use.

```shell
# From a file
glab mr note create <iid> < /tmp/body.md

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
inline a quoted heredoc into `--description`, or for very large or reusable
bodies write to a file and use `--description "$(cat /tmp/desc.md)"`.

### Threaded replies on merge requests

`glab mr note create` supports `--reply <discussion-id>` for replying inside
an MR thread. The value can be the full discussion ID or a unique prefix of
at least 8 characters.

Diff comments accept a single line (`--line 42`), a range (`--line 10:15`),
a removed line (`--old-line 7`), or no line for a file-level comment.

```shell
glab mr note create  <iid> --reply <discussion-id> -m "I agree!"
glab mr note create  <iid> --file main.go --line 42 -m "Needs refactoring"
glab mr note create  <iid> --file main.go --line 10:15 -m "Extract this block"
glab mr note create  <iid> --file main.go --old-line 7 -m "Why was this removed?"
glab mr note create  <iid> --file main.go -m "General comment on this file"
glab mr note create  <iid> -m "LGTM" --unique    # idempotent: skip if same body exists
```

`glab mr note resolve` / `reopen` take the MR identifier followed by the
discussion identifier. The identifier can be a discussion ID (full 40-char
hex or 8+ char prefix) or a note ID (integer; the parent discussion is
looked up automatically):

```shell
glab mr note resolve <iid> <discussion-id>
glab mr note resolve <iid> <note-id>           # integer note ID also works
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

## CI/CD pipeline inspection

For agent use, prefer the non-streaming commands:

```shell
# Current branch status (one-shot, exits immediately)
glab ci status
glab ci status --output json                     # programmatic

# Inspect a specific pipeline
glab ci get --pipeline-id <id> --output json
glab ci get --merge-request <iid> --with-job-details
glab ci get --merge-request <iid> --status=failed --with-job-details

# Retry a failed job by ID (non-interactive when ID is provided)
glab ci retry <job-id>

# Read a finished job's log without streaming
glab api projects/:id/jobs/<job-id>/trace
```

Avoid `glab ci trace` (streams output — doesn't exit until the job
finishes) and `glab ci view` (interactive terminal UI).

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

```shell
# -f / --raw-field — literal string value
glab api projects/:id/issues/:iid/notes -f body="comment text"

# -F / --field — reads @file as a string
glab api projects/:id/issues/:iid/notes -F body=@/tmp/comment.md

# --input — raw request body from a file (or '-' for stdin). Does NOT set
# Content-Type. Without the header, JSON endpoints return HTTP 415.
glab api projects/:id/issues/:iid/notes \
  --input /tmp/body.json \
  -H "Content-Type: application/json"
```

## Common mistakes

- **`-m` is required on `note` commands** — without it, `glab issue note` and
  `glab incident note` open `$EDITOR` (which hangs in non-interactive
  environments). `glab mr note create` falls back to reading stdin on a pipe,
  but still opens `$EDITOR` on a TTY.
- **Use `glab mr note create`, not `glab mr note -m`** — the `--message`,
  `--unique`, `--resolve`, and `--unresolve` flags on the root `glab mr note`
  command are deprecated. Use the `create`, `resolve`, and `reopen`
  subcommands instead.
- **Editor-opening flags are unsafe in agent environments** — avoid
  `--description "-"` on `issue create` / `mr create` / `mr update` and
  avoid omitting `-m` on `note` commands. Pass an explicit value or pipe
  from stdin instead.
- **`glab issue note` and `glab incident note` only post root-level
  comments** — use `glab mr note create --reply` for MRs, or
  `glab api .../discussions/<id>/notes -f body=...` for issues/incidents.
- **`--input` requires an explicit `Content-Type` header** — `glab api
  --input file.json` sends raw bytes without setting Content-Type, causing
  HTTP 415. Add `-H "Content-Type: application/json"` or use `-f` / `-F`
  instead.
- **`glab ci retry` takes a job ID, not a pipeline ID** — to retry an
  entire pipeline, use `glab api projects/:id/pipelines/<id>/retry -X POST`.
- **`glab ci trace` streams** — it blocks until the job finishes. For
  agents, use `glab ci get` for pipeline state or
  `glab api projects/:id/jobs/<job-id>/trace` to fetch a finished log.
- **Always `--push` on `glab mr create`** — without it the remote branch
  may not exist and MR creation fails.
- **No `--state` on `mr list`** — use `--all`, `--merged`, or `--closed`.
- **No `--body` flag** — `--body` is a `gh` flag. `glab` uses `--description`.
- **Labels** — `--label` to add, `--unlabel` to remove. Scoped labels like
  `status::doing` auto-replace within their scope.
