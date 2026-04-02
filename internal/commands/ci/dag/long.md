Generate a DAG (Directed Acyclic Graph) visualization of a `.gitlab-ci.yml` pipeline.

The diagram shows pipeline jobs grouped by stage, with edges representing
`needs:` dependencies between jobs and implicit stage ordering.

By default, the local `.gitlab-ci.yml` file is parsed directly. Use `--compiled`
to send the file to the GitLab API first, which resolves all `include:` directives
and returns the fully expanded configuration.

The diagram is rendered as an SVG and opened in your default browser via a local
HTTP server. Use `--output svg` to print the raw SVG to stdout instead.