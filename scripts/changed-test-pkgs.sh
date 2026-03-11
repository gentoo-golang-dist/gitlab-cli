#!/usr/bin/env bash
#
# Outputs Go packages that need testing based on changes vs origin/main.
# Includes both directly changed packages and their reverse dependencies
# (packages that import a changed package).
#
# Usage:
#   ./scripts/changed-test-pkgs.sh          # outputs space-separated package list
#   make test-changed                        # runs tests via this script
#
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

MODULE=$(go list -m)

# Get changed .go files: committed branch changes (three-dot merge-base)
# plus any uncommitted changes in the working tree.
COMMITTED=$(git diff --name-only origin/main...HEAD -- '*.go' 2>/dev/null || true)
UNCOMMITTED=$(git diff --name-only HEAD -- '*.go' 2>/dev/null || true)
CHANGED_FILES=$(printf '%s\n%s' "$COMMITTED" "$UNCOMMITTED" | sort -u | grep -v '^$' || true)

if [ -z "$CHANGED_FILES" ]; then
  exit 0
fi

# Convert file paths to package import paths, skipping repo-root files
# (root .go files like main.go belong to the root module package, but
# testing "./" would run everything — handle explicitly).
CHANGED_PKGS=$(echo "$CHANGED_FILES" \
  | xargs -I{} dirname {} \
  | sort -u \
  | grep -v '^\.$' \
  | sed "s|^|${MODULE}/|")

# If there are root-level .go changes, add the module root package
if echo "$CHANGED_FILES" | xargs -I{} dirname {} | grep -q '^\.$'; then
  CHANGED_PKGS=$(printf '%s\n%s' "$MODULE" "$CHANGED_PKGS")
fi

if [ -z "$CHANGED_PKGS" ]; then
  exit 0
fi

# Find reverse dependencies: packages in this module that import any changed package
REVERSE_DEPS=$(go list -f '{{.ImportPath}} {{join .Deps " "}}' ./... 2>/dev/null \
  | while IFS= read -r line; do
      pkg="${line%% *}"
      deps=" ${line#* } "
      for cp in $CHANGED_PKGS; do
        if echo "$deps" | grep -q " $cp "; then
          echo "$pkg"
          break
        fi
      done
    done)

# Combine changed packages + reverse deps, deduplicate
printf '%s\n%s' "$CHANGED_PKGS" "$REVERSE_DEPS" \
  | sort -u \
  | grep -v '^$' \
  | tr '\n' ' '
