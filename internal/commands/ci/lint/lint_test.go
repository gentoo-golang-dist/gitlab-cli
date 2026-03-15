//go:build !integration

package lint

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func TestCheckPathArg_defaultsToConfiguredPath(t *testing.T) {
	t.Parallel()

	opts := options{defaultPath: ".gitlab-ci.yml"}

	err := opts.checkPathArg(nil)
	require.NoError(t, err)
	require.Equal(t, ".gitlab-ci.yml", opts.path)
}

func TestNewCmdLint_errors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		args             string
		errMsg           string
		showHaveBaseRepo bool
		setupMock        func(tc *gitlabtesting.TestClient)
	}{
		{
			name:             "with invalid path specified",
			args:             "WRONG_PATH",
			errMsg:           "WRONG_PATH: no such file or directory",
			showHaveBaseRepo: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockProjects.EXPECT().
					GetProject("OWNER/REPO", gomock.Any()).
					Return(&gitlab.Project{ID: 123}, nil, nil)
			},
		},
		{
			name:             "without base repo",
			args:             testdataPath(".gitlab-ci.yaml"),
			errMsg:           "you must be in a GitLab project repository for this action.",
			showHaveBaseRepo: false,
			setupMock:        func(tc *gitlabtesting.TestClient) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testClient := gitlabtesting.NewTestClient(t)
			tt.setupMock(testClient)

			opts := []cmdtest.FactoryOption{
				cmdtest.WithGitLabClient(testClient.Client),
			}
			if !tt.showHaveBaseRepo {
				opts = append(opts, cmdtest.WithBaseRepoError(fmt.Errorf("no base repo present")))
			}

			exec := cmdtest.SetupCmdForTest(t, NewCmdLint, false, opts...)

			_, err := exec(tt.args)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errMsg)
		})
	}
}
