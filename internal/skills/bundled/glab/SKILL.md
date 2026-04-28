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
GitLab operations. Do NOT call GitLab APIs directly; use `glab` instead.

Run `glab <command> --help` for detailed flag and usage information.

## Merge requests

```bash
# Create — always use --push so the branch exists on the remote
glab mr create --push --title "feat: add feature" --description "$(cat /tmp/mr-description.md)"

# View and update
glab mr view <iid>
glab mr update <iid> --description "$(cat /tmp/description.md)"
```

**Templates:** Check `.gitlab/merge_request_templates/` for project-specific
templates and follow their structure.

## Issues

```bash
glab issue view <iid>
glab issue list --label "priority::P1,status::doing"
glab issue create --title "Bug: title" --description "$(cat /tmp/issue-description.md)"
```

**Templates:** Check `.gitlab/issue_templates/` for project-specific templates.
**References:** Link related issues with `#123` and MRs with `!456`.

## CI/CD

```bash
glab ci status              # current pipeline status
glab ci list                # recent pipelines
glab ci trace <job-id>      # view job log
glab ci retry <pipeline-id> # retry failed jobs
```

## Machine-readable output

Use `--output json` where available, and pipe to `jq` for processing:

```bash
glab mr list --output json | jq '.[].title'
glab api projects/:id/pipelines | jq '.[0].status'
```

## Gotchas

- **Always `--push` on `glab mr create`** — without it, the remote branch
  may not exist and the MR creation fails.
- **Write long text to a file first** — use `$(cat /tmp/file.md)` for
  descriptions and comments. Do not inline long strings or strings with
  backticks in `-m "..."` or `--description "..."`.
- **No `--body` flag** — glab uses `--description`, not `--body` (that is `gh`).
- **No `--jq` flag** — pipe to `jq` instead: `glab api ... | jq '...'`.
- **No `--state` on `mr list`** — use `--all`, `--merged`, or `--closed`.
- **`<iid>` is the project-scoped ID** — the number shown in the GitLab UI
  (e.g., `#123` for issues, `!456` for MRs).

## Guidelines

1. **Read context first** — `glab issue view` or `glab mr view` before acting.
2. **Use project templates** — check `.gitlab/` for issue and MR templates.
3. **Reference properly** — `Closes #123`, `Related to !456` in commits.
4. **Quote special characters** — use single quotes: `git commit -m 'fix: resolve issue !123'`.
