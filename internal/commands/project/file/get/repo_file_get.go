package get

import (
	"encoding/json"
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	path         string
	ref          string
	lfs          bool
	outputFormat string

	io        *iostreams.IOStreams
	repo      glrepo.Interface
	client    *gitlab.Client
	apiClient func(repoHost string) (*api.Client, error)
	baseRepo  func() (glrepo.Interface, error)
}

func NewCmdFileGet(f cmdutils.Factory) *cobra.Command {
	opts := options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "get <path> --ref <sha|branch|tag> [flags]",
		Short: "Read a file from a repository at a specific ref.",
		Long: heredoc.Doc(`
		Print the contents of a single file from a GitLab repository at a specific
		commit, branch, or tag. No local clone required.

		The default output is raw bytes, suitable for piping or redirecting to a file.
		Use '--output json' for a structured response containing metadata such as
		'blob_id', 'commit_id', and 'last_commit_id'.

		Pass a SHA to '--ref' (rather than a branch name) for results that are
		immutable and cacheable.
		`),
		Example: heredoc.Doc(`
		# Read README.md at the tip of the main branch
		glab repo file get README.md --ref main

		# Read a file at a specific commit, structured output
		glab repo file get docs/index.md --ref 1a2b3c4d --output json

		# Read a file from a different repository
		glab repo file get internal/main.go --repo gitlab-org/cli --ref main

		# Resolve an LFS pointer to the underlying binary and save it to disk
		glab repo file get assets/big.bin --ref main --lfs > big.bin
		`),
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.path = args[0]
			if err := opts.complete(); err != nil {
				return err
			}
			return opts.run()
		},
	}

	cmd.Flags().StringVarP(&opts.ref, "ref", "r", "", "Branch, tag, or commit SHA to read from. Required.")
	_ = cmd.MarkFlagRequired("ref")
	cmd.Flags().BoolVar(&opts.lfs, "lfs", false, "If the path is an LFS pointer, resolve to the underlying binary. Only applies with --output text.")
	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)

	return cmd
}

func (o *options) complete() error {
	repo, err := o.baseRepo()
	if err != nil {
		return cmdutils.WrapError(err, "Could not determine the target repository. Pass --repo OWNER/REPO or run inside a Git repository.")
	}
	o.repo = repo

	apiClient, err := o.apiClient(o.repo.RepoHost())
	if err != nil {
		return err
	}
	o.client = apiClient.Lab()
	return nil
}

func (o *options) run() error {
	if o.outputFormat == "json" {
		return o.runJSON()
	}
	return o.runRaw()
}

func (o *options) runRaw() error {
	rawOpts := &gitlab.GetRawFileOptions{Ref: &o.ref}
	if o.lfs {
		rawOpts.LFS = &o.lfs
	}

	raw, _, err := o.client.RepositoryFiles.GetRawFile(o.repo.FullName(), o.path, rawOpts)
	if err != nil {
		return cmdutils.WrapError(err, fmt.Sprintf("Failed to read %q at ref %q.", o.path, o.ref))
	}

	_, err = o.io.StdOut.Write(raw)
	return err
}

func (o *options) runJSON() error {
	file, _, err := o.client.RepositoryFiles.GetFile(o.repo.FullName(), o.path, &gitlab.GetFileOptions{Ref: &o.ref})
	if err != nil {
		return cmdutils.WrapError(err, fmt.Sprintf("Failed to read %q at ref %q.", o.path, o.ref))
	}

	encoded, err := json.Marshal(file)
	if err != nil {
		return err
	}
	fmt.Fprintln(o.io.StdOut, string(encoded))
	return nil
}
