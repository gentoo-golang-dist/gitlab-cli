package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/commands/workitems/api"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestDisplayWorkItemList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		workItems     []api.WorkItem
		isTTY         bool
		expectedParts []string
		notExpected   []string
	}{
		{
			name:          "empty list",
			workItems:     []api.WorkItem{},
			isTTY:         false,
			expectedParts: []string{},
		},
		{
			name:  "single open work item",
			isTTY: true,
			workItems: []api.WorkItem{
				{
					IID:   "123",
					Title: "Test Epic",
					State: "OPEN",
					WorkItemType: struct {
						Name string `json:"name"`
					}{Name: "Epic"},
					Author: struct {
						Username string `json:"username"`
					}{Username: "testuser"},
					WebURL: "https://gitlab.com/group/project/-/work_items/123",
				},
			},
			expectedParts: []string{"TYPE", "IID", "Epic", "123", "Test Epic", "OPEN", "testuser"},
		},
		{
			name:  "multiple work items with different states",
			isTTY: false,
			workItems: []api.WorkItem{
				{
					IID:   "1",
					Title: "Open Epic",
					State: "OPEN",
					WorkItemType: struct {
						Name string `json:"name"`
					}{Name: "Epic"},
					Author: struct {
						Username string `json:"username"`
					}{Username: "user1"},
					WebURL: "https://example.com/1",
				},
				{
					IID:   "2",
					Title: "Closed Issue",
					State: "CLOSED",
					WorkItemType: struct {
						Name string `json:"name"`
					}{Name: "Issue"},
					Author: struct {
						Username string `json:"username"`
					}{Username: "user2"},
					WebURL: "https://example.com/2",
				},
			},
			expectedParts: []string{"Open Epic", "Closed Issue", "OPEN", "CLOSED", "Epic", "Issue"},
		},
		{
			name:  "multiple types displayed",
			isTTY: false,
			workItems: []api.WorkItem{
				{
					IID:   "10",
					Title: "Epic Item",
					State: "OPEN",
					WorkItemType: struct {
						Name string `json:"name"`
					}{Name: "Epic"},
					Author: struct {
						Username string `json:"username"`
					}{Username: "author1"},
					WebURL: "https://example.com/10",
				},
				{
					IID:   "20",
					Title: "Task Item",
					State: "OPEN",
					WorkItemType: struct {
						Name string `json:"name"`
					}{Name: "Task"},
					Author: struct {
						Username string `json:"username"`
					}{Username: "author2"},
					WebURL: "https://example.com/20",
				},
			},
			expectedParts: []string{"Epic", "Task", "Epic Item", "Task Item"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ios, _, _, _ := cmdtest.TestIOStreams(cmdtest.WithTestIOStreamsAsTTY(tt.isTTY))

			output := DisplayWorkItemList(ios, tt.workItems)

			if len(tt.expectedParts) == 0 {
				assert.Empty(t, output)
			} else {
				for _, part := range tt.expectedParts {
					assert.Contains(t, output, part)
				}
			}

			for _, part := range tt.notExpected {
				assert.NotContains(t, output, part)
			}
		})
	}
}
