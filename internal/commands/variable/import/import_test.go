package importCmd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/google/shlex"
	"github.com/stretchr/testify/assert"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/glinstance"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
	"gitlab.com/gitlab-org/cli/internal/testing/httpmock"
)

func Test_NewCmdImport(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		cli      string
		wants    options
		wantsErr bool
	}{
		{
			name:     "no args",
			cli:      "",
			wantsErr: false,
		},
		{
			name:     "with group",
			cli:      "--group mygroup",
			wants:    options{group: "mygroup"},
			wantsErr: false,
		},
		{
			name:     "missing file path",
			cli:      "--file",
			wantsErr: true,
		},
		{
			name:     "with update flag",
			cli:      "--file vars.json --update",
			wants:    options{filePath: "vars.json", update: true},
			wantsErr: false,
		},
		{
			name:     "with stdin flag",
			cli:      "--stdin",
			wants:    options{fromStdin: true},
			wantsErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			io, _, _, _ := cmdtest.TestIOStreams()
			f := cmdtest.NewTestFactory(io)

			argv, err := shlex.Split(test.cli)
			assert.NoError(t, err)

			var gotOpts *options
			cmd := NewCmdImport(f, func(opts *options) error {
				gotOpts = opts
				return nil
			})
			cmd.SetArgs(argv)
			cmd.SetIn(&bytes.Buffer{})
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})

			_, err = cmd.ExecuteC()
			if test.wantsErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			if gotOpts != nil {
				assert.Equal(t, test.wants.group, gotOpts.group)
				assert.Equal(t, test.wants.filePath, gotOpts.filePath)
				assert.Equal(t, test.wants.fromStdin, gotOpts.fromStdin)
				assert.Equal(t, test.wants.update, gotOpts.update)
			}
		})
	}
}

func Test_run_FileAndStdin(t *testing.T) {
	t.Parallel()
	io, _, _, _ := cmdtest.TestIOStreams()
	f := cmdtest.NewTestFactory(io)

	tmpFile, err := os.CreateTemp(t.TempDir(), "vars.json")
	assert.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	variables := []gitlab.ProjectVariable{{Key: "VAR1", Value: "value1"}}
	data, _ := json.Marshal(variables)
	_, _ = tmpFile.Write(data)
	tmpFile.Close()

	tests := []struct {
		name        string
		opts        *options
		expectError string
	}{
		{
			name: "no input",
			opts: &options{
				io:        io,
				apiClient: f.ApiClient,
				baseRepo:  f.BaseRepo,
			},
			expectError: "no input source provided",
		},
		{
			name: "invalid file path",
			opts: &options{
				io:        io,
				apiClient: f.ApiClient,
				baseRepo:  f.BaseRepo,
				filePath:  "missing.json",
			},
			expectError: "failed to read file",
		},
		{
			name: "invalid stdin read",
			opts: &options{
				io:        io,
				apiClient: f.ApiClient,
				baseRepo:  f.BaseRepo,
				fromStdin: true,
			},
			expectError: "failed to read from stdin: no data",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.opts.run()
			assert.ErrorContains(t, err, test.expectError)
		})
	}
}

func Test_importProjectVariables_CreateAndUpdate(t *testing.T) {
	reg := &httpmock.Mocker{MatchURL: httpmock.FullURL}
	defer reg.Verify(t)

	io, _, stdout, _ := cmdtest.TestIOStreams()
	reg.RegisterResponder(http.MethodGet, "https://gitlab.com/api/v4/projects/owner%2Frepo/variables/VAR1",
		httpmock.NewStringResponse(http.StatusNotFound, `{"code":"404","status":"Not Found"}`))

	reg.RegisterResponder(http.MethodPost, "https://gitlab.com/api/v4/projects/owner%2Frepo/variables",
		httpmock.NewStringResponse(http.StatusCreated, `{"key": "VAR1", "value": "value1"}`))

	opts := &options{
		apiClient: func(repoHost string) (*api.Client, error) {
			return cmdtest.NewTestApiClient(t, &http.Client{Transport: reg}, "", "gitlab.com"), nil
		},
		baseRepo: func() (glrepo.Interface, error) {
			return glrepo.FromFullName("owner/repo", glinstance.DefaultHostname)
		},
		io: io,
	}

	vars := []gitlab.ProjectVariable{{Key: "VAR1", Value: "value1"}}
	client, _ := opts.apiClient("gitlab.com")

	err := opts.importProjectVariables(client.Lab(), "owner/repo", vars)
	assert.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created variable: VAR1")
}

func Test_importProjectVariables_UpdateExisting(t *testing.T) {
	reg := &httpmock.Mocker{MatchURL: httpmock.FullURL}
	defer reg.Verify(t)

	io, _, stdout, _ := cmdtest.TestIOStreams()

	reg.RegisterResponder(http.MethodGet, "https://gitlab.com/api/v4/projects/owner%2Frepo/variables/VAR1",
		httpmock.NewStringResponse(http.StatusOK, `{"key":"VAR1","value":"value1"}`))

	reg.RegisterResponder(http.MethodPut, "https://gitlab.com/api/v4/projects/owner%2Frepo/variables/VAR1",
		httpmock.NewStringResponse(http.StatusOK, `{"key":"VAR1","value":"value1"}`))

	opts := &options{
		update: true,
		apiClient: func(repoHost string) (*api.Client, error) {
			return cmdtest.NewTestApiClient(t, &http.Client{Transport: reg}, "", "gitlab.com"), nil
		},
		baseRepo: func() (glrepo.Interface, error) {
			return glrepo.FromFullName("owner/repo", glinstance.DefaultHostname)
		},
		io: io,
	}

	client, _ := opts.apiClient("gitlab.com")
	vars := []gitlab.ProjectVariable{{Key: "VAR1", Value: "value1"}}

	err := opts.importProjectVariables(client.Lab(), "owner/repo", vars)
	assert.NoError(t, err)
	assert.Contains(t, stdout.String(), "Updated variable: VAR1")
}

func Test_importGroupVariables_CreateAndUpdate(t *testing.T) {
	reg := &httpmock.Mocker{MatchURL: httpmock.FullURL}
	defer reg.Verify(t)

	io, _, stdout, _ := cmdtest.TestIOStreams()

	reg.RegisterResponder(http.MethodGet, "https://gitlab.com/api/v4/groups/group/variables/VAR1",
		httpmock.NewStringResponse(http.StatusNotFound, `{"code":"404","status":"Not Found"}`))

	reg.RegisterResponder(http.MethodPost, "https://gitlab.com/api/v4/groups/group/variables",
		httpmock.NewStringResponse(http.StatusCreated, `{"key":"VAR1","value":"value1"}`))

	opts := &options{
		group: "group",
		apiClient: func(repoHost string) (*api.Client, error) {
			return cmdtest.NewTestApiClient(t, &http.Client{Transport: reg}, "", "gitlab.com"), nil
		},
		io: io,
	}

	client, _ := opts.apiClient("gitlab.com")
	vars := []gitlab.ProjectVariable{{Key: "VAR1", Value: "value1"}}

	err := opts.importGroupVariables(client.Lab(), vars)
	assert.NoError(t, err)
	assert.Contains(t, stdout.String(), "Created variable: VAR1")
}

func Test_importGroupVariables_Existing_NoUpdate(t *testing.T) {
	reg := &httpmock.Mocker{MatchURL: httpmock.FullURL}
	defer reg.Verify(t)

	io, _, _, _ := cmdtest.TestIOStreams()

	reg.RegisterResponder(http.MethodGet,
		"https://gitlab.com/api/v4/groups/group/variables/VAR1",
		httpmock.NewStringResponse(http.StatusOK, `{"key": "VAR1", "value": "oldvalue"}`))

	opts := &options{
		group: "group",
		apiClient: func(repoHost string) (*api.Client, error) {
			return cmdtest.NewTestApiClient(t, &http.Client{Transport: reg}, "", "gitlab.com"), nil
		},
		io: io,
	}

	client, _ := opts.apiClient("gitlab.com")
	vars := []gitlab.ProjectVariable{{Key: "VAR1", Value: "value1"}}

	err := opts.importGroupVariables(client.Lab(), vars)
	assert.ErrorContains(t, err, "variable \"VAR1\" already exists")
}
