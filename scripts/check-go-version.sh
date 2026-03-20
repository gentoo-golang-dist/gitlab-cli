#!/usr/bin/env bash
#
# Checks that the Go version is consistent across go.mod, .tool-versions,
# and .gitlab-ci.yml.

set -euo pipefail

errors=0

go_mod_version=$(awk '/^go / { print $2 }' go.mod)
tool_versions_version=$(awk '/^golang / { print $2 }' .tool-versions)
ci_version=$(awk -F'"' '/GO_VERSION:/ { print $2; exit }' .gitlab-ci.yml)

echo "Go versions found:"
echo "  go.mod:          ${go_mod_version:-<missing>}"
echo "  .tool-versions:  ${tool_versions_version:-<missing>}"
echo "  .gitlab-ci.yml:  ${ci_version:-<missing>}"
echo

if [ -z "$go_mod_version" ]; then
  echo "✖ ERROR: Could not parse Go version from go.mod"
  errors=1
fi

if [ -z "$tool_versions_version" ]; then
  echo "✖ ERROR: Could not parse golang version from .tool-versions"
  errors=1
fi

if [ -z "$ci_version" ]; then
  echo "✖ ERROR: Could not parse GO_VERSION from .gitlab-ci.yml"
  errors=1
fi

if [ "$errors" -eq 1 ]; then
  exit 1
fi

if [ "$go_mod_version" != "$tool_versions_version" ]; then
  echo "✖ ERROR: .tool-versions (${tool_versions_version}) does not match go.mod (${go_mod_version})"
  errors=1
fi

if [ "$go_mod_version" != "$ci_version" ]; then
  echo "✖ ERROR: .gitlab-ci.yml GO_VERSION (${ci_version}) does not match go.mod (${go_mod_version})"
  errors=1
fi

if [ "$errors" -eq 0 ]; then
  echo "✔ All Go versions are consistent."
else
  echo
  echo "Please update all files to use the same Go version."
  exit 1
fi
