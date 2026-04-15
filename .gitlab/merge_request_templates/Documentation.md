## Description

<!--- Describe your changes in detail -->

## Related issues and merge requests

## Documentation checklist

- [ ] The `Short` field is written in the imperative mood and ends with a period.
- [ ] The command includes a `Long` field (Synopsis). It does not repeat the `Short` field.
- [ ] Experimental or beta features use `text.ExperimentalString` or `text.BetaString`.
- [ ] Experimental or beta commands use `text.ExperimentalString` or `text.BetaString` in the
      `Long` field, and append `(EXPERIMENTAL)` to the `Short` field.
- [ ] Experimental flags prepend `(EXPERIMENTAL)` to the flag description and include a note in
      the `Long` field. The `Short` field is not modified for flag-only experimental features.
      for boolean flags.
- [ ] The `Example` field includes at least one non-trivial example. Non-obvious examples
      are annotated with a `# Comment text` line.
- [ ] `make gen-docs` has been run and the updated files in `docs/source/` are committed.

/label ~"devops::create" ~"group::code review" ~"Category:GitLab CLI" ~"cli"
/label ~documentation ~"type::maintenance" ~"docs::improvement" ~"maintenance::refactor"
/assign me
