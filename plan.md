# Migration Plan: Merge `note` into `glab mr note`

## Problem

`glab mr note` only supports creating flat (non-threaded) notes and resolve/unresolve by note ID. The standalone `note` project has diff comments, replies, drafts, batch reviews, discussion listing, update/delete, and proper diff line mapping — all missing from glab.

## Current State

- `glab mr note` creates threaded discussions (via Discussions API), has `--unique`, `--resolve`, `--unresolve`, editor prompt
- `--unique` paginates through all notes (PerPage=100, follows NextPage) instead of checking only the first page
- `note` is a separate binary with full MR comment management
- Diff parser package ported to `internal/diff/` (parse.go, parse_test.go) — all tests passing
- Position-building utilities in `internal/commands/mr/mrutils/position.go`: `GetLatestDiffVersion`, `FindFileDiff`, `BuildDiffPosition`, `ParseLine`, `ResolveDiscussionID`, `ListAllDiscussions`, `FindNoteInDiscussions`, `lineCode` — compiles, all tests pass
- `resolveDiscussion` in `mr_note_create.go` uses `FindNoteInDiscussions` (no more inline pagination/search)
- `--file`, `--line`, `--old-line` flags added to `NewCmdNote()` for diff comments — fetches latest diff version, finds file diff, builds position, creates discussion with position. Mutually exclusive with `--resolve`/`--unresolve`. `--line` and `--old-line` are mutually exclusive. Supports single line, range (N:M), old-side lines, and file-level comments (no line). All tests passing.
- `--reply <discussion-id>` flag added — accepts full 40-char ID or 8+ char prefix, uses `ResolveDiscussionID` then `AddMergeRequestDiscussionNote`. Mutually exclusive with `--file`, `--resolve`, `--unresolve`. All tests passing.

## User-Facing Changes

None of these warrant a major version bump, but stay on the lookout for further breaking changes during implementation.

### 1. General notes now create threaded discussions

`glab mr note -m "text"` currently creates a flat, non-resolvable note via the Notes API. After migration, it creates a **discussion thread** via the Discussions API.

- Notes become **resolvable** (they weren't before)
- Notes become **threaded** (replies create a thread instead of triggering a type conversion)
- This matches GitLab web UI behavior

Unlikely to break anyone — scripts would have to be filtering by `resolvable: false` to notice.

### 2. `--unique` checks all notes (not just first page)

Currently `--unique` only checks the first 30 notes (`DefaultListLimit`). After migration, it paginates through all notes. This is a bug fix — dedup was silently broken on MRs with 30+ notes.

### 3. `--resolve`/`--unresolve` accept discussion ID in addition to note ID

The existing `--resolve <note-id>` behavior is preserved. New `resolve`/`unresolve` subcommands accept discussion IDs (with 8+ char prefix matching). Additive only.

---

## Migration Steps

### Step 1: Add `--internal` flag

Add `--internal` flag for confidential notes. Only valid for general notes (no `--file`). Uses `Internal: gl.Ptr(true)` on the Notes API (the Discussions API doesn't support internal, so internal general notes use the flat Notes API — this is a GitLab API limitation).

Mark mutually exclusive with `--file`.

### Step 2: Add stdin body support

When `-m` is not provided and stdin is not a TTY, read body from stdin instead of opening the editor prompt. Keep the editor prompt for interactive TTY use.

```go
if strings.TrimSpace(body) == "" {
    if !f.IO().IsStdinTTY() {
        // read from stdin
        data, err := io.ReadAll(f.IO().In)
        body = strings.TrimSpace(string(data))
    } else {
        // existing editor prompt
    }
}
```

### Step 3: Add `list` subcommand

New file: `internal/commands/mr/note/mr_note_list.go`

```
glab mr note list [<id>|<branch>] [--filter all|general|diff|system] [--state all|resolved|unresolved] [--file <path>] [--json]
```

Port filtering logic from `note/cmd/list.go`. Use glab's `tableprinter` or similar for human output, `print_json` for `--json`.

Register as subcommand of `NewCmdNote()`.

### Step 4: Add `resolve`/`unresolve` subcommands

New file: `internal/commands/mr/note/mr_note_resolve.go`

```
glab mr note resolve [<mr-id>|<branch>] <discussion-id>
glab mr note unresolve [<mr-id>|<branch>] <discussion-id>
```

Accepts 8+ char prefix with disambiguation error (exit code 3). Keep existing `--resolve`/`--unresolve` flags as aliases for backward compat (these take note IDs, not discussion IDs).

### Step 5: Add `update` subcommand

New file: `internal/commands/mr/note/mr_note_update.go`

```
glab mr note update [<mr-id>|<branch>] <note-id> [-m <body>]
```

Uses `FindNoteInDiscussions` from `mrutils/position.go` to locate the discussion, then `Discussions.UpdateMergeRequestDiscussionNote()`. Supports `-m` and stdin.

### Step 6: Add `delete` subcommand

New file: `internal/commands/mr/note/mr_note_delete.go`

```
glab mr note delete [<mr-id>|<branch>] <note-id> [--yes]
```

Confirmation prompt (skip with `--yes`), uses `FindNoteInDiscussions` then `Discussions.DeleteMergeRequestDiscussionNote()`.

### Step 7: Add `draft` subcommand group

New directory: `internal/commands/mr/note/draft/`

```
glab mr note draft create [<mr-id>|<branch>] [-m <body>] [--file ...] [--reply ...] [--resolve]
glab mr note draft list [<mr-id>|<branch>] [--json]
glab mr note draft update [<mr-id>|<branch>] <draft-id> [-m <body>]
glab mr note draft delete [<mr-id>|<branch>] <draft-id>
glab mr note draft publish [<mr-id>|<branch>] <draft-id>
glab mr note draft publish [<mr-id>|<branch>] --all
```

Port from `note/cmd/draft.go`. Uses same position-building utilities. Adapt to glab Factory pattern.

### Step 8: Add `review` subcommand

New file: `internal/commands/mr/note/mr_note_review.go`

```
glab mr note review [<mr-id>|<branch>] [--publish] < comments.json
```

Port from `note/cmd/review.go`. Reads JSON array from stdin, creates draft notes, optionally bulk-publishes. Key command for AI agent/editor integration.

### Step 9: Tests for all new commands

Each new file needs tests using glab's `cmdtest` framework with `gitlabtesting.NewTestClient(t)` mock pattern. Port and adapt the diff parser tests directly. All other tests are new (the `note` project has no command-level tests beyond the diff parser).

### Step 10: Documentation

- Update `docs/source/mr/note.md` (auto-generated from command definitions)
- Run `make gen-docs`
- Add changelog entries noting the flat→thread and `--unique` pagination behavior changes
