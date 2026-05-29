# glab CLI agent instructions

Go-based GitLab CLI. Entrypoint: `cmd/glab/main.go`. Commands live under
`internal/commands/<noun>/<verb>/` (noun-first grammar, for example,
`glab mr create`).

## Project structure

- `cmd/glab/` - CLI entrypoint; sets up Cobra root command and theme.
- `cmd/gen-docs/` - Doc generator invoked by `make gen-docs`.
- `internal/commands/` - Command implementations, one package per command.
- `internal/cmdutils/` - Shared command-building helpers. Only
  `internal/commands/**` can import this, enforced by `depguard` in
  `.golangci.yml`.
- `internal/api/`, `internal/auth/`, `internal/config/`, `internal/git/`,
  `internal/glrepo/`, `internal/iostreams/` - Shared infrastructure.
- `docs/source/` - Generated from Go source. Never edit directly.

## Working in this codebase

The most common feedback agents (and humans) receive on this project is
"use the existing helper instead of writing a new one." Before you write
code:

1. **Search for an existing helper** in the directories below before
   introducing a new one. New abstractions need to clear a high bar.
   - `internal/cmdutils/` - flag wiring (`EnableRepoOverride`,
     `EnableJSONOutput`, `NewEnumValue`), the `Factory` interface,
     command helpers.
   - `internal/testing/cmdtest/` - test setup (`SetupCmdForTest`,
     `NewTestFactory`, `FactoryOption`s like `WithStdin`, `WithBranch`,
     `WithGitLabClient`, `WithApiClient`).
   - `internal/iostreams/` - all output (`LogInfo*`, `LogError*`,
     `PrintJSON`) and interactive prompts (`Confirm`, `Input`, `Select`,
     `Hyperlink`).
   - `internal/api/` - GitLab client construction.
   - `internal/glrepo/` - repository interface (`glrepo.Interface`).
   - `internal/tableprinter/` - formatted text output.
   - `internal/text/` - `ExperimentalString`, `BetaString` for marking
     experimental and beta features.
2. **Read the matching block of
   [`.gitlab/duo/mr-review-instructions.yaml`](.gitlab/duo/mr-review-instructions.yaml).**
   It is the canonical source for code conventions and is what GitLab Duo
   Code Review grades MRs against. The blocks are scoped by `fileFilters`,
   so only the ones that match the file you are editing apply.
3. **Look at a canonical example** for the kind of change you are making:
   - New command: `internal/commands/gpg-key/get/get.go`.
   - Paginated list command: `internal/commands/securefile/list/list.go`.
   - Command tests with API mocks:
     `internal/commands/gpg-key/get/get_test.go`.
   - JSON output assertion test:
     `internal/commands/runnercontroller/token/list/list_test.go`.
4. **Audit your diff against
   [`.gitlab/duo/mr-review-instructions.yaml`](.gitlab/duo/mr-review-instructions.yaml)
   before committing.** A grep for the rule keywords against your changes
   catches the easy hits (`fmt.Fprintf`, `httpmock`, missing
   `t.Parallel()`, `_ =`).

## Verify changes before you push

Lefthook runs automatically on `git push`. Install it once with
`lefthook install`. Install all tools with `make bootstrap`, which uses
`mise` and `.tool-versions`.

```shell
lefthook run pre-push  # build, lint against origin/main, test-changed, generated-doc/code check, markdown/vale/lychee
make check             # test + lint
```

To skip hooks, use `LEFTHOOK=0 git push` or
`LEFTHOOK_EXCLUDE=pre-push git push`. Treat these like `--no-verify`, an
escape hatch for debugging the hooks themselves, not a workaround for slow
builds. The hooks catch generated-doc/code drift and lint regressions
before merge.

## Common commands

```shell
make build                                    # compile to ./bin/glab
make lint                                     # golangci-lint (full)
make fix                                      # golangci-lint --fix + gofmt + goimports
make test                                     # all unit tests (gotestsum, writes coverage.txt/xml)
make test-changed                             # tests changed packages + reverse deps against origin/main
make test-race                                # unit tests with -race
go test ./internal/commands/mr/note/...       # single package
go test ./internal/commands/mr/note/... -run TestCreate
make gen-docs                                 # regenerate docs/source/** from cobra definitions
make generate                                 # go generate ./... (includes config stubs)
make gen-config                               # config stubs from internal/config/config.yaml.lock
```

`make test` forcibly clears `VISUAL`, `EDITOR`, `PAGER`, and `GITLAB_TOKEN`,
and sets `CI_PROJECT_PATH` from the origin remote. Some tests depend on
this. If you run `go test` directly and see environment-dependent failures,
replicate that setup.

> [!note]
> Local vendor workflow: `vendor/` is gitignored. Do not request vendor
> updates in merge requests. On an inconsistent-vendoring error,
> run `go mod vendor` to resync. Do not use `-mod=mod`, which bypasses the
> vendor directory instead of fixing it.

## Integration tests

Integration tests are tagged `//go:build integration`, use the file suffix
`_integration_test.go`, and use the test name suffix `_Integration`. They
are not run by `make test`. To run them, use `make integration-test-race`,
which adds `-tags=integration`. They call a real GitLab instance. Locally
they are skipped unless both `GITLAB_TEST_HOST` and `GITLAB_TOKEN_TEST` are
set (see `GetHostOrSkip` in `test/helpers.go`). The token must have the
`api` scope. The
`glab duo` tests require a GitLab Duo-enabled user.

## Documentation is generated

- Never edit files under `docs/source/`. They are regenerated from each
  `cobra.Command`'s `Short`, `Long`, `Example`, and flag description fields
  by `cmd/gen-docs/docs.go` through `make gen-docs`.
- The pre-commit hook regenerates docs when `internal/commands/**` changes,
  and fails if the result differs from what is committed. After you change
  a command, run `make gen-docs` and stage `docs/`.
- The pre-push hook also runs `make generate` and fails on drift.
- Follow the [GitLab CLI (glab) documentation style guide](https://docs.gitlab.com/development/documentation/cli_styleguide/).

## Lint rules to watch for

`.golangci.yml` enforces the following rules:

- Do not send raw JSON to stdout. Use `iostreams.IOStreams.PrintJSON()`
  instead of `json.Marshal` or `json.NewEncoder` for stdout output. For
  non-stdout serialization, add `//nolint:forbidigo` with a reason. See
  the `forbidigo` configuration.
- Imports of `internal/cmdutils` are forbidden outside
  `internal/commands/**`.
- Pre-push runs `golangci-lint run --new-from-rev=origin/main`, which only
  flags new issues compared to `main`.

## Command conventions

[`.gitlab/duo/mr-review-instructions.yaml`](.gitlab/duo/mr-review-instructions.yaml)
is the source that GitLab Duo Code Review enforces. Before you make changes in
`internal/commands/**`, read the matching `fileFilters` section: for example,
`Command structure`, `Command documentation`, `Flag handling`,
`IO streams and output`, `API client and pagination`,
`Test setup with cmdtest and gitlabtesting`, or `Test discipline`.

Highlights:

- Noun-first verbs with shared semantics: `create`, `list`, `get`,
  `update`, and `delete`. See the `Grammar` section in `CONTRIBUTING.md`
  before you introduce a new verb.
- The per-command options struct is unexported and named `options`, not
  `xxxOptions`. The constructor, if present, is `newOptions`, and the
  `NewCmd*` factory takes a `cmdutils.Factory`. Implement only the needed
  subset of `complete`, `validate`, and `run`. Copy the pattern from a
  neighboring command.
- Commit messages use Conventional Commits, enforced by the `commit-msg`
  hook through `scripts/commit-lint`. Requires Node.js.

## Skills

`internal/commands/skills/` has a dedicated pre-commit validator
(`go test ./internal/commands/skills/...`) that catches a missing
`SKILL.md`, bad front matter, an empty `name` or `description`,
asset-directory mismatches, and malformed registry entries. Run it after
you change anything under that tree.

## Environment variables

- `GITLAB_TOKEN` - API token. Overrides configuration.
- `GITLAB_HOST`, `GITLAB_URI`, `GL_HOST` - Default GitLab instance,
  outside Git repositories.
- `GLAB_CONFIG_DIR` - Overrides the configuration directory. Highest priority.
- `GLAB_ENABLE_CI_AUTOLOGIN=true` - Together with `GITLAB_CI=true`,
  enables `CI_JOB_TOKEN` auto-login.
- `DEBUG=true` - Verbose logging for Git commands, expanded aliases, and
  DNS.

## Create merge requests

When a merge request relates to an issue, add `/copy_metadata #<issue-id>`
on its own line in the description. GitLab copies the issue's labels,
milestone, and related metadata to the merge request, so you do not need
`glab mr create --label` flags.

## Review merge request feedback

When asked to look at review feedback on an open MR, use the `glab` CLI
(or the equivalent `glab` MCP tools) end-to-end. Do not reach for raw
`glab api` + ad-hoc JSON parsing for tasks that already have a
higher-level subcommand.

- Fetch the MR including discussions:
  `glab mr view <id> --output json` (or `mcp__glab__glab_mr_view`).
- Read a specific thread or filter for unresolved comments from the JSON
  payload.
- Reply to a thread:
  `glab mr note create --reply <discussion-id> --message "..."`.
- Resolve a thread: `glab mr note resolve <discussion-id>`.
  Pass **only** the discussion ID - a trailing MR ID argument errors out.
- Add a new inline diff comment on the latest version of an MR:
  `glab mr note create --file <path> --line <N>` (or `--line N:M` for a
  range, or `--old-line N` for a deletion). Anchors the note to that diff
  line on the current MR version.

The goal is to keep the agent and human review surfaces consistent: both
go through the same `glab` commands, so the audit trail and notification
behavior stay the same.

## Git and commit hygiene

- Always create a new commit rather than pushing `fixup!` commits to a
  branch under review. Squash locally before pushing if you want a clean
  history.
- After touching any `cobra.Command` definition, run `make gen-docs` and
  commit the regenerated `docs/source/**` in the same commit as the
  source change.
- The current working directory is the repository root. Use bare `git`
  commands - no need for `git -C <path>`.
