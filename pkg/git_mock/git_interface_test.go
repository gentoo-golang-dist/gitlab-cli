package git_mock

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestCurrentBranch(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*MockGitInterface)
		expectedBranch string
		expectedError  error
	}{
		{
			name: "successful branch retrieval",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().CurrentBranch().Return("main", nil)
			},
			expectedBranch: "main",
			expectedError:  nil,
		},
		{
			name: "detached head error",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().CurrentBranch().Return("", ErrNotOnAnyBranch)
			},
			expectedBranch: "",
			expectedError:  ErrNotOnAnyBranch,
		},
		{
			name: "unknown error",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().CurrentBranch().Return("", errors.New("error!"))
			},
			expectedBranch: "",
			expectedError:  errors.New("error!"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit)

			branch, err := mockGit.CurrentBranch()

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedBranch, branch)
		})
	}
}

func TestGetDefaultBranch(t *testing.T) {
	tests := []struct {
		name           string
		remote         string
		setupMock      func(*MockGitInterface, string)
		expectedBranch string
		expectedError  error
	}{
		{
			name:   "successful default branch retrieval",
			remote: "origin",
			setupMock: func(m *MockGitInterface, remote string) {
				m.EXPECT().GetDefaultBranch(remote).Return("main", nil)
			},
			expectedBranch: "main",
			expectedError:  nil,
		},
		{
			name:   "remote show command fails",
			remote: "origin",
			setupMock: func(m *MockGitInterface, remote string) {
				m.EXPECT().GetDefaultBranch(remote).Return(DefaultBranch, errors.New("could not find default branch"))
			},
			expectedBranch: DefaultBranch,
			expectedError:  errors.New("could not find default branch"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.remote)

			branch, err := mockGit.GetDefaultBranch(tt.remote)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedBranch, branch)
		})
	}
}

func TestRemoteBranchExists(t *testing.T) {
	tests := []struct {
		name          string
		remote        string
		branch        string
		setupMock     func(*MockGitInterface, string, string)
		expected      bool
		expectedError error
	}{
		{
			name:   "branch exists",
			remote: "origin",
			branch: "feature",
			setupMock: func(m *MockGitInterface, remote string, branch string) {
				m.EXPECT().RemoteBranchExists(remote, branch).Return(true, nil)
			},
			expected:      true,
			expectedError: nil,
		},
		{
			name:   "branch does not exist",
			branch: "BranchDoesNotExist",
			setupMock: func(m *MockGitInterface, remote string, branch string) {
				m.EXPECT().RemoteBranchExists(remote, branch).Return(false, errors.New("could not find remote branch"))
			},
			expected:      false,
			expectedError: errors.New("could not find remote branch"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.remote, tt.branch)

			exists, err := mockGit.RemoteBranchExists(tt.remote, tt.branch)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expected, exists)
		})
	}
}

func TestDeleteLocalBranch(t *testing.T) {
	tests := []struct {
		name          string
		branch        string
		setupMock     func(*MockGitInterface, string)
		expectedError error
	}{
		{
			name:   "successful branch deletion",
			branch: "feature",
			setupMock: func(m *MockGitInterface, branch string) {
				m.EXPECT().DeleteLocalBranch(branch).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:   "branch deletion fails",
			branch: "BranchDoesNotExist",
			setupMock: func(m *MockGitInterface, branch string) {
				m.EXPECT().DeleteLocalBranch(branch).Return(errors.New("could not delete local branch"))
			},
			expectedError: errors.New("could not delete local branch"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.branch)

			err := mockGit.DeleteLocalBranch(tt.branch)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCheckoutBranch(t *testing.T) {
	tests := []struct {
		name          string
		branch        string
		setupMock     func(*MockGitInterface, string)
		expectedError error
	}{
		{
			name:   "successful branch checkout",
			branch: "feature",
			setupMock: func(m *MockGitInterface, branch string) {
				m.EXPECT().CheckoutBranch(branch).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:   "branch checkout fails",
			branch: "BranchDoesNotExist",
			setupMock: func(m *MockGitInterface, branch string) {
				m.EXPECT().CheckoutBranch(branch).Return(errors.New("could not checkout branch"))
			},
			expectedError: errors.New("could not checkout branch"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.branch)

			err := mockGit.CheckoutBranch(tt.branch)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewStandardGitRunner(t *testing.T) {
	tests := []struct {
		name          string
		gitBinary     string
		expectedValue string
	}{
		{
			name:          "with provided binary path",
			gitBinary:     "/usr/local/bin/git",
			expectedValue: "/usr/local/bin/git",
		},
		{
			name:          "with empty binary path",
			gitBinary:     "",
			expectedValue: "git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := NewStandardGitRunner(tt.gitBinary)
			require.Equal(t, tt.expectedValue, runner.gitBinary)
		})
	}
}

func TestGetRemoteURL(t *testing.T) {
	tests := []struct {
		name          string
		remoteAlias   string
		setupMock     func(*MockGitInterface, string)
		expectedURL   string
		expectedError error
	}{
		{
			name:        "successful remote URL retrieval",
			remoteAlias: "origin",
			setupMock: func(m *MockGitInterface, remoteAlias string) {
				m.EXPECT().GetRemoteURL(remoteAlias).Return("https://github.com/user/repo.git", nil)
			},
			expectedURL:   "https://github.com/user/repo.git",
			expectedError: nil,
		},
		{
			name:        "remote URL not found",
			remoteAlias: "invalid",
			setupMock: func(m *MockGitInterface, remoteAlias string) {
				m.EXPECT().GetRemoteURL(remoteAlias).Return("", errors.New("unknown config key: remote.invalid.url"))
			},
			expectedURL:   "",
			expectedError: errors.New("unknown config key: remote.invalid.url"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.remoteAlias)

			url, err := mockGit.GetRemoteURL(tt.remoteAlias)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedURL, url)
		})
	}
}

func TestShowRefs(t *testing.T) {
	tests := []struct {
		name          string
		refs          []string
		setupMock     func(*MockGitInterface, []string)
		expectedRefs  []Ref
		expectedError error
	}{
		{
			name: "successful refs retrieval",
			refs: []string{"refs/heads/main"},
			setupMock: func(m *MockGitInterface, refs []string) {
				args := make([]any, len(refs))
				for i, ref := range refs {
					args[i] = ref
				}
				m.EXPECT().ShowRefs(args...).Return([]Ref{
					{Hash: "abc123", Name: "refs/heads/main"},
				}, nil)
			},
			expectedRefs: []Ref{
				{Hash: "abc123", Name: "refs/heads/main"},
			},
			expectedError: nil,
		},
		{
			name: "refs not found",
			refs: []string{"refs/heads/nonexistent"},
			setupMock: func(m *MockGitInterface, refs []string) {
				args := make([]any, len(refs))
				for i, ref := range refs {
					args[i] = ref
				}
				m.EXPECT().ShowRefs(args...).Return([]Ref{}, errors.New("fatal: bad ref"))
			},
			expectedRefs:  []Ref{},
			expectedError: errors.New("fatal: bad ref"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.refs)

			refs, err := mockGit.ShowRefs(tt.refs...)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedRefs, refs)
		})
	}
}

func TestListRemotes(t *testing.T) {
	tests := []struct {
		name            string
		setupMock       func(*MockGitInterface)
		expectedRemotes []string
		expectedError   error
	}{
		{
			name: "successful remotes listing",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().ListRemotes().Return([]string{
					"origin\thttps://github.com/user/repo.git (fetch)",
					"origin\thttps://github.com/user/repo.git (push)",
				}, nil)
			},
			expectedRemotes: []string{
				"origin\thttps://github.com/user/repo.git (fetch)",
				"origin\thttps://github.com/user/repo.git (push)",
			},
			expectedError: nil,
		},
		{
			name: "no remotes",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().ListRemotes().Return([]string{}, nil)
			},
			expectedRemotes: []string{},
			expectedError:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit)

			remotes, err := mockGit.ListRemotes()

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedRemotes, remotes)
		})
	}
}

func TestConfig(t *testing.T) {
	tests := []struct {
		name          string
		key           string
		setupMock     func(*MockGitInterface, string)
		expectedValue string
		expectedError error
	}{
		{
			name: "successful config retrieval",
			key:  "user.name",
			setupMock: func(m *MockGitInterface, key string) {
				m.EXPECT().Config(key).Return("John Doe", nil)
			},
			expectedValue: "John Doe",
			expectedError: nil,
		},
		{
			name: "config key not found",
			key:  "invalid.key",
			setupMock: func(m *MockGitInterface, key string) {
				m.EXPECT().Config(key).Return("", errors.New("unknown config key: invalid.key"))
			},
			expectedValue: "",
			expectedError: errors.New("unknown config key: invalid.key"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.key)

			value, err := mockGit.Config(tt.key)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedValue, value)
		})
	}
}

func TestSetUpstream(t *testing.T) {
	tests := []struct {
		name          string
		remote        string
		branch        string
		setupMock     func(*MockGitInterface, string, string)
		expectedError error
	}{
		{
			name:   "successful upstream set",
			remote: "origin",
			branch: "main",
			setupMock: func(m *MockGitInterface, remote, branch string) {
				m.EXPECT().SetUpstream(remote, branch).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:   "upstream set fails",
			remote: "origin",
			branch: "main",
			setupMock: func(m *MockGitInterface, remote, branch string) {
				m.EXPECT().SetUpstream(remote, branch).Return(errors.New("could not set upstream"))
			},
			expectedError: errors.New("could not set upstream"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.remote, tt.branch)

			err := mockGit.SetUpstream(tt.remote, tt.branch)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAddRemote(t *testing.T) {
	tests := []struct {
		name          string
		remoteName    string
		remoteURL     string
		setupMock     func(*MockGitInterface, string, string)
		expectedError error
	}{
		{
			name:       "successful remote add",
			remoteName: "upstream",
			remoteURL:  "https://github.com/upstream/repo.git",
			setupMock: func(m *MockGitInterface, name, url string) {
				m.EXPECT().AddRemote(name, url).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:       "remote already exists",
			remoteName: "origin",
			remoteURL:  "https://github.com/user/repo.git",
			setupMock: func(m *MockGitInterface, name, url string) {
				m.EXPECT().AddRemote(name, url).Return(errors.New("could not add remote"))
			},
			expectedError: errors.New("could not add remote"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.remoteName, tt.remoteURL)

			err := mockGit.AddRemote(tt.remoteName, tt.remoteURL)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSetRemoteResolution(t *testing.T) {
	tests := []struct {
		name          string
		remoteName    string
		resolution    string
		setupMock     func(*MockGitInterface, string, string)
		expectedError error
	}{
		{
			name:       "successful remote resolution set",
			remoteName: "origin",
			resolution: "https://gitlab.com/user/repo",
			setupMock: func(m *MockGitInterface, name, resolution string) {
				m.EXPECT().SetRemoteResolution(name, resolution).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:       "remote resolution set fails",
			remoteName: "origin",
			resolution: "https://gitlab.com/user/repo",
			setupMock: func(m *MockGitInterface, name, resolution string) {
				m.EXPECT().SetRemoteResolution(name, resolution).Return(errors.New("setting git config"))
			},
			expectedError: errors.New("setting git config"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.remoteName, tt.resolution)

			err := mockGit.SetRemoteResolution(tt.remoteName, tt.resolution)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSetRemoteConfig(t *testing.T) {
	tests := []struct {
		name          string
		remote        string
		key           string
		value         string
		setupMock     func(*MockGitInterface, string, string, string)
		expectedError error
	}{
		{
			name:   "successful remote config set",
			remote: "origin",
			key:    "pushurl",
			value:  "https://github.com/user/repo.git",
			setupMock: func(m *MockGitInterface, remote, key, value string) {
				m.EXPECT().SetRemoteConfig(remote, key, value).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:   "remote config set fails",
			remote: "origin",
			key:    "pushurl",
			value:  "https://github.com/user/repo.git",
			setupMock: func(m *MockGitInterface, remote, key, value string) {
				m.EXPECT().SetRemoteConfig(remote, key, value).Return(errors.New("setting git config"))
			},
			expectedError: errors.New("setting git config"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.remote, tt.key, tt.value)

			err := mockGit.SetRemoteConfig(tt.remote, tt.key, tt.value)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetAllConfig(t *testing.T) {
	tests := []struct {
		name           string
		key            string
		setupMock      func(*MockGitInterface, string)
		expectedOutput []byte
		expectedError  error
	}{
		{
			name: "successful config retrieval",
			key:  "remote.origin.url",
			setupMock: func(m *MockGitInterface, key string) {
				m.EXPECT().GetAllConfig(key).Return([]byte("https://github.com/user/repo.git\n"), nil)
			},
			expectedOutput: []byte("https://github.com/user/repo.git\n"),
			expectedError:  nil,
		},
		{
			name: "config key not found",
			key:  "nonexistent.key",
			setupMock: func(m *MockGitInterface, key string) {
				m.EXPECT().GetAllConfig(key).Return(nil, nil)
			},
			expectedOutput: nil,
			expectedError:  nil,
		},
		{
			name: "invalid config key",
			key:  "invalid",
			setupMock: func(m *MockGitInterface, key string) {
				m.EXPECT().GetAllConfig(key).Return(nil, errors.New("incorrect Git configuration key."))
			},
			expectedOutput: nil,
			expectedError:  errors.New("incorrect Git configuration key."),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.key)

			output, err := mockGit.GetAllConfig(tt.key)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedOutput, output)
		})
	}
}

func TestCheckoutNewBranch(t *testing.T) {
	tests := []struct {
		name          string
		branch        string
		setupMock     func(*MockGitInterface, string)
		expectedError error
	}{
		{
			name:   "successful new branch creation",
			branch: "feature",
			setupMock: func(m *MockGitInterface, branch string) {
				m.EXPECT().CheckoutNewBranch(branch).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:   "branch creation fails",
			branch: "feature",
			setupMock: func(m *MockGitInterface, branch string) {
				m.EXPECT().CheckoutNewBranch(branch).Return(errors.New("could not create new branch"))
			},
			expectedError: errors.New("could not create new branch"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.branch)

			err := mockGit.CheckoutNewBranch(tt.branch)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUncommittedChangeCount(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*MockGitInterface)
		expectedCount int
		expectedError error
	}{
		{
			name: "no uncommitted changes",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().UncommittedChangeCount().Return(0, nil)
			},
			expectedCount: 0,
			expectedError: nil,
		},
		{
			name: "three uncommitted changes",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().UncommittedChangeCount().Return(3, nil)
			},
			expectedCount: 3,
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit)

			count, err := mockGit.UncommittedChangeCount()

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedCount, count)
		})
	}
}

func TestLatestCommit(t *testing.T) {
	tests := []struct {
		name           string
		ref            string
		setupMock      func(*MockGitInterface, string)
		expectedCommit *Commit
		expectedError  error
	}{
		{
			name: "successful commit retrieval",
			ref:  "HEAD",
			setupMock: func(m *MockGitInterface, ref string) {
				m.EXPECT().LatestCommit(ref).Return(&Commit{
					Sha:   "abc123",
					Title: "Fix bug in feature",
				}, nil)
			},
			expectedCommit: &Commit{
				Sha:   "abc123",
				Title: "Fix bug in feature",
			},
			expectedError: nil,
		},
		{
			name: "commit not found",
			ref:  "nonexistent",
			setupMock: func(m *MockGitInterface, ref string) {
				m.EXPECT().LatestCommit(ref).Return(&Commit{}, errors.New("could not get latest commit"))
			},
			expectedCommit: &Commit{},
			expectedError:  errors.New("could not get latest commit"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.ref)

			commit, err := mockGit.LatestCommit(tt.ref)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedCommit, commit)
		})
	}
}

func TestPush(t *testing.T) {
	tests := []struct {
		name          string
		remote        string
		ref           string
		setupMock     func(*MockGitInterface, string, string)
		expectedError error
	}{
		{
			name:   "successful push",
			remote: "origin",
			ref:    "main",
			setupMock: func(m *MockGitInterface, remote, ref string) {
				m.EXPECT().Push(remote, ref).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:   "push fails",
			remote: "origin",
			ref:    "main",
			setupMock: func(m *MockGitInterface, remote, ref string) {
				m.EXPECT().Push(remote, ref).Return(errors.New("could not push"))
			},
			expectedError: errors.New("could not push"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.remote, tt.ref)

			err := mockGit.Push(tt.remote, tt.ref)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGitUserName(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*MockGitInterface)
		expectedName  string
		expectedError error
	}{
		{
			name: "successful user name retrieval",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().GitUserName().Return("John Doe", nil)
			},
			expectedName:  "John Doe",
			expectedError: nil,
		},
		{
			name: "user name not configured",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().GitUserName().Return("", errors.New("could not get user name"))
			},
			expectedName:  "",
			expectedError: errors.New("could not get user name"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit)

			name, err := mockGit.GitUserName()

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedName, name)
		})
	}
}

func TestCommits(t *testing.T) {
	tests := []struct {
		name            string
		baseRef         string
		headRef         string
		setupMock       func(*MockGitInterface, string, string)
		expectedCommits []*Commit
		expectedError   error
	}{
		{
			name:    "successful commits retrieval",
			baseRef: "main",
			headRef: "feature",
			setupMock: func(m *MockGitInterface, baseRef, headRef string) {
				m.EXPECT().Commits(baseRef, headRef).Return([]*Commit{
					{Sha: "abc123", Title: "Add feature"},
					{Sha: "def456", Title: "Fix bug"},
				}, nil)
			},
			expectedCommits: []*Commit{
				{Sha: "abc123", Title: "Add feature"},
				{Sha: "def456", Title: "Fix bug"},
			},
			expectedError: nil,
		},
		{
			name:    "no commits between refs",
			baseRef: "main",
			headRef: "main",
			setupMock: func(m *MockGitInterface, baseRef, headRef string) {
				m.EXPECT().Commits(baseRef, headRef).Return([]*Commit{}, errors.New("could not find any commits between main and main."))
			},
			expectedCommits: []*Commit{},
			expectedError:   errors.New("could not find any commits between main and main."),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.baseRef, tt.headRef)

			commits, err := mockGit.Commits(tt.baseRef, tt.headRef)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedCommits, commits)
		})
	}
}

func TestCommitBody(t *testing.T) {
	tests := []struct {
		name          string
		sha           string
		setupMock     func(*MockGitInterface, string)
		expectedBody  string
		expectedError error
	}{
		{
			name: "successful commit body retrieval",
			sha:  "abc123",
			setupMock: func(m *MockGitInterface, sha string) {
				m.EXPECT().CommitBody(sha).Return("This is the commit body\n\nWith multiple lines", nil)
			},
			expectedBody:  "This is the commit body\n\nWith multiple lines",
			expectedError: nil,
		},
		{
			name: "empty commit body",
			sha:  "def456",
			setupMock: func(m *MockGitInterface, sha string) {
				m.EXPECT().CommitBody(sha).Return("", nil)
			},
			expectedBody:  "",
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.sha)

			body, err := mockGit.CommitBody(tt.sha)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedBody, body)
		})
	}
}

func TestHasLocalBranch(t *testing.T) {
	tests := []struct {
		name          string
		branch        string
		setupMock     func(*MockGitInterface, string)
		expected      bool
		expectedError error
	}{
		{
			name:   "branch exists",
			branch: "feature",
			setupMock: func(m *MockGitInterface, branch string) {
				m.EXPECT().HasLocalBranch(branch).Return(true, nil)
			},
			expected:      true,
			expectedError: nil,
		},
		{
			name:   "branch does not exist",
			branch: "nonexistent",
			setupMock: func(m *MockGitInterface, branch string) {
				m.EXPECT().HasLocalBranch(branch).Return(false, nil)
			},
			expected:      false,
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.branch)

			exists, err := mockGit.HasLocalBranch(tt.branch)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expected, exists)
		})
	}
}

func Test_parseDefaultBranch(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedBranch string
		expectedError  error
	}{
		{
			name: "parses HEAD branch from standard output",
			input: `* remote origin
  Fetch URL: git@gitlab.com:gitlab-org/cli.git
  Push  URL: git@gitlab.com:gitlab-org/cli.git
  HEAD branch: main
  Remote branches:
    main tracked
    dev  tracked`,
			expectedBranch: "main",
			expectedError:  nil,
		},
		{
			name: "handles extra spaces around HEAD branch",
			input: `* remote origin
  Fetch URL: git@gitlab.com:example/repo.git
  Push  URL: git@gitlab.com:example/repo.git
  HEAD branch:    main    
  Remote branches:
    main tracked`,
			expectedBranch: "main",
			expectedError:  nil,
		},
		{
			name: "parses branch name with dashes",
			input: `* remote origin
  Fetch URL: git@gitlab.com:example/repo.git
  Push  URL: git@gitlab.com:example/repo.git
  HEAD branch: release-2.0
  Remote branches:
    release-2.0 tracked`,
			expectedBranch: "release-2.0",
			expectedError:  nil,
		},
		{
			name: "parses branch name with slashes",
			input: `* remote origin
  Fetch URL: git@gitlab.com:example/repo.git
  Push  URL: git@gitlab.com:example/repo.git
  HEAD branch: feature/new-feature
  Remote branches:
    feature/new-feature tracked`,
			expectedBranch: "feature/new-feature",
			expectedError:  nil,
		},
		{
			name: "falls back to parsing (HEAD) marker",
			input: `* remote origin
  Fetch URL: git@gitlab.com:example/repo.git
  Push  URL: git@gitlab.com:example/repo.git
  Remote branches:
    main (HEAD)
    dev tracked`,
			expectedBranch: "main",
			expectedError:  nil,
		},
		{
			name: "handles (HEAD) marker with tracked",
			input: `* remote origin
  Fetch URL: git@gitlab.com:example/repo.git
  Push  URL: git@gitlab.com:example/repo.git
  Remote branches:
    master tracked (HEAD)
    dev tracked`,
			expectedBranch: "master",
			expectedError:  nil,
		},
		{
			name: "errors when no HEAD branch found",
			input: `* remote origin
  Fetch URL: git@gitlab.com:example/repo.git
  Push  URL: git@gitlab.com:example/repo.git
  Remote branches:
    main tracked
    dev tracked`,
			expectedBranch: "",
			expectedError:  errors.New("could not determine default branch from remote output"),
		},
		{
			name:           "errors on empty input",
			input:          "",
			expectedBranch: "",
			expectedError:  errors.New("could not determine default branch from remote output"),
		},
		{
			name: "errors on malformed output",
			input: `This is not valid git remote output
Just some random text`,
			expectedBranch: "",
			expectedError:  errors.New("could not determine default branch from remote output"),
		},
		{
			name:           "handles Windows-style line endings",
			input:          "* remote origin\r\n  Fetch URL: git@gitlab.com:example/repo.git\r\n  Push  URL: git@gitlab.com:example/repo.git\r\n  HEAD branch: main\r\n  Remote branches:\r\n    main tracked",
			expectedBranch: "main",
			expectedError:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branch, err := parseDefaultBranch([]byte(tt.input))

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedBranch, branch)
		})
	}
}

func TestReadBranchConfig(t *testing.T) {
	tests := []struct {
		name           string
		branch         string
		setupMock      func(*MockGitInterface, string)
		expectedConfig BranchConfig
	}{
		{
			name:   "successful branch config retrieval with remote name",
			branch: "main",
			setupMock: func(m *MockGitInterface, branch string) {
				m.EXPECT().ReadBranchConfig(branch).Return(BranchConfig{
					RemoteName: "origin",
					MergeRef:   "refs/heads/main",
				})
			},
			expectedConfig: BranchConfig{
				RemoteName: "origin",
				MergeRef:   "refs/heads/main",
			},
		},
		{
			name:   "branch config with remote URL",
			branch: "feature",
			setupMock: func(m *MockGitInterface, branch string) {
				m.EXPECT().ReadBranchConfig(branch).Return(BranchConfig{
					RemoteURL: "https://github.com/user/repo.git",
					MergeRef:  "refs/heads/feature",
				})
			},
			expectedConfig: BranchConfig{
				RemoteURL: "https://github.com/user/repo.git",
				MergeRef:  "refs/heads/feature",
			},
		},
		{
			name:   "empty branch config",
			branch: "orphan",
			setupMock: func(m *MockGitInterface, branch string) {
				m.EXPECT().ReadBranchConfig(branch).Return(BranchConfig{})
			},
			expectedConfig: BranchConfig{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.branch)

			config := mockGit.ReadBranchConfig(tt.branch)

			require.Equal(t, tt.expectedConfig, config)
		})
	}
}

func TestRunClone(t *testing.T) {
	tests := []struct {
		name          string
		cloneURL      string
		target        string
		args          []string
		setupMock     func(*MockGitInterface, string, string, []string)
		expectedDir   string
		expectedError error
	}{
		{
			name:     "successful clone with target",
			cloneURL: "https://github.com/user/repo.git",
			target:   "myrepo",
			args:     []string{"--depth", "1"},
			setupMock: func(m *MockGitInterface, cloneURL, target string, args []string) {
				m.EXPECT().RunClone(cloneURL, target, args).Return(target, nil)
			},
			expectedDir:   "myrepo",
			expectedError: nil,
		},
		{
			name:     "successful clone without target",
			cloneURL: "https://github.com/user/repo.git",
			target:   "",
			args:     []string{},
			setupMock: func(m *MockGitInterface, cloneURL, target string, args []string) {
				m.EXPECT().RunClone(cloneURL, target, args).Return("repo", nil)
			},
			expectedDir:   "repo",
			expectedError: nil,
		},
		{
			name:     "clone fails",
			cloneURL: "https://github.com/user/nonexistent.git",
			target:   "",
			args:     []string{},
			setupMock: func(m *MockGitInterface, cloneURL, target string, args []string) {
				m.EXPECT().RunClone(cloneURL, target, args).Return("", errors.New("repository not found"))
			},
			expectedDir:   "",
			expectedError: errors.New("repository not found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.cloneURL, tt.target, tt.args)

			dir, err := mockGit.RunClone(tt.cloneURL, tt.target, tt.args)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedDir, dir)
		})
	}
}

func TestAddUpstreamRemote(t *testing.T) {
	tests := []struct {
		name          string
		upstreamURL   string
		cloneDir      string
		setupMock     func(*MockGitInterface, string, string)
		expectedError error
	}{
		{
			name:        "successful upstream remote addition",
			upstreamURL: "https://github.com/upstream/repo.git",
			cloneDir:    "myrepo",
			setupMock: func(m *MockGitInterface, upstreamURL, cloneDir string) {
				m.EXPECT().AddUpstreamRemote(upstreamURL, cloneDir).Return(nil)
			},
			expectedError: nil,
		},
		{
			name:        "upstream remote addition fails",
			upstreamURL: "https://github.com/upstream/repo.git",
			cloneDir:    "nonexistent",
			setupMock: func(m *MockGitInterface, upstreamURL, cloneDir string) {
				m.EXPECT().AddUpstreamRemote(upstreamURL, cloneDir).Return(errors.New("not a git repository"))
			},
			expectedError: errors.New("not a git repository"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.upstreamURL, tt.cloneDir)

			err := mockGit.AddUpstreamRemote(tt.upstreamURL, tt.cloneDir)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestToplevelDir(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*MockGitInterface)
		expectedDir   string
		expectedError error
	}{
		{
			name: "successful toplevel directory retrieval",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().ToplevelDir().Return("/Users/gary/code/work/cli", nil)
			},
			expectedDir:   "/Users/gary/code/work/cli",
			expectedError: nil,
		},
		{
			name: "not in a git repository",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().ToplevelDir().Return("", errors.New("could not get top-level directory"))
			},
			expectedDir:   "",
			expectedError: errors.New("could not get top-level directory"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit)

			dir, err := mockGit.ToplevelDir()

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedDir, dir)
		})
	}
}

func TestDescribeByTags(t *testing.T) {
	tests := []struct {
		name             string
		setupMock        func(*MockGitInterface)
		expectedDescribe string
		expectedError    error
	}{
		{
			name: "successful describe",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().DescribeByTags().Return("v1.2.3-4-gabc123\n", nil)
			},
			expectedDescribe: "v1.2.3-4-gabc123\n",
			expectedError:    nil,
		},
		{
			name: "no tags found",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().DescribeByTags().Return("", errors.New("running describe"))
			},
			expectedDescribe: "",
			expectedError:    errors.New("running describe"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit)

			describe, err := mockGit.DescribeByTags()

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedDescribe, describe)
		})
	}
}

func TestListTags(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*MockGitInterface)
		expectedTags  []string
		expectedError error
	}{
		{
			name: "successful tags listing",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().ListTags().Return([]string{"v1.0.0", "v1.1.0", "v1.2.0"}, nil)
			},
			expectedTags:  []string{"v1.0.0", "v1.1.0", "v1.2.0"},
			expectedError: nil,
		},
		{
			name: "no tags",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().ListTags().Return(nil, nil)
			},
			expectedTags:  nil,
			expectedError: nil,
		},
		{
			name: "tags listing fails",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().ListTags().Return(nil, errors.New("running tag"))
			},
			expectedTags:  nil,
			expectedError: errors.New("running tag"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit)

			tags, err := mockGit.ListTags()

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedTags, tags)
		})
	}
}

func TestRunCmd(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		setupMock     func(*MockGitInterface, []string)
		expectedError error
	}{
		{
			name: "successful command execution",
			args: []string{"status", "--short"},
			setupMock: func(m *MockGitInterface, args []string) {
				m.EXPECT().RunCmd(args).Return(nil)
			},
			expectedError: nil,
		},
		{
			name: "command execution fails",
			args: []string{"invalid-command"},
			setupMock: func(m *MockGitInterface, args []string) {
				m.EXPECT().RunCmd(args).Return(errors.New("git: 'invalid-command' is not a git command"))
			},
			expectedError: errors.New("git: 'invalid-command' is not a git command"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit, tt.args)

			err := mockGit.RunCmd(tt.args)

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestTrackingRef_String(t *testing.T) {
	tests := []struct {
		name     string
		ref      TrackingRef
		expected string
	}{
		{
			name: "standard tracking ref",
			ref: TrackingRef{
				RemoteName: "origin",
				BranchName: "main",
			},
			expected: "refs/remotes/origin/main",
		},
		{
			name: "tracking ref with slash in branch name",
			ref: TrackingRef{
				RemoteName: "upstream",
				BranchName: "feature/new-feature",
			},
			expected: "refs/remotes/upstream/feature/new-feature",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.ref.String()
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestRemotes(t *testing.T) {
	tests := []struct {
		name           string
		setupMock      func(*MockGitInterface)
		expectedRemotes RemoteSet
		expectedError   error
	}{
		{
			name: "successful remotes retrieval",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().Remotes().Return(RemoteSet{
					&Remote{
						Name:     "origin",
						FetchURL: "https://github.com/user/repo.git",
						PushURL:  "https://github.com/user/repo.git",
					},
					&Remote{
						Name:     "upstream",
						FetchURL: "https://github.com/upstream/repo.git",
						PushURL:  "https://github.com/upstream/repo.git",
					},
				}, nil)
			},
			expectedRemotes: RemoteSet{
				&Remote{
					Name:     "origin",
					FetchURL: "https://github.com/user/repo.git",
					PushURL:  "https://github.com/user/repo.git",
				},
				&Remote{
					Name:     "upstream",
					FetchURL: "https://github.com/upstream/repo.git",
					PushURL:  "https://github.com/upstream/repo.git",
				},
			},
			expectedError: nil,
		},
		{
			name: "remotes with resolved URLs",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().Remotes().Return(RemoteSet{
					&Remote{
						Name:     "origin",
						FetchURL: "git@github.com:user/repo.git",
						PushURL:  "git@github.com:user/repo.git",
						Resolved: "https://gitlab.com/user/repo",
					},
				}, nil)
			},
			expectedRemotes: RemoteSet{
				&Remote{
					Name:     "origin",
					FetchURL: "git@github.com:user/repo.git",
					PushURL:  "git@github.com:user/repo.git",
					Resolved: "https://gitlab.com/user/repo",
				},
			},
			expectedError: nil,
		},
		{
			name: "no remotes",
			setupMock: func(m *MockGitInterface) {
				m.EXPECT().Remotes().Return(RemoteSet{}, nil)
			},
			expectedRemotes: RemoteSet{},
			expectedError:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockGit := NewMockGitInterface(ctrl)

			tt.setupMock(mockGit)

			remotes, err := mockGit.Remotes()

			if tt.expectedError != nil {
				require.Error(t, err)
				require.Equal(t, tt.expectedError.Error(), err.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expectedRemotes, remotes)
		})
	}
}

func TestRemote_String(t *testing.T) {
	tests := []struct {
		name     string
		remote   Remote
		expected string
	}{
		{
			name: "remote with name",
			remote: Remote{
				Name:     "origin",
				FetchURL: "https://github.com/user/repo.git",
				PushURL:  "https://github.com/user/repo.git",
			},
			expected: "origin",
		},
		{
			name: "remote with resolved URL",
			remote: Remote{
				Name:     "upstream",
				FetchURL: "git@github.com:upstream/repo.git",
				PushURL:  "git@github.com:upstream/repo.git",
				Resolved: "https://gitlab.com/upstream/repo",
			},
			expected: "upstream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.remote.String()
			require.Equal(t, tt.expected, result)
		})
	}
}
