Generate a visualization of a `.gitlab-ci.yml` pipeline as a DAG.

The diagram shows pipeline jobs grouped by stage, with edges representing
`needs:` dependencies between jobs and implicit stage ordering.

The configuration is sent to the GitLab API first, which resolves
`include:` directives, `extends:` chains, and `default:` blocks — the resulting
diagram matches what GitLab would run. You must be inside a cloned
GitLab project (or pass `--repo`) so the project can be identified.

The diagram is rendered as an SVG. By default it is written to a temporary
file and opened in your default browser. Use `--web` to start an interactive
local HTTP server with pan/zoom, or `--output svg` to print the raw SVG to
stdout for scripting.
