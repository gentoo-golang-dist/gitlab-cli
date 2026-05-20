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
	repositoryID     int64
	includeTags      bool
	includeTagsCount bool
	outputFormat     string

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
		Use:     "view <repository-id> [flags]",
		Short:   "View a container registry repository.",
		Long:    "View details for a single container registry repository.",
		Aliases: []string{"get"},
		Args:    cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			# View a container registry repository
			glab container-registry repository view 123

			# Include tag details
			glab container-registry repository view 123 --include-tags`),
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

	cmd.Flags().BoolVar(&opts.includeTags, "include-tags", false, "Include tags in the response.")
	cmd.Flags().BoolVar(&opts.includeTagsCount, "include-tags-count", true, "Include the number of tags in the response.")
	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)

	return cmd
}

func (o *options) complete(args []string) error {
	repositoryID, err := registryutils.ParseID(args[0], "repository ID")
	if err != nil {
		return &cmdutils.FlagError{Err: err}
	}
	o.repositoryID = repositoryID

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

	opts := &gitlab.GetSingleRegistryRepositoryOptions{}
	if o.includeTags {
		opts.Tags = new(true)
	}
	if o.includeTagsCount {
		opts.TagsCount = new(true)
	}

	repository, _, err := client.ContainerRegistry.GetSingleRegistryRepository(o.repositoryID, opts)
	if err != nil {
		return cmdutils.WrapError(err, fmt.Sprintf("failed to fetch container registry repository %d on %s.", o.repositoryID, repo.FullName()))
	}

	if o.outputFormat == "json" {
		return o.io.PrintJSON(repository)
	}

	fmt.Fprintln(o.io.StdOut, registryutils.DisplayRepository(o.io, repository))
	return nil
}
