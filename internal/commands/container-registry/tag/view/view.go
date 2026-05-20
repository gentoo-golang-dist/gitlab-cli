package view

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/container_registry/registryutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	repositoryID int64
	tagName      string
	outputFormat string

	io           *iostreams.IOStreams
	gitlabClient func() (*gitlab.Client, error)
	baseRepo     func() (glrepo.Interface, error)
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:     "view <repository-id> <tag-name> [flags]",
		Short:   "View a container registry tag.",
		Long:    "View details for a single container registry tag.",
		Aliases: []string{"get"},
		Args:    cobra.ExactArgs(2),
		Example: heredoc.Doc(`
			# View a container registry tag
			glab container-registry tag view 123 latest`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(args); err != nil {
				return err
			}

			return opts.run()
		},
	}

	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)

	return cmd
}

func (o *options) complete(args []string) error {
	repositoryID, err := registryutils.ParseID(args[0], "repository ID")
	if err != nil {
		return &cmdutils.FlagError{Err: err}
	}
	o.repositoryID = repositoryID
	o.tagName = args[1]

	return nil
}

func (o *options) run() error {
	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	repo, err := o.baseRepo()
	if err != nil {
		return err
	}

	tag, _, err := client.ContainerRegistry.GetRegistryRepositoryTagDetail(
		repo.FullName(),
		o.repositoryID,
		o.tagName,
	)
	if err != nil {
		return cmdutils.WrapError(err, fmt.Sprintf("failed to fetch container registry tag %q.", o.tagName))
	}

	if o.outputFormat == "json" {
		return o.io.PrintJSON(tag)
	}

	fmt.Fprintln(o.io.StdOut, registryutils.DisplayTag(o.io, tag))
	return nil
}
