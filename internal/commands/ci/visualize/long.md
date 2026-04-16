Generate a visualization of a `.gitlab-ci.yml` pipeline as a DAG.

The diagram shows pipeline jobs grouped by stage, with edges representing
`needs:` dependencies between jobs and implicit stage ordering.

The configuration is always sent to the GitLab API first, which resolves
`include:` directives, `extends:` chains, and `default:` blocks — the resulting
diagram matches exactly what GitLab would run. You must be inside a cloned
GitLab project (or pass `--repo`) so the project can be identified.

By default, all jobs are shown. Use simulation flags like `--branch`, `--tag`,
`--source-branch`, or `--source` to simulate a specific pipeline configuration.
This evaluates `rules:` conditions and filters the diagram to show only jobs
that would actually run. Any CI variable can be set with `--var KEY=VALUE`.

The diagram is rendered as an SVG. By default it is written to a temporary
file and opened in your default browser. Use `--web` to start an interactive
local HTTP server with pan/zoom, or `--output svg` to print the raw SVG to
stdout for scripting.
