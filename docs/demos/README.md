# Demo recordings

Terminal demos in this repository are generated from [VHS](https://github.com/charmbracelet/vhs)
tape files. A tape is a plain-text script that drives a virtual terminal and
produces a deterministic GIF. Storing them in the repo means recordings are
reviewable, diffable, and reproducible by anyone with VHS installed.

## Install VHS

```shell
# macOS
brew install vhs

# Other platforms: https://github.com/charmbracelet/vhs#installation
```

VHS also requires `ttyd` and `ffmpeg`; Homebrew installs both as dependencies.

## Run an existing tape

Tapes are written to be run from this directory:

```shell
cd docs/demos
vhs getting-started.tape
```

Each tape stages its own working directory inside its hidden setup block so
`glab` picks up a project from a `git` remote regardless of where VHS was
launched from. Output paths in tape files are relative to this directory.

## Shared styling

All tapes `Source base.tape` at the top, which pulls in the canonical
terminal settings — dimensions, theme, typography, frame, window bar. Edit
`base.tape` to change the look across every demo at once. Per-tape files
should only contain:

- The `Source base.tape` line
- `Output` for that tape's GIF
- A `Hide` … `Show` block staging the working directory and prompt
- The script itself

`base.tape` is a partial — it only contains `Set` directives and isn't meant
to be run directly with `vhs`.

## Authentication

Tapes invoke `glab` against the live GitLab API, so the shell VHS spawns
needs a valid token. Pass one in through the environment when invoking
`vhs`:

```shell
GITLAB_TOKEN="<your-personal-access-token>" vhs getting-started.tape
```

`glab` reads `GITLAB_TOKEN` ahead of the keyring and config file, and VHS
inherits its parent's environment, so the token flows through to every
command in the tape. Use `GITLAB_HOST` the same way if you need to target a
non-`gitlab.com` instance.

A few things worth knowing:

- The token only needs `read_api`. If the tape clones over SSH, `git` uses
  your SSH key — the token isn't consulted for that step.
- Don't bake tokens into tape files. Always pass them in via the environment
  at recording time.
- If your interactive shell wraps `glab` in an alias (for example
  `alias glab='op plugin run -- glab'`), VHS's bash session may or may not
  inherit it depending on your `.bashrc`. The bundled tapes defensively
  `unalias glab` in their hidden setup block so they always exercise the
  binary on `PATH`. Do the same in new tapes.

## Add a new tape

1. Create `docs/demos/<name>.tape` and `Source base.tape` as the first
   directive — this inherits the shared look.
2. Mirror the hidden setup pattern from `getting-started.tape`: stage a
   working directory, `unalias glab`, set the orange `❯` `PS1`, `clear`.
3. Prefer flags that bound output (`--per-page`, `--limit`) over relying on
   the user's default config — the recording should look the same on any
   machine.
4. Use `Wait+Screen /regex/` instead of `Sleep` when timing a long-running
   command, so the recording adapts to network latency.
5. Commit both the `.tape` and the generated GIF together.

## Current tapes

| Tape | Output | Used in |
| --- | --- | --- |
| `base.tape` | _(partial — sourced by other tapes)_ | All demos |
| `getting-started.tape` | `../source/img/getting-started.gif` | `README.md`, `docs/source/_index.md` |
