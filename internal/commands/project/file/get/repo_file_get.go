package get

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

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
		Long: heredoc.Docf(`
		Print the contents of a single file from a GitLab repository at a specific
		commit, branch, or tag. No local clone required.

		The default output is raw bytes, suitable for piping or redirecting to a file.
		Use %[1]s--output json%[1]s for a structured response containing metadata such as
		%[1]sblob_id%[1]s, %[1]scommit_id%[1]s, and %[1]slast_commit_id%[1]s. The %[1]scontent%[1]s
		field in JSON mode is base64-encoded (matching the GitLab REST API).

		Pass a SHA to %[1]s--ref%[1]s (rather than a branch name) for results that are
		immutable and cacheable.
		`, "`"),
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
			opts.path = strings.TrimSpace(args[0])
			opts.ref = strings.TrimSpace(opts.ref)
			if err := opts.validate(); err != nil {
				return err
			}
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
	cmdutils.EnableRepoOverride(cmd, f)

	return cmd
}

func (o *options) validate() error {
	if o.path == "" {
		return errors.New("path argument cannot be empty")
	}
	if o.ref == "" {
		return errors.New(`flag "--ref" cannot be empty`)
	}
	if o.lfs && o.outputFormat == "json" {
		return errors.New("--lfs cannot be used with --output json: the JSON metadata endpoint does not resolve LFS pointers")
	}
	return nil
}

func (o *options) complete() error {
	repo, err := o.baseRepo()
	if err != nil {
		return fmt.Errorf("could not determine the target repository, pass --repo OWNER/REPO or run inside a Git repository: %w", err)
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
		return fmt.Errorf("failed to read %q at ref %q: %w", o.path, o.ref, err)
	}

	_, err = o.io.StdOut.Write(raw)
	return err
}

func (o *options) runJSON() error {
	file, _, err := o.client.RepositoryFiles.GetFile(o.repo.FullName(), o.path, &gitlab.GetFileOptions{Ref: &o.ref})
	if err != nil {
		return fmt.Errorf("failed to read %q at ref %q: %w", o.path, o.ref, err)
	}

	encoded, err := json.Marshal(file)
	if err != nil {
		return err
	}
	if _, err := o.io.StdOut.Write(encoded); err != nil {
		return err
	}
	_, err = io.WriteString(o.io.StdOut, "\n")
	return err
}
