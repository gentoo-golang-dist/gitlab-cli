# Analysis: Merging `note` into `glab mr note`

## Scope

The `note` project (`/Users/tomas/workspace/gl/note`) is a standalone CLI (`notes`) for managing GitLab MR comments. The goal is to merge its features into `glab mr note`.

## Feature Comparison

| Feature | `glab mr note` | `note` |
|---------|:-:|:-:|
| Create general note | ✅ (Notes API — flat note) | ✅ (Discussions API — threaded) |
| Create diff note (line comment) | ❌ | ✅ |
| Create multiline diff comment | ❌ | ✅ |
| Comment on old (removed) line | ❌ | ✅ |
| File-level comment | ❌ | ✅ |
| Reply to discussion | ❌ | ✅ |
| List discussions | ❌ | ✅ (with filter/state/file) |
| Update note | ❌ | ✅ |
| Delete note | ❌ | ✅ |
| Resolve/unresolve | ✅ (by note ID → find discussion) | ✅ (by discussion ID prefix) |
| Draft notes (create/list/update/delete) | ❌ | ✅ |
| Publish drafts (single/all) | ❌ | ✅ |
| Batch review from JSON | ❌ | ✅ |
| Internal/confidential notes | ❌ | ✅ |
| `--unique` dedup | ✅ | ❌ |
| Editor prompt when no message | ✅ (huh form) | ❌ (reads stdin or errors) |
| MR resolution by branch name | ✅ (`MRFromArgs`) | ❌ (by IID only, auto-detect via `glab mr view`) |
| Discussion ID prefix matching | ❌ | ✅ (min 8 chars, exit code 3 on ambiguity) |
| Typed exit codes | ❌ | ✅ (0/1/2/3/4) |
| JSON output | ❌ | ✅ |
| Diff parsing (line mapping) | ❌ | ✅ (`internal/diff`) |

## Incompatibilities

### 1. General note creation: flat note vs. threaded discussion

**This is the most significant incompatibility.**

`glab mr note` creates a **flat note** via `client.Notes.CreateMergeRequestNote()`. Flat notes are standalone, non-threaded, and **not resolvable**.

`note create` (without `--file`) creates a **discussion thread** via `client.Discussions.CreateMergeRequestDiscussion()`. Discussion threads are threaded, resolvable, and can be replied to.

The GitLab web UI creates discussion threads for user-initiated comments, so `note`'s behavior is correct. Changing `glab mr note` to create threads is a **breaking behavior change** — existing users/scripts relying on flat notes (non-resolvable, no thread) would get different semantics.

**Resolution options:**

- **A)** Change default to discussion (matches web UI), add `--no-thread` flag for legacy flat-note behavior. Breaking change but arguably a bug fix.
- **B)** Add `--thread` flag, default off (preserve back-compat). Diff notes and replies always create discussions regardless.
- **C)** Just switch to discussions. Flat notes are rarely intentionally used.

### 2. Resolve/unresolve: note ID vs. discussion ID

`glab mr note --resolve <note-id>` takes a **note ID**, then iterates all discussions to find the parent discussion, then resolves it. This is indirect and requires two API operations (list discussions + resolve).

`notes resolve <discussion-id-prefix>` takes a **discussion ID** (or 8+ char prefix) directly and resolves it. This is more direct and supports prefix matching.

**Incompatibility**: Different identifier types (note ID vs discussion ID). Both are valid use cases — note IDs are visible in `#note_123` URLs, discussion IDs are visible in `notes list` output.

**Resolution**: Support both. Keep `--resolve <note-id>` for backward compat, add `notes resolve <discussion-id>` as a separate subcommand or accept both forms.

### 3. No-message behavior: editor prompt vs. stdin/error

`glab mr note` opens an interactive editor (via `huh` form library) when `-m` is not provided. This is good for interactive use.

`note create` reads from stdin if it's a pipe, or errors if stdin is a TTY and `-m` is not set. This is good for scripting.

**Resolution**: Keep `glab`'s editor behavior (interactive prompt when TTY), add stdin pipe support (read body from stdin when not a TTY). Both behaviors serve different use cases and aren't mutually exclusive.

### 4. MR identification

`glab mr note` accepts `<id>` or `<branch>` as positional arg, resolved via `mrutils.MRFromArgs()`. This is deeply integrated into glab's infrastructure.

`note` uses `--mr <iid>` flag or auto-detects via `glab mr view --output json`. The auto-detection shells out to glab.

**Resolution**: Use glab's native `MRFromArgs()`. The `--mr` flag from `note` becomes unnecessary — glab already handles this with positional args and `-R` repo override.

### 5. Authentication

`note` resolves tokens via `$GITLAB_TOKEN` or `glab auth credential-helper`. This is redundant inside glab — the `Factory` pattern provides `f.GitLabClient()` which handles all auth.

**Resolution**: Use glab's `Factory.GitLabClient()`. No changes needed.

### 6. Output format

`glab mr note` outputs `{web_url}#note_{id}` — a clickable link.

`note create` outputs `Created discussion {id_prefix}` — a short summary.

**Resolution**: Keep glab's URL output format for note creation (more useful). Add `--json` output support across commands.

### 7. Error handling

`note` uses typed exit codes (0=success, 1=API, 2=usage, 3=ambiguous, 4=not found).

`glab` uses cobra's default error handling — errors bubble up and are printed by the root command.

**Resolution**: Map `note`'s error types to appropriate cobra/glab error patterns. Typed exit codes can be preserved by wrapping errors.

### 8. `--unique` flag (glab-only)

`glab mr note --unique` deduplicates by checking existing notes before creating. This feature doesn't exist in `note`.

**Resolution**: Keep it. It's orthogonal to the new features.

### 9. `int64` page type in resolve pagination

`glab mr note` uses `int64` for the page variable when paginating discussions. The `go-gitlab` library uses `int` for `ListOptions.Page`. This is a minor bug in the current code (works on 64-bit systems but technically wrong).

**Resolution**: Fix during merge.

## Proposed Command Structure

```shell
glab mr note [<id>|<branch>] -m "text"              # create general note (→ discussion)
glab mr note [<id>|<branch>] --file X --line N -m "" # create diff note
glab mr note [<id>|<branch>] --reply <id> -m ""      # reply to thread
glab mr note [<id>|<branch>] --resolve <note-id>     # resolve (existing compat)
glab mr note [<id>|<branch>] --unresolve <note-id>   # unresolve (existing compat)

glab mr note list [<id>|<branch>]                     # list discussions (NEW)
glab mr note list --filter diff --state unresolved    # filtered list
glab mr note resolve <discussion-id>                  # resolve by discussion ID (NEW)
glab mr note unresolve <discussion-id>                # unresolve by discussion ID (NEW)
glab mr note update <note-id> -m "new body"           # update note (NEW)
glab mr note delete <note-id>                         # delete note (NEW)

glab mr note draft create -m "text"                   # draft note (NEW)
glab mr note draft list                               # list drafts (NEW)
glab mr note draft publish --all                      # submit review (NEW)

glab mr note review < comments.json                   # batch review (NEW)
```

Alternative: some of these could be separate subcommands under `glab mr` rather than nesting under `note`:

```shell
glab mr discuss list    # instead of glab mr note list
glab mr review          # instead of glab mr note review
```

## Key Modules to Port

### 1. `internal/diff` — diff parser (REQUIRED)

Self-contained package. Parse unified diffs to build line mappings for diff note position construction. No external dependencies beyond stdlib. **Can be ported directly** into glab's internal packages.

### 2. Diff position building logic (REQUIRED)

Scattered across `cmd/create.go`, `cmd/draft.go`, `cmd/review.go`. The `buildDiffPosition()` and `buildPositionForReview()` functions fetch MR diff versions, find file diffs, parse lines, and construct `PositionOptions`. This logic needs consolidation into a shared package.

### 3. Discussion ID prefix resolution (REQUIRED)

`igitlab.ResolveDiscussionID()` — fetches all discussions and prefix-matches. Simple utility, port to glab's API layer or mrutils.

### 4. `igitlab.FindNoteInDiscussions()` — note lookup

Already partially duplicated in glab's resolve logic. Consolidate.

### 5. Review/batch JSON input (NEW)

The `reviewEntry` struct and `runReview()` logic. New capability for glab.

## Dependencies

`note` uses:

- `gitlab.com/gitlab-org/api/client-go` — same as glab ✅
- `github.com/spf13/cobra` — same as glab ✅
- `golang.org/x/term` — for TTY detection (glab has its own `iostreams` package)
- No other external deps

The `note` project shells out to `glab` for project/MR detection and auth. Inside glab, all of this is handled natively by the Factory pattern.

## Migration Risks

1. **Breaking change on general note creation** — switching from flat notes to discussions changes semantics. Needs explicit decision.
2. **Subcommand structure** — current `glab mr note` is a single command. Adding subcommands (`list`, `resolve`, `draft`, `review`) changes the command tree. The bare `glab mr note` invocation must remain backward-compatible.
3. **Test migration** — `note` has no tests (only `diff/parse_test.go`). glab uses `cmdtest` framework with mock clients. All ported features need new tests in glab's style.
4. **The `issuable/note` shared code** — glab shares note creation logic between issues and MRs via `issuable/note`. MR-specific features (diff notes, drafts, resolve) cannot be generalized to issues, so the MR note command will diverge further from the issuable pattern.
