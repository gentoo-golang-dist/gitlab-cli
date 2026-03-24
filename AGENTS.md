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
