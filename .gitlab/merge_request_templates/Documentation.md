## Description

<!--- Describe your changes in detail -->

## Related issues and merge requests

## Documentation checklist

- [ ] The `Short` field is written in the imperative mood and ends with a period.
- [ ] The command includes a `Long` field (Synopsis). It does not repeat the `Short` field.
- [ ] Experimental or beta features use `text.ExperimentalString` or `text.BetaString`.
      `(EXPERIMENTAL)` appears in both the `Short` field and any affected flag descriptions.
- [ ] Flag descriptions start with a capital letter, end with a period, and state defaults
      for boolean flags.
- [ ] The `Example` field includes at least one non-trivial example. Non-obvious examples
      are annotated with a `# Comment text` line.
- [ ] `make gen-docs` has been run and the updated files in `docs/source/` are committed.

/label ~"devops::create" ~"group::code review" ~"Category:GitLab CLI" ~"cli"
/label ~documentation ~"type::maintenance" ~"docs::improvement" ~"maintenance::refactor"
/assign me
