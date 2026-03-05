# Migration Plan: Merge `note` into `glab mr note`

## Problem

The standalone `note` project has batch reviews — missing from glab.

## Current State

- `glab mr note` creates threaded discussions (via Discussions API), has `--unique`, `--resolve`, `--unresolve`, editor prompt
- `--unique` paginates through all notes (PerPage=100, follows NextPage) instead of checking only the first page
- `note` is a separate binary with full MR comment management
- Diff parser package ported to `internal/diff/` (parse.go, parse_test.go) — all tests passing
- Position-building utilities in `internal/commands/mr/mrutils/position.go`: `GetLatestDiffVersion`, `FindFileDiff`, `BuildDiffPosition`, `ParseLine`, `ResolveDiscussionID`, `ListAllDiscussions`, `FindNoteInDiscussions`, `lineCode` — compiles, all tests pass
- `resolveDiscussion` in `mr_note_create.go` uses `FindNoteInDiscussions` (no more inline pagination/search)
- `--file`, `--line`, `--old-line` flags added to `NewCmdNote()` for diff comments — fetches latest diff version, finds file diff, builds position, creates discussion with position. Mutually exclusive with `--resolve`/`--unresolve`. `--line` and `--old-line` are mutually exclusive. Supports single line, range (N:M), old-side lines, and file-level comments (no line). All tests passing.
- `--reply <discussion-id>` flag added — accepts full 40-char ID or 8+ char prefix, uses `ResolveDiscussionID` then `AddMergeRequestDiscussionNote`. Mutually exclusive with `--file`, `--resolve`, `--unresolve`. All tests passing.
- `--internal` flag added for confidential notes. Uses flat Notes API (`CreateMergeRequestNote` with `Internal: true`) since Discussions API doesn't support internal. Mutually exclusive with `--file`, `--reply`, `--resolve`, `--unresolve`. All tests passing.
- Stdin body support: when `-m` is not provided and stdin is not a TTY, body is read from stdin. `-m` flag takes priority over stdin. Interactive editor prompt only shown for TTY. All tests passing.
- `list` subcommand added (`mr_note_list.go`): `glab mr note list [<id>|<branch>] [--filter all|general|diff|system] [--state all|resolved|unresolved] [--file <path>] [--json]`. Registered as subcommand of `NewCmdNote()`. Filtering by type, resolution state, and file path. Human output with 8-char ID prefix, resolution badge, file position, truncated bodies. JSON output for scripting. 14 tests passing.
- `resolve`/`unresolve` subcommands added (`mr_note_resolve.go`): `glab mr note resolve [<mr-id>|<branch>] <discussion-id>` and `glab mr note unresolve [<mr-id>|<branch>] <discussion-id>`. Accept 8+ char prefix with disambiguation error. Registered as subcommands of `NewCmdNote()`. Existing `--resolve`/`--unresolve` flags preserved as backward-compat aliases (note ID based). 10 tests passing.
- `update` subcommand added (`mr_note_update.go`): `glab mr note update [<mr-id>|<branch>] <note-id> [-m <body>]`. Uses `FindNoteInDiscussions` to locate discussion, then `UpdateMergeRequestDiscussionNote`. Supports `-m` flag and stdin. 8 tests passing.
- `delete` subcommand added (`mr_note_delete.go`): `glab mr note delete [<mr-id>|<branch>] <note-id> [--yes]`. Uses `FindNoteInDiscussions` then `DeleteMergeRequestDiscussionNote`. Confirmation prompt (skip with `--yes`/`-y`), non-TTY without `--yes` errors. 7 tests passing.
- `draft` subcommand group added (`internal/commands/mr/note/draft/`): `create`, `list`, `update`, `delete`, `publish`. Uses DraftNotes API. `create` supports `--file`/`--line`/`--old-line` (diff position), `--reply` (discussion reply), `--resolve` (auto-resolve on publish). `list` supports `--json`. `delete` has `--yes` confirmation. `publish` supports single draft or `--all` (submit review). Registered as subcommand of `NewCmdNote()`. 22 tests passing.
- `review` subcommand added (`mr_note_review.go`): `glab mr note review [<mr-id>|<branch>] [--publish] < comments.json`. Reads JSON array from stdin, batch-creates draft notes. Supports general comments, diff comments (`file`/`line`/`old_line`), replies (`reply`/`resolve`). `--publish` publishes all drafts after creation. Pre-fetches diff version only when needed. 10 tests passing.
- Position utilities (`mrutils/position.go`) fully tested in `position_test.go`: `ParseLine`, `FindFileDiff`, `BuildDiffPosition`, `lineCode`, `ResolveDiscussionID`, `FindNoteInDiscussions` — 26 tests passing.
- Documentation generated via `make gen-docs`: `docs/source/mr/note/_index.md` with all flags/examples/subcommands, plus individual docs for all subcommands (`list`, `delete`, `resolve`, `unresolve`, `update`, `review`, `draft/*`). Stale flat `docs/source/mr/note.md` removed. Changelog is auto-generated from git history (no manual entries needed).

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

All migration steps complete.
