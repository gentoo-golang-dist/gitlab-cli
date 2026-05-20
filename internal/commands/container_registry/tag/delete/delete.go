package delete

import (
	"context"
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
	forceDelete  bool

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
		Use:   "delete <repository-id> <tag-name> [flags]",
		Short: "Delete a container registry tag.",
		Long: heredoc.Doc(`
			Delete a tag from a container registry repository.
		`),
		Aliases: []string{"del", "rm"},
		Args:    cobra.ExactArgs(2),
		Example: heredoc.Doc(`
			# Delete a container registry tag with a confirmation prompt
			glab container-registry tag delete 123 latest

			# Skip the confirmation prompt
			glab container-registry tag delete 123 latest --yes`),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(args); err != nil {
				return err
			}
			if err := opts.validate(); err != nil {
				return err
			}

			return opts.run(cmd.Context())
		},
	}

	cmd.Flags().BoolVarP(&opts.forceDelete, "yes", "y", false, "Skip the confirmation prompt.")

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

func (o *options) validate() error {
	if !o.forceDelete && !o.io.PromptEnabled() {
		return &cmdutils.FlagError{Err: fmt.Errorf("--yes or -y flag is required when not running interactively")}
	}

	return nil
}

func (o *options) run(ctx context.Context) error {
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
	var tagPath string
	if tag != nil {
		tagPath = tag.Path
	}
	if tagPath == "" {
		tagPath = fmt.Sprintf("%s:%s", repo.FullName(), o.tagName)
	}

	if !o.forceDelete && o.io.PromptEnabled() {
		fmt.Fprintf(o.io.StdErr, "This action will permanently delete container registry tag %q.\n\n", tagPath)
		err = o.io.Confirm(ctx, &o.forceDelete, fmt.Sprintf("Are you ABSOLUTELY SURE you wish to delete container registry tag %q?", tagPath))
		if err != nil {
			return cmdutils.WrapError(err, "could not prompt")
		}
	}

	if !o.forceDelete {
		return cmdutils.CancelError()
	}

	c := o.io.Color()
	o.io.LogInfof("%s Deleting container registry tag %s\n",
		c.ProgressIcon(),
		tagPath)

	_, err = client.ContainerRegistry.DeleteRegistryRepositoryTag(repo.FullName(), o.repositoryID, o.tagName)
	if err != nil {
		return cmdutils.WrapError(err, "failed to delete container registry tag.")
	}

	o.io.LogInfof(c.Bold("%s Container registry tag %q deleted.\n"), c.RedCheck(), o.tagName)
	return nil
}
