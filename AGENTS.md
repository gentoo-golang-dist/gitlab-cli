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
content must be authored in the Go source. Do not edit files in `docs/source/` directly.

When you add or update a command, follow the conventions in the
[GitLab CLI (glab) documentation style guide](https://docs.gitlab.com/development/documentation/cli_styleguide/).
