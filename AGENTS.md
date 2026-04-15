# glab CLI

## Verify Before Pushing

Lefthook runs automatically on `git push` (install once with `lefthook install`).
To run the checks manually:

```bash
lefthook run pre-push                         # all pre-push checks
```

## Running Individual Checks

```bash
make build                                    # compile
make lint                                     # golangci-lint
make fix                                      # auto-fix lint issues (gofmt + goimports)
make test                                     # all unit tests
make test-changed                             # test changed packages + reverse deps vs main
go test ./internal/commands/mr/note/...       # single package
go test ./internal/commands/mr/note/... -run TestCreate  # single test
make gen-docs                                 # regenerate docs from cobra definitions
make generate                                 # go generate (config stubs, etc.)
```

## Documentation conventions

CLI documentation is generated from Go source files by `make gen-docs`. All documentation
content must be authored in the Go source — do not edit files in `docs/source/` directly.

For the full style guide, see
[GitLab CLI (glab) documentation style guide](https://docs.gitlab.com/development/documentation/cli_styleguide/).

When you add or update a command, apply these conventions in the `cobra.Command` definition:

### Short description (`Short` field)

- Write in the imperative mood and end with a period. For example: `Create a new merge request.`
- Do not duplicate the command name.
- For experimental commands, append `(EXPERIMENTAL)`. For example: `Create a new stacked diff. (EXPERIMENTAL)`

### Synopsis (`Long` field)

- All new commands must include a `Long` field.
- Do not repeat information already in the `Short` field.
- Use lists for multiple constraints, input types, or behavioral differences.
- If the command requires a minimum GitLab version, state this. For example:
  `This command requires GitLab 17.0 or later.`
- For experimental or beta commands, use `text.ExperimentalString` or `text.BetaString`
  from `internal/text/text.go`. Do not copy these strings manually.

### Examples (`Example` field)

- Every command must include at least one example that demonstrates real-world usage.
- Do not use examples that only repeat the usage line.
- Annotate non-obvious examples with a `# Comment text` line above the command.
- For commands with many options, show the most common combinations first.

### Flag descriptions

- Start with a capital letter and end with a period.
- Use angle-bracket placeholders for user-supplied values. For example: `<username>`, `<branch>`.
- Always state the default for boolean flags. For example: `(default false)`.
- For experimental flags, prepend `(EXPERIMENTAL)` to the description and add a note in the
  `Long` field explaining the experimental status and any feature flag requirements.
