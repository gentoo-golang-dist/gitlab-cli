//go:build !integration

package get

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/v2/testing"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/glinstance"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func withApiClientErrorOption(err error) cmdtest.FactoryOption {
	return func(f *cmdtest.Factory) {
		f.ApiClientStub = func(string) (*api.Client, error) {
			return nil, err
		}
	}
}

func TestRepoFileGet_HelpExposesRepoOverride(t *testing.T) {
	f := cmdtest.NewTestFactory(nil)
	cmd := NewCmdFileGet(f)

	flag := cmd.PersistentFlags().Lookup("repo")
	require.NotNil(t, flag, "repo flag must be reachable from `glab repo file get --help`")
	assert.False(t, flag.Hidden, "repo flag must not be hidden")
	assert.Equal(t, "R", flag.Shorthand, "repo flag must keep -R short form")

	assert.NotNil(t, cmd.Flags().Lookup("output"))
	assert.NotNil(t, cmd.Flags().Lookup("ref"))
	assert.NotNil(t, cmd.Flags().Lookup("lfs"))
}

func TestRepoFileGet(t *testing.T) {
	tests := []struct {
		name             string
		cli              string
		setupMocks       func(t *testing.T, tc *gitlabtesting.TestClient)
		apiClientInitErr error
		wantErr          string
		wantStdout       string
	}{
		{
			name: "text output happy path",
			cli:  "README.md --ref deadbeef",
			setupMocks: func(t *testing.T, tc *gitlabtesting.TestClient) {
				t.Helper()
				tc.MockRepositoryFiles.EXPECT().
					GetRawFile("OWNER/REPO", "README.md", gomock.Cond(func(o *gitlab.GetRawFileOptions) bool {
						return o != nil && o.Ref != nil && *o.Ref == "deadbeef" && o.LFS == nil
					})).
					Return([]byte("hello world\n"), nil, nil)
			},
			wantStdout: "hello world\n",
		},
		{
			name: "json output happy path",
			cli:  "docs/index.md --ref deadbeef --output json",
			setupMocks: func(t *testing.T, tc *gitlabtesting.TestClient) {
				t.Helper()
				tc.MockRepositoryFiles.EXPECT().
					GetFile("OWNER/REPO", "docs/index.md", gomock.Cond(func(o *gitlab.GetFileOptions) bool {
						return o != nil && o.Ref != nil && *o.Ref == "deadbeef"
					})).
					Return(&gitlab.File{
						FileName:     "index.md",
						FilePath:     "docs/index.md",
						Size:         12,
						Encoding:     "base64",
						Content:      "aGVsbG8gd29ybGQK",
						Ref:          "deadbeef",
						BlobID:       "abc123",
						CommitID:     "deadbeef",
						LastCommitID: "deadbeef",
						SHA256:       "deadbeefdeadbeef",
					}, nil, nil)
			},
			// Exact-match: guards against accidental MarshalIndent / extra newline /
			// dropped or renamed fields.
			wantStdout: `{"file_name":"index.md","file_path":"docs/index.md","size":12,"encoding":"base64","content":"aGVsbG8gd29ybGQK","execute_filemode":false,"ref":"deadbeef","blob_id":"abc123","commit_id":"deadbeef","content_sha256":"deadbeefdeadbeef","last_commit_id":"deadbeef"}` + "\n",
		},
		{
			name:    "missing --ref errors before any API call",
			cli:     "README.md",
			wantErr: `required flag(s) "ref" not set`,
		},
		{
			name:    "empty --ref is rejected client-side",
			cli:     `README.md --ref ""`,
			wantErr: `flag "--ref" cannot be empty`,
		},
		{
			name:    "empty path is rejected client-side",
			cli:     `"" --ref deadbeef`,
			wantErr: "path argument cannot be empty",
		},
		{
			name:    "whitespace-only path is rejected client-side",
			cli:     `"   " --ref deadbeef`,
			wantErr: "path argument cannot be empty",
		},
		{
			name:    "--lfs with --output json is rejected client-side",
			cli:     "README.md --ref deadbeef --output json --lfs",
			wantErr: "--lfs cannot be used with --output json",
		},
		{
			name: "404 surfaces both prefix and inner error",
			cli:  "missing.md --ref deadbeef",
			setupMocks: func(t *testing.T, tc *gitlabtesting.TestClient) {
				t.Helper()
				tc.MockRepositoryFiles.EXPECT().
					GetRawFile("OWNER/REPO", "missing.md", gomock.Any()).
					Return(nil, &gitlab.Response{Response: &http.Response{StatusCode: http.StatusNotFound}}, errors.New("404 not found"))
			},
			wantErr: `failed to read "missing.md" at ref "deadbeef": 404 not found`,
		},
		{
			name: "--ref accepts a branch name and forwards it",
			cli:  "README.md --ref feature/foo",
			setupMocks: func(t *testing.T, tc *gitlabtesting.TestClient) {
				t.Helper()
				tc.MockRepositoryFiles.EXPECT().
					GetRawFile("OWNER/REPO", "README.md", gomock.Cond(func(o *gitlab.GetRawFileOptions) bool {
						return o != nil && o.Ref != nil && *o.Ref == "feature/foo"
					})).
					Return([]byte("ok\n"), nil, nil)
			},
			wantStdout: "ok\n",
		},
		{
			name: "--lfs forwards LFS option",
			cli:  "assets/big.bin --ref deadbeef --lfs",
			setupMocks: func(t *testing.T, tc *gitlabtesting.TestClient) {
				t.Helper()
				tc.MockRepositoryFiles.EXPECT().
					GetRawFile("OWNER/REPO", "assets/big.bin", gomock.Cond(func(o *gitlab.GetRawFileOptions) bool {
						return o != nil && o.LFS != nil && *o.LFS
					})).
					Return([]byte("\x00\x01\x02"), nil, nil)
			},
			wantStdout: "\x00\x01\x02",
		},
		{
			name: "--lfs unset omits LFS option",
			cli:  "README.md --ref deadbeef",
			setupMocks: func(t *testing.T, tc *gitlabtesting.TestClient) {
				t.Helper()
				tc.MockRepositoryFiles.EXPECT().
					GetRawFile("OWNER/REPO", "README.md", gomock.Cond(func(o *gitlab.GetRawFileOptions) bool {
						return o != nil && o.LFS == nil
					})).
					Return([]byte("ok"), nil, nil)
			},
			wantStdout: "ok",
		},
		{
			name: "--repo override is honored",
			cli:  "README.md --ref deadbeef --repo gitlab-org/cli",
			setupMocks: func(t *testing.T, tc *gitlabtesting.TestClient) {
				t.Helper()
				tc.MockRepositoryFiles.EXPECT().
					GetRawFile("gitlab-org/cli", "README.md", gomock.Any()).
					Return([]byte("hi"), nil, nil)
			},
			wantStdout: "hi",
		},
		{
			name:             "apiClient factory error surfaces from complete()",
			cli:              "README.md --ref deadbeef",
			apiClientInitErr: errors.New("api client init failed"),
			wantErr:          "api client init failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testClient := gitlabtesting.NewTestClient(t)
			if tc.setupMocks != nil {
				tc.setupMocks(t, testClient)
			}

			apiClient, err := api.NewClient(
				func(*http.Client) (gitlab.AuthSource, error) {
					return gitlab.AccessTokenAuthSource{Token: ""}, nil
				},
				api.WithGitLabClient(testClient.Client),
			)
			require.NoError(t, err)

			factoryOpts := []cmdtest.FactoryOption{
				cmdtest.WithGitLabClient(testClient.Client),
				cmdtest.WithBranch("main"),
				cmdtest.WithBaseRepo("OWNER", "REPO", glinstance.DefaultHostname),
				cmdtest.WithApiClient(apiClient),
			}
			if tc.apiClientInitErr != nil {
				factoryOpts = append(factoryOpts, withApiClientErrorOption(tc.apiClientInitErr))
			}

			cmdExec := cmdtest.SetupCmdForTest(t, NewCmdFileGet, false, factoryOpts...)

			out, err := cmdExec(tc.cli)

			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			if tc.wantStdout != "" {
				assert.Equal(t, tc.wantStdout, out.String())
			}
		})
	}
}
