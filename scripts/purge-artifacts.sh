#!/usr/bin/env bash
set -euo pipefail

# Script to bulk delete job artifacts from the project
# This uses the GitLab bulk delete API to set expiry to "now" for eligible artifacts
# See: https://docs.gitlab.com/ee/api/job_artifacts.html#delete-project-artifacts

PROJECT_PATH="gitlab-org/cli"
DRY_RUN=false
GITLAB_TOKEN=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --token)
            GITLAB_TOKEN="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--dry-run] [--token TOKEN]"
            exit 1
            ;;
    esac
done

echo "=========================================="
echo "GitLab Artifact Cleanup Script"
echo "=========================================="
echo ""

# Check if glab is available
if ! command -v glab &> /dev/null; then
    echo "❌ Error: 'glab' command not found"
    echo "   Please ensure glab is installed and in your PATH"
    exit 1
fi

# Check if jq is available
if ! command -v jq &> /dev/null; then
    echo "❌ Error: 'jq' command not found"
    echo "   Please install jq for JSON processing"
    exit 1
fi

# Check authentication
if [[ -z "$GITLAB_TOKEN" ]]; then
    if ! glab auth status &> /dev/null; then
        echo "❌ Error: Not authenticated with GitLab"
        echo "   Either run 'glab auth login' or use --token flag"
        echo "   Usage: $0 [--token YOUR_TOKEN]"
        exit 1
    fi
fi

# Set up glab command with optional token
if [[ -n "$GITLAB_TOKEN" ]]; then
    GLAB_CMD="env GITLAB_TOKEN=${GITLAB_TOKEN} glab"
else
    GLAB_CMD="glab"
fi

# URL encode the project path
PROJECT_ENCODED=$(printf '%s' "${PROJECT_PATH}" | jq -sRr @uri)

echo "📊 Analyzing artifacts for project: ${PROJECT_PATH}"
echo ""

# Fetch jobs with artifacts (limited sample)
echo "Fetching recent jobs with artifacts..."
JOBS=$($GLAB_CMD api "projects/${PROJECT_ENCODED}/jobs?per_page=100&with_artifacts=true" 2>/dev/null || echo "[]")

if [[ "$JOBS" == "[]" ]]; then
    echo "ℹ️  No recent jobs with artifacts found (checked last 100 jobs)"
    echo ""
fi

# Analyze artifacts
TOTAL_JOBS=$(echo "$JOBS" | jq 'length')
NO_EXPIRY=$(echo "$JOBS" | jq '[.[] | select(.artifacts_expire_at == null)] | length')
WITH_EXPIRY=$(echo "$JOBS" | jq '[.[] | select(.artifacts_expire_at != null)] | length')

echo "Recent artifacts (last 100 jobs with artifacts):"
echo "  - Total jobs with artifacts: ${TOTAL_JOBS}"
echo "  - Jobs with NO expiration set: ${NO_EXPIRY}"
echo "  - Jobs with expiration already set: ${WITH_EXPIRY}"
echo ""

if [[ $NO_EXPIRY -gt 0 ]]; then
    echo "Sample jobs with NO expiration:"
    echo "$JOBS" | jq -r '[.[] | select(.artifacts_expire_at == null)] | .[:5] | .[] | "  - \(.name) (pipeline #\(.pipeline.iid), \(.created_at[:10]))"'
    echo ""
fi

echo "ℹ️  About the bulk delete API:"
echo ""
echo "What WILL be deleted:"
echo "  - Artifacts from older pipelines (not the most recent successful per ref)"
echo "  - This includes artifacts that already have expire_in set"
echo "  - Artifacts are set to expire immediately, then deleted asynchronously"
echo ""
echo "What will NOT be deleted:"
echo "  - Artifacts from the most recent successful pipeline of each branch/tag"
echo "  - Job logs (never deleted)"
echo "  - Release assets (stored separately)"
echo ""
echo "⚠️  WARNING: This action cannot be undone!"
echo ""

if [[ "$DRY_RUN" == "true" ]]; then
    echo "🔍 DRY RUN MODE - No changes will be made"
    echo ""
    echo "To actually delete artifacts, run without --dry-run flag"
    exit 0
fi

# Prompt for confirmation
read -p "Do you want to proceed with deletion? (yes/no): " -r
echo
if [[ ! $REPLY =~ ^[Yy][Ee][Ss]$ ]]; then
    echo "Aborted."
    exit 0
fi

echo "🗑️  Deleting artifacts for project: ${PROJECT_PATH}"
echo ""

# Call the bulk delete API
RESPONSE=$($GLAB_CMD api -X DELETE "projects/${PROJECT_ENCODED}/artifacts" 2>&1) || {
    EXIT_CODE=$?
    echo "❌ Error: Failed to delete artifacts"
    echo "${RESPONSE}"
    exit ${EXIT_CODE}
}

echo "✅ Success! Artifact deletion has been queued."
echo ""
echo "📝 Note: Artifacts are deleted asynchronously in the background."
echo "   It may take a few minutes for storage usage to reflect the changes."
echo ""
echo "💡 Next steps:"
echo "   1. Merge the artifact expiration PR to prevent future accumulation"
echo "   2. Disable 'Keep latest artifacts' setting if not already done"
echo "   3. Monitor storage usage at:"
echo "      https://gitlab.com/${PROJECT_PATH}/-/usage_quotas"
