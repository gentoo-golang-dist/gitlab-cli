package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestNewCmdUpdate_Flags(t *testing.T) {
	cfg, err := config.Init()
	assert.NoError(t, err)

	ios, _, _, _ := cmdtest.TestIOStreams()
	f := cmdutils.NewFactory(ios, false, cfg, api.BuildInfo{})

	cmd := NewCmdUpdate(f)

	// Test that new flags are available
	linkedIssuesFlag := cmd.Flags().Lookup("linked-issues")
	assert.NotNil(t, linkedIssuesFlag)

	linkTypeFlag := cmd.Flags().Lookup("link-type")
	assert.NotNil(t, linkTypeFlag)

	unlinkIssuesFlag := cmd.Flags().Lookup("unlink-issues")
	assert.NotNil(t, unlinkIssuesFlag)

	// Test default values
	linkType, err := cmd.Flags().GetString("link-type")
	assert.NoError(t, err)
	assert.Equal(t, "relates_to", linkType)

	linkedIssues, err := cmd.Flags().GetIntSlice("linked-issues")
	assert.NoError(t, err)
	assert.Empty(t, linkedIssues)

	unlinkIssues, err := cmd.Flags().GetIntSlice("unlink-issues")
	assert.NoError(t, err)
	assert.Empty(t, unlinkIssues)
}

func TestHandleIssueLinks_ValidLinkType(t *testing.T) {
	cfg, err := config.Init()
	assert.NoError(t, err)

	ios, _, _, _ := cmdtest.TestIOStreams()
	f := cmdutils.NewFactory(ios, false, cfg, api.BuildInfo{})

	cmd := NewCmdUpdate(f)

	// Test valid link types
	validTypes := []string{"relates_to", "blocks", "blocked_by"}
	for _, linkType := range validTypes {
		err := cmd.Flags().Set("link-type", linkType)
		assert.NoError(t, err)

		value, err := cmd.Flags().GetString("link-type")
		assert.NoError(t, err)
		assert.Equal(t, linkType, value)
	}
}
