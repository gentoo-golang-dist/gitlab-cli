# Produces getting-started.gif — the introductory glab demo embedded in
# README.md and docs/source/_index.md.
#
# Run from this directory:
#   cd docs/demos && GITLAB_TOKEN=glpat-… vhs getting-started.tape

Source base.tape

Output ../source/img/getting-started.gif

# Stage the recording shell in a fresh temp dir with a clean prompt so the
# clone lands somewhere disposable and the recording starts on a blank line.
# Clear any inherited `glab` alias so the tape exercises the binary on PATH.
Hide
Type `cd "$(mktemp -d)"`
Enter
Type `unalias glab 2>/dev/null || true`
Enter
Type `export PS1="\[\033[38;2;252;109;38m\]❯\[\033[0m\] "`
Enter
Type "clear"
Enter
Show

Sleep 1s

Type "glab repo clone gitlab-org/cli"
Enter
Sleep 7s

Type "cd cli"
Enter
Sleep 500ms

Type "glab issue list --per-page 8"
Enter
Sleep 3s

Type "glab mr list --per-page 8"
Enter
Sleep 3s

Sleep 2s
