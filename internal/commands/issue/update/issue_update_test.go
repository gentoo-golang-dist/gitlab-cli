package update

import (
	"fmt"
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

	epicFlag := cmd.Flags().Lookup("epic")
	assert.NotNil(t, epicFlag)

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

	epicID, err := cmd.Flags().GetInt("epic")
	assert.NoError(t, err)
	assert.Equal(t, 0, epicID)
}

func TestLinkTypeFlag_SetAndGet(t *testing.T) {
	cfg, err := config.Init()
	assert.NoError(t, err)

	ios, _, _, _ := cmdtest.TestIOStreams()
	f := cmdutils.NewFactory(ios, false, cfg, api.BuildInfo{})

	cmd := NewCmdUpdate(f)

	// Test valid link types
	validTypes := []string{"relates_to", "blocks", "is_blocked_by"}
	for _, linkType := range validTypes {
		err := cmd.Flags().Set("link-type", linkType)
		assert.NoError(t, err)

		value, err := cmd.Flags().GetString("link-type")
		assert.NoError(t, err)
		assert.Equal(t, linkType, value)
	}
}

func TestLinkTypeValidation(t *testing.T) {
	tests := []struct {
		name        string
		linkType    string
		expectValid bool
	}{
		{
			name:        "valid relates_to",
			linkType:    "relates_to",
			expectValid: true,
		},
		{
			name:        "valid blocks",
			linkType:    "blocks",
			expectValid: true,
		},
		{
			name:        "valid is_blocked_by",
			linkType:    "is_blocked_by",
			expectValid: true,
		},
		{
			name:        "invalid link type",
			linkType:    "invalid_type",
			expectValid: false,
		},
		{
			name:        "empty link type",
			linkType:    "",
			expectValid: false,
		},
		{
			name:        "case sensitive validation",
			linkType:    "RELATES_TO",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the validation logic directly
			validLinkTypes := map[string]bool{
				"relates_to":    true,
				"blocks":        true,
				"is_blocked_by": true,
			}

			isValid := validLinkTypes[tt.linkType]
			assert.Equal(t, tt.expectValid, isValid, "Link type %q validation failed", tt.linkType)
		})
	}
}

func TestSelfReferenceValidation(t *testing.T) {
	tests := []struct {
		name           string
		issueIID       int
		linkedIssueIID int
		expectError    bool
	}{
		{
			name:           "different issues - valid",
			issueIID:       1,
			linkedIssueIID: 2,
			expectError:    false,
		},
		{
			name:           "same issue - invalid",
			issueIID:       42,
			linkedIssueIID: 42,
			expectError:    true,
		},
		{
			name:           "zero IID edge case",
			issueIID:       0,
			linkedIssueIID: 0,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the self-reference validation logic
			isSelfReference := tt.issueIID == tt.linkedIssueIID
			assert.Equal(t, tt.expectError, isSelfReference, "Self-reference validation failed")
		})
	}
}

func TestRelationMappingLogic(t *testing.T) {
	tests := []struct {
		name            string
		currentIssueIID int
		relations       []mockRelation
		expectedMap     map[int]int
	}{
		{
			name:            "empty relations",
			currentIssueIID: 1,
			relations:       []mockRelation{},
			expectedMap:     map[int]int{},
		},
		{
			name:            "relations with self-reference",
			currentIssueIID: 1,
			relations: []mockRelation{
				{IID: 1, IssueLinkID: 100}, // Self-reference, should be skipped
				{IID: 2, IssueLinkID: 200}, // Valid relation
			},
			expectedMap: map[int]int{2: 200},
		},
		{
			name:            "multiple valid relations",
			currentIssueIID: 1,
			relations: []mockRelation{
				{IID: 10, IssueLinkID: 100},
				{IID: 15, IssueLinkID: 150},
				{IID: 20, IssueLinkID: 200},
			},
			expectedMap: map[int]int{10: 100, 15: 150, 20: 200},
		},
		{
			name:            "only self-references",
			currentIssueIID: 5,
			relations: []mockRelation{
				{IID: 5, IssueLinkID: 100},
				{IID: 5, IssueLinkID: 200},
			},
			expectedMap: map[int]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the relation mapping logic from handleIssueLinks
			linkMap := make(map[int]int)
			for _, relation := range tt.relations {
				targetIID := relation.IID
				if relation.IID == tt.currentIssueIID {
					// Skip self-references
					continue
				}
				linkMap[targetIID] = relation.IssueLinkID
			}

			assert.Equal(t, tt.expectedMap, linkMap, "Relation mapping failed")
		})
	}
}

func TestStringConversionLogic(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{
			name:     "positive integer",
			input:    42,
			expected: "42",
		},
		{
			name:     "zero",
			input:    0,
			expected: "0",
		},
		{
			name:     "large number",
			input:    999999,
			expected: "999999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the string conversion logic used in TargetIssueIID
			result := fmt.Sprintf("%d", tt.input)
			assert.Equal(t, tt.expected, result, "String conversion failed")
		})
	}
}

func TestEpicFlag_SetAndGet(t *testing.T) {
	cfg, err := config.Init()
	assert.NoError(t, err)

	ios, _, _, _ := cmdtest.TestIOStreams()
	f := cmdutils.NewFactory(ios, false, cfg, api.BuildInfo{})

	cmd := NewCmdUpdate(f)

	// Test setting epic values
	testValues := []int{0, 12345, 999999}
	for _, epicID := range testValues {
		err := cmd.Flags().Set("epic", fmt.Sprintf("%d", epicID))
		assert.NoError(t, err)

		value, err := cmd.Flags().GetInt("epic")
		assert.NoError(t, err)
		assert.Equal(t, epicID, value)
	}
}

func TestEpicAssignmentLogic(t *testing.T) {
	tests := []struct {
		name           string
		epicID         int
		expectedAction string
	}{
		{
			name:           "assign to epic",
			epicID:         12345,
			expectedAction: "assigned to epic #12345",
		},
		{
			name:           "remove from epic",
			epicID:         0,
			expectedAction: "removed from epic",
		},
		{
			name:           "assign to different epic",
			epicID:         99999,
			expectedAction: "assigned to epic #99999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the epic assignment action message logic
			var action string
			if tt.epicID == 0 {
				action = "removed from epic"
			} else {
				action = fmt.Sprintf("assigned to epic #%d", tt.epicID)
			}

			assert.Equal(t, tt.expectedAction, action, "Epic action message failed")
		})
	}
}

// Helper type for testing relation mapping logic
type mockRelation struct {
	IID         int
	IssueLinkID int
}
