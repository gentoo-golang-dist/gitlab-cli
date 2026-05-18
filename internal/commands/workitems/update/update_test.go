//go:build !integration

package update

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/glinstance"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestWorkItemsUpdate(t *testing.T) {
	t.Run("success cases", func(t *testing.T) {
		tests := []struct {
			name       string
			args       string
			workItem   *gitlab.WorkItem
			wantOutput string
		}{
			{
				name: "updates work item title in project scope",
				args: "1 --title \"Test Issue\"",
				workItem: &gitlab.WorkItem{
					IID:    1,
					Title:  "Test Issue",
					WebURL: "https://gitlab.com/OWNER/REPO/-/work_items/1",
				},
				wantOutput: "https://gitlab.com/OWNER/REPO/-/work_items/1",
			},
			{
				name: "updates work item title in group scope",
				args: "2 --group my-group --title \"Test Epic\"",
				workItem: &gitlab.WorkItem{
					IID:    2,
					Title:  "Test Epic",
					WebURL: "https://gitlab.com/groups/my-group/-/work_items/2",
				},
				wantOutput: "https://gitlab.com/groups/my-group/-/work_items/2",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				tc := gitlabtesting.NewTestClient(t)
				tc.MockWorkItems.EXPECT().
					UpdateWorkItem(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(tt.workItem, &gitlab.Response{}, nil)

				exec := cmdtest.SetupCmdForTest(
					t,
					NewCmd,
					false,
					cmdtest.WithGitLabClient(tc.Client),
					cmdtest.WithBaseRepo("OWNER", "REPO", glinstance.DefaultHostname),
				)

				out, err := exec(tt.args)
				require.NoError(t, err)
				assert.Contains(t, out.OutBuf.String(), tt.wantOutput)
			})
		}
	})
}

func TestWorkItemsUpdate_FlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantErr string
	}{
		{
			name:    "invalid <iid> arg",
			args:    "abc",
			wantErr: "invalid work item ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := cmdtest.SetupCmdForTest(
				t,
				NewCmd,
				false,
				cmdtest.WithBaseRepo("OWNER", "REPO", glinstance.DefaultHostname),
			)

			_, err := exec(tt.args)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
