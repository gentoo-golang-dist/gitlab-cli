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
