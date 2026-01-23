//go:build !integration

package lint

import (
	"fmt"
	"path"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	gitlabtesting "gitlab.com/gitlab-org/api/client-go/testing"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
)

func Test_lintRun(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name             string
		testFile         string
		cliArgs          string
		StdOut           string
		wantErr          bool
		errMsg           string
		showHaveBaseRepo bool
		setupMock        func(tc *gitlabtesting.TestClient)
	}

	tests := []testCase{
		{
			name:             "with invalid path specified",
			testFile:         "WRONG_PATH",
			StdOut:           "",
			wantErr:          true,
			errMsg:           "WRONG_PATH: no such file or directory",
			showHaveBaseRepo: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockProjects.EXPECT().
					GetProject("OWNER/REPO", gomock.Any()).
					Return(&gitlab.Project{
						ID: 123,
					}, nil, nil)
			},
		},
		{
			name:             "without base repo",
			testFile:         ".gitlab.ci.yaml",
			StdOut:           "",
			wantErr:          true,
			errMsg:           "You must be in a GitLab project repository for this action.\nError: no base repo present",
			showHaveBaseRepo: false,
			setupMock: func(tc *gitlabtesting.TestClient) {
				// No mock needed - fails before API call
			},
		},
		{
			name:             "when a valid path is specified and yaml is valid",
			testFile:         ".gitlab-ci.yaml",
			StdOut:           "Validating...\n✓ CI/CD YAML is valid!\n",
			wantErr:          false,
			errMsg:           "",
			showHaveBaseRepo: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockProjects.EXPECT().
					GetProject("OWNER/REPO", gomock.Any()).
					Return(&gitlab.Project{
						ID: 123,
					}, nil, nil)
				tc.MockValidate.EXPECT().
					ProjectNamespaceLint(int64(123), gomock.Any()).
					Return(&gitlab.ProjectLintResult{
						Valid: true,
					}, nil, nil)
			},
		},
		{
			name:             "when --dry-run is used without --ref",
			testFile:         ".gitlab-ci.yaml",
			cliArgs:          "--dry-run",
			StdOut:           "Validating...\n✓ CI/CD YAML is valid!\n",
			wantErr:          false,
			errMsg:           "",
			showHaveBaseRepo: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockProjects.EXPECT().
					GetProject("OWNER/REPO", gomock.Any()).
					Return(&gitlab.Project{
						ID: 123,
					}, nil, nil)
				tc.MockValidate.EXPECT().
					ProjectNamespaceLint(int64(123), gomock.Any()).
					Return(&gitlab.ProjectLintResult{
						Valid: true,
					}, nil, nil)
			},
		},
		{
			name:             "when --dry-run is used with --ref",
			testFile:         ".gitlab-ci.yaml",
			cliArgs:          "--dry-run --ref=main",
			StdOut:           "Validating...\n✓ CI/CD YAML is valid!\n",
			wantErr:          false,
			errMsg:           "",
			showHaveBaseRepo: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockProjects.EXPECT().
					GetProject("OWNER/REPO", gomock.Any()).
					Return(&gitlab.Project{
						ID: 123,
					}, nil, nil)
				tc.MockValidate.EXPECT().
					ProjectNamespaceLint(int64(123), gomock.Any()).
					Return(&gitlab.ProjectLintResult{
						Valid: true,
					}, nil, nil)
			},
		},
		{
			name:             "component template with component context flags",
			testFile:         "component-template.yaml",
			cliArgs:          "--component-name=my-component --component-version=1.0.0 --component-sha=abc123 --component-reference=gitlab.com/org/project/my-component@1.0.0",
			StdOut:           "Validating...\n✓ CI/CD YAML is valid!\n",
			wantErr:          false,
			errMsg:           "",
			showHaveBaseRepo: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockProjects.EXPECT().
					GetProject("OWNER/REPO", gomock.Any()).
					Return(&gitlab.Project{
						ID: 123,
					}, nil, nil)
				tc.MockValidate.EXPECT().
					ProjectNamespaceLint(int64(123), gomock.Any()).
					Return(&gitlab.ProjectLintResult{
						Valid: true,
					}, nil, nil)
			},
		},
		{
			name:             "component template with partial component context flags",
			testFile:         "component-template.yaml",
			cliArgs:          "--component-name=my-component --component-version=2.0.0",
			StdOut:           "Validating...\n✓ CI/CD YAML is valid!\n",
			wantErr:          false,
			errMsg:           "",
			showHaveBaseRepo: true,
			setupMock: func(tc *gitlabtesting.TestClient) {
				tc.MockProjects.EXPECT().
					GetProject("OWNER/REPO", gomock.Any()).
					Return(&gitlab.Project{
						ID: 123,
					}, nil, nil)
				tc.MockValidate.EXPECT().
					ProjectNamespaceLint(int64(123), gomock.Any()).
					Return(&gitlab.ProjectLintResult{
						Valid: true,
					}, nil, nil)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// GIVEN
			testClient := gitlabtesting.NewTestClient(t)
			tt.setupMock(testClient)

			_, filename, _, _ := runtime.Caller(0)
			args := path.Join(path.Dir(filename), "testdata", tt.testFile)
			if tt.cliArgs != "" {
				args += " " + tt.cliArgs
			}

			opts := []cmdtest.FactoryOption{
				cmdtest.WithGitLabClient(testClient.Client),
			}
			if !tt.showHaveBaseRepo {
				opts = append(opts, cmdtest.WithBaseRepoError(fmt.Errorf("no base repo present")))
			}

			exec := cmdtest.SetupCmdForTest(t, NewCmdLint, false, opts...)

			// WHEN
			result, err := exec(args)

			// THEN
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tt.StdOut, result.String())
		})
	}
}

func Test_replaceComponentContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		content            string
		componentName      string
		componentVersion   string
		componentSHA       string
		componentReference string
		expected           string
	}{
		{
			name:             "replaces component.name",
			content:          `image: registry.example.com/$[[ component.name ]]:latest`,
			componentName:    "my-component",
			componentVersion: "",
			expected:         `image: registry.example.com/my-component:latest`,
		},
		{
			name:             "replaces component.version",
			content:          `image: registry.example.com/app:$[[ component.version ]]`,
			componentVersion: "1.2.3",
			expected:         `image: registry.example.com/app:1.2.3`,
		},
		{
			name:         "replaces component.sha",
			content:      `script: echo "SHA: $[[ component.sha ]]"`,
			componentSHA: "abc123def456",
			expected:     `script: echo "SHA: abc123def456"`,
		},
		{
			name:               "replaces component.reference",
			content:            `script: echo "Reference: $[[ component.reference ]]"`,
			componentReference: "gitlab.com/org/project/my-component@1.0.0",
			expected:           `script: echo "Reference: gitlab.com/org/project/my-component@1.0.0"`,
		},
		{
			name:               "replaces all component context fields",
			content:            `image: $[[ component.name ]]:$[[ component.version ]] # SHA: $[[ component.sha ]], Ref: $[[ component.reference ]]`,
			componentName:      "my-app",
			componentVersion:   "2.0.0",
			componentSHA:       "sha256",
			componentReference: "example.com/org/project/my-app@2.0.0",
			expected:           `image: my-app:2.0.0 # SHA: sha256, Ref: example.com/org/project/my-app@2.0.0`,
		},
		{
			name:             "handles whitespace in expressions",
			content:          `image: $[[component.name]]:$[[ component.version ]]`,
			componentName:    "app",
			componentVersion: "1.0.0",
			expected:         `image: app:1.0.0`,
		},
		{
			name:             "handles extra whitespace in expressions",
			content:          `image: $[[  component.name  ]]:$[[   component.version   ]]`,
			componentName:    "app",
			componentVersion: "1.0.0",
			expected:         `image: app:1.0.0`,
		},
		{
			name:             "leaves unset fields unchanged",
			content:          `image: $[[ component.name ]]:$[[ component.version ]] # SHA: $[[ component.sha ]]`,
			componentName:    "app",
			componentVersion: "",
			componentSHA:     "",
			expected:         `image: app:$[[ component.version ]] # SHA: $[[ component.sha ]]`,
		},
		{
			name:     "returns unchanged content when no flags provided",
			content:  `image: $[[ component.name ]]:$[[ component.version ]]`,
			expected: `image: $[[ component.name ]]:$[[ component.version ]]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := &options{
				componentName:      tt.componentName,
				componentVersion:   tt.componentVersion,
				componentSHA:       tt.componentSHA,
				componentReference: tt.componentReference,
			}

			result := opts.replaceComponentContext(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_hasComponentContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name               string
		componentName      string
		componentVersion   string
		componentSHA       string
		componentReference string
		expected           bool
	}{
		{
			name:     "returns false when no flags provided",
			expected: false,
		},
		{
			name:          "returns true when componentName is provided",
			componentName: "my-component",
			expected:      true,
		},
		{
			name:             "returns true when componentVersion is provided",
			componentVersion: "1.0.0",
			expected:         true,
		},
		{
			name:         "returns true when componentSHA is provided",
			componentSHA: "abc123",
			expected:     true,
		},
		{
			name:               "returns true when componentReference is provided",
			componentReference: "example.com/org/project/component@1.0.0",
			expected:           true,
		},
		{
			name:               "returns true when all flags are provided",
			componentName:      "my-component",
			componentVersion:   "1.0.0",
			componentSHA:       "abc123",
			componentReference: "example.com/org/project/component@1.0.0",
			expected:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			opts := &options{
				componentName:      tt.componentName,
				componentVersion:   tt.componentVersion,
				componentSHA:       tt.componentSHA,
				componentReference: tt.componentReference,
			}

			result := opts.hasComponentContext()
			assert.Equal(t, tt.expected, result)
		})
	}
}
