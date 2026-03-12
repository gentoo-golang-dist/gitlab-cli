//go:build !integration

package create

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

func TestWorkItemsCreate(t *testing.T) {
	tests := []struct {
		name       string
		args       string
		setupMock  func(tc *gitlabtesting.TestClient)
		wantErr    bool
		wantOutput string
	}{
		{
			name: "create work item in project",
			args: "--type issue --title \"Test Issue\"",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockWorkItems.EXPECT().CreateWorkItem("OWNER/REPO", gitlab.WorkItemTypeIssue, gomock.Any()).
					Return(&gitlab.WorkItem{
						IID:    1,
						Title:  "Test Issue",
						WebURL: "https://gitlab.com/OWNER/REPO/-/work_items/1",
					}, &gitlab.Response{}, nil)
			},
			wantOutput: "https://gitlab.com/OWNER/REPO/-/work_items/1",
		},
		{
			name: "create work item in group",
			args: "--group my-group --type epic --title \"Test Epic\"",
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockWorkItems.EXPECT().CreateWorkItem("my-group", gitlab.WorkItemTypeEpic, gomock.Any()).
					Return(&gitlab.WorkItem{
						IID:    1,
						Title:  "Test Epic",
						WebURL: "https://gitlab.com/my-group/-/work_items/1",
					}, &gitlab.Response{}, nil)
			},
			wantOutput: "https://gitlab.com/my-group/-/work_items/1",
		},
		{
			name:    "handles API error",
			args:    "--type issue --title \"Test Issue\"",
			wantErr: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockWorkItems.EXPECT().
					CreateWorkItem("OWNER/REPO", gitlab.WorkItemTypeIssue, gomock.Any()).
					Return(nil, nil, assert.AnError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tc := gitlabtesting.NewTestClient(t)
			tt.setupMock(tc)

			exec := cmdtest.SetupCmdForTest(
				t,
				NewCmd,
				false,
				cmdtest.WithGitLabClient(tc.Client),
				cmdtest.WithBaseRepo("OWNER", "REPO", glinstance.DefaultHostname),
			)

			out, err := exec(tt.args)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Contains(t, out.OutBuf.String(), tt.wantOutput)
			}
		})
	}
}

func TestWorkItemsCreate_FlagValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    string
		wantErr string
	}{
		{
			name:    "missing work item type",
			args:    "",
			wantErr: "required flag(s) \"type\" not set",
		},
		{
			name:    "unknown type",
			args:    "--type bogus",
			wantErr: "must be one of",
		},
		{
			name:    "missing --title",
			args:    "--type issue",
			wantErr: "--title required",
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
