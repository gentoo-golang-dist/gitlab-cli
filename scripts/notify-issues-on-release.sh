#!/bin/sh
set -eu

# This script comments on issues referenced in a release and closes them if needed
# Usage: ./notify-issues-on-release.sh <version-tag>

if [ -z "$1" ]; then
  echo "Usage: $0 <version-tag>"
  echo "Example: $0 v1.89.0"
  exit 1
fi

current_tag="$1"
version="${current_tag#v}" # Remove 'v' prefix if present

# Find the previous release tag (skipping prereleases if current is a full release)
echo "Finding previous release tag..."
start_ref="$current_tag^"
previous_tag=""
is_prerelease() {
  case "$1" in
    *-*) return 0 ;;
    *)   return 1 ;;
  esac
}
while [ -z "$previous_tag" ] || { is_prerelease "$previous_tag" && ! is_prerelease "$current_tag"; }; do
  previous_tag="$(git describe --tags "$start_ref" --abbrev=0 2>/dev/null || echo "")"
  if [ -z "$previous_tag" ]; then
    echo "No previous tag found, using initial commit"
    previous_tag="$(git rev-list --max-parents=0 HEAD)"
    break
  fi
  start_ref="$previous_tag^"
done

echo "Comparing $previous_tag...$current_tag"

# Extract issue numbers from merge commits
# Looking for patterns like "Closes #1234", "Fixes #1234", "Resolves #1234"
issue_numbers=$(git log --merges --pretty=format:"%b" "$previous_tag".."$current_tag" | \
  grep -oiE "(close|closes|closed|fix|fixes|fixed|resolve|resolves|resolved) #[0-9]+" | \
  grep -oE "#[0-9]+" | \
  sed 's/#//' | \
  sort -u)

if [ -z "$issue_numbers" ]; then
  echo "No issues found in commits between $previous_tag and $current_tag"
  exit 0
fi

echo "Found issues: $(echo "$issue_numbers" | tr '\n' ' ')"
echo ""

# Process each issue
for issue in $issue_numbers; do
  echo "Processing issue #$issue..."

  # Check current state
  state=""
  if ! state=$(NO_COLOR=1 glab issue view "$issue" --output json | jq -r '.state // empty' 2>/dev/null); then
    state=""
  fi

  if [ -z "$state" ]; then
    echo "✗ Could not fetch issue #$issue (may be authentication, rate limit, or network issue)"
    echo ""
    continue
  fi

  if [ "$state" = "closed" ]; then
    # Issue already closed - just notify about the release
    echo "✓ Issue #$issue already closed, adding release comment"
    glab issue note "$issue" -m "🎉 This issue has been resolved in [glab v$version](https://gitlab.com/gitlab-org/cli/-/releases/$current_tag)" || \
      echo "✗ Failed to add release comment to issue #$issue"

  elif [ "$state" = "opened" ]; then
    # Issue should have been closed but wasn't - close it now
    echo "⚠️  Issue #$issue was referenced but not auto-closed, closing now"

    # Close the issue with explanation
    glab issue close "$issue" -m "Closing this issue as it was resolved in [glab v$version](https://gitlab.com/gitlab-org/cli/-/releases/$current_tag).

If this issue should remain open or was closed prematurely, please mention a project maintainer and we'll reopen it." || \
      echo "✗ Failed to close issue #$issue"

  else
    echo "✗ Unexpected state for issue #$issue: $state"
  fi

  echo ""
done

echo "Done! Processed $(echo "$issue_numbers" | wc -w) issue(s)"
