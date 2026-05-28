# glab CLI - Agent Instructions

## Quick Start

**Build:** `make build` (or `go build -o bin/glab ./cmd/glab`)  
**Test:** `make test` (or `go test ./...`)  
**Lint:** `make lint` (or `golangci-lint run`)  
**Fix lint:** `make fix` (auto-fixes gofmt, goimports, golangci-lint)

## Project Structure

- **`cmd/glab/main.go`** - CLI entrypoint; sets up Cobra root command and theme
- **`internal/commands/`** - Command implementations organized by feature (mr, issue, ci, etc.)
- **`internal/cmdutils/`** - Shared utilities for command building (only imported by `internal/commands/`)
- **`internal/api/`** - GitLab API client wrappers
- **`internal/config/`** - Configuration file handling (XDG Base Directory spec)
- **`internal/auth/`** - Authentication (OAuth, tokens, CI job tokens)
- **`internal/git/`** - Git operations
- **`internal/glrepo/`** - Repository metadata (project path, host detection)
- **`internal/iostreams/`** - Output formatting (JSON, tables, markdown)
- **`docs/source/`** - **Auto-generated from Go source** — do not edit directly

## Critical Workflows

### Documentation

**All CLI docs are auto-generated from Go source.** Never edit `docs/source/` directly.

- Update `cobra.Command` fields: `Short`, `Long`, `Example`, flag descriptions
- Regenerate: `make gen-docs`
- Pre-commit hook validates docs are in sync; blocks commit if out of date
- Style guide: [GitLab CLI documentation style guide](https://docs.gitlab.com/development/documentation/cli_styleguide/)

### Testing

**Unit tests:** `make test` or `go test ./...`  
**Single package:** `go test ./internal/commands/mr/note/...`  
**Single test:** `go test ./internal/commands/mr/note/... -run TestCreate`  
**Changed packages only:** `make test-changed` (tests changed packages + reverse deps vs `origin/main`)  
**Race detection:** `make test-race` (enabled in CI)

**Integration tests** (real API calls to gitlab.com):
- Require `GITLAB_TEST_HOST` and `GITLAB_TOKEN_TEST` environment variables
- Skipped locally if env vars not set; fail in CI if not set
- Use `_integration_test.go` suffix and `_Integration` test suffix
- Token must have `api` scope; user must have GitLab Duo seat for `glab duo` tests

### Code Generation

- **Config stubs:** `make gen-config` (from `internal/config/config.yaml.lock`)
- **Docs:** `make gen-docs` (from Cobra command definitions)
- **Go generate:** `make generate` (runs all `//go:generate` directives)
- Pre-push hook validates generated code is in sync

### Linting & Formatting

**Linter config:** `.golangci.yml` (strict rules; see `forbidigo` for JSON marshaling restrictions)

Key rules:
- Use `IOStreams.PrintJSON()` for stdout JSON output (not `json.Marshal`)
- No `internal/cmdutils` imports outside `internal/commands/`
- No test utilities in production code

**Auto-fix:** `make fix` (gofmt, goimports, golangci-lint --fix)

### Git Hooks (Lefthook)

Install once: `make bootstrap` then `lefthook install`

**Pre-commit:**
- Go formatting & linting (auto-fix)
- Shell script linting
- Markdown auto-fix
- Skills validation
- Docs regeneration check (blocks if out of sync)

**Commit-msg:** Validates conventional commits format

**Pre-push:**
- Build check
- Lint (only changes vs `origin/main`)
- Unit tests on changed packages
- Generated code sync check
- Markdown & prose linting
- Link checking

Skip hooks: `LEFTHOOK=0 git commit/push`

## Command Structure Conventions

Commands follow noun-first, verb-second pattern: `glab <noun> <verb>` (e.g., `glab mr create`)

Standard verbs:
- `create` - singular object
- `list` - multiple objects
- `get` - single object by ID
- `update` - modify object
- `delete` - remove object(s)

Command options struct pattern:
```go
type options struct {
    // private fields
}

func newOptions() *options { ... }
func (o *options) validate() error { ... }
func (o *options) run(ctx context.Context) error { ... }
```

## Configuration & Environment

**Config locations** (XDG Base Directory spec):
- `~/.config/glab-cli/config.yml` (legacy, checked first)
- `$XDG_CONFIG_HOME/glab-cli/config.yml` (platform-specific)
- `$XDG_CONFIG_DIRS/glab-cli/config.yml` (system-wide)
- `.git/glab-cli/config.yml` (per-repo)

**Key env vars:**
- `GITLAB_TOKEN` - API token (overrides config)
- `GITLAB_HOST` - Default GitLab instance (outside git repos)
- `GLAB_CONFIG_DIR` - Override config directory
- `DEBUG=true` - Verbose logging (Git commands, expanded aliases, DNS details)
- `GLAB_ENABLE_CI_AUTOLOGIN=true` - Auto-login in CI jobs

## Commit Message Format

Conventional commits required (enforced by pre-commit hook):

```
<type>(<scope>): <description>

<body>

<footer>
```

Types: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`

Example: `feat(mr): add approval count to list output`

## Release & Versioning

- Follows SemVer
- **MAJOR:** Breaking changes (delete command, required flag, behavior change)
- **MINOR:** New command or optional flag
- **PATCH:** Bug fix

Release process: Tag commit, CI builds and publishes via goreleaser

## Verify Before Pushing

```bash
lefthook run pre-push  # all checks
make check             # tests + linting
```

## Common Gotchas

1. **Docs out of sync:** Pre-commit hook blocks; run `make gen-docs` and stage changes
2. **JSON output:** Use `IOStreams.PrintJSON()`, not `json.Marshal`
3. **Config file permissions:** `internal/config/config.yaml.lock` needs `chmod 600`
4. **Integration tests:** Require env vars; skipped locally if not set
5. **Reverse dependencies:** `make test-changed` tests affected packages, not just changed ones
6. **Skills validation:** `internal/commands/skills/` has special pre-commit validation
