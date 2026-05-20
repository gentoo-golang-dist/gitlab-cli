package delete_tags

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
	repositoryID    int64
	nameRegexDelete string
	nameRegexKeep   string
	keepN           int
	olderThan       string
	forceDelete     bool

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
		Use:   "delete-tags <repository-id> [flags]",
		Short: "Delete container registry tags in bulk.",
		Long: heredoc.Doc(`
			Delete container registry tags in bulk based on tag name, age, and
			number of tags to keep.

			GitLab schedules matching tags for deletion asynchronously. The tags
			may remain visible until the background deletion job has completed.
		`),
		Args: cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			# Delete tags matching a regular expression
			glab container-registry tag delete-tags 123 --name-regex-delete '^release-.*' --yes

			# Delete old tags, but keep the 10 most recent tags
			glab container-registry tag delete-tags 123 --name-regex-delete '.*' --keep-n 10 --older-than 30d --yes`),
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

	cmd.Flags().StringVar(&opts.nameRegexDelete, "name-regex-delete", "", "Regular expression for tag names to delete.")
	cmd.Flags().StringVar(&opts.nameRegexKeep, "name-regex-keep", "", "Regular expression for tag names to keep.")
	cmd.Flags().IntVar(&opts.keepN, "keep-n", 0, "Keep the latest N matching tags.")
	cmd.Flags().StringVar(&opts.olderThan, "older-than", "", "Delete tags older than the given duration, such as 7d or 1month.")
	cmd.Flags().BoolVarP(&opts.forceDelete, "yes", "y", false, "Skip the confirmation prompt.")

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

func (o *options) validate() error {
	if o.nameRegexDelete == "" {
		return &cmdutils.FlagError{Err: fmt.Errorf("--name-regex-delete is required")}
	}
	if o.keepN < 0 {
		return &cmdutils.FlagError{Err: fmt.Errorf("--keep-n must be zero or a positive integer")}
	}
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

	if !o.forceDelete && o.io.PromptEnabled() {
		fmt.Fprintf(o.io.StdErr, "This action will permanently delete container registry tags from repository %d on %s.\n\n", o.repositoryID, repo.FullName())
		err = o.io.Confirm(ctx, &o.forceDelete, fmt.Sprintf("Are you ABSOLUTELY SURE you wish to delete container registry tags from repository %d?", o.repositoryID))
		if err != nil {
			return cmdutils.WrapError(err, "could not prompt")
		}
	}

	if !o.forceDelete {
		return cmdutils.CancelError()
	}

	deleteOpts := &gitlab.DeleteRegistryRepositoryTagsOptions{
		NameRegexpDelete: new(o.nameRegexDelete),
	}
	if o.nameRegexKeep != "" {
		deleteOpts.NameRegexpKeep = new(o.nameRegexKeep)
	}
	if o.keepN > 0 {
		keepN := int64(o.keepN)
		deleteOpts.KeepN = new(keepN)
	}
	if o.olderThan != "" {
		deleteOpts.OlderThan = new(o.olderThan)
	}

	c := o.io.Color()
	o.io.LogInfof("%s Scheduling container registry tags for deletion %s=%s %s=%d\n",
		c.ProgressIcon(),
		c.Blue("repo"), repo.FullName(),
		c.Blue("repository"), o.repositoryID)

	_, err = client.ContainerRegistry.DeleteRegistryRepositoryTags(repo.FullName(), o.repositoryID, deleteOpts)
	if err != nil {
		return cmdutils.WrapError(err, "failed to delete container registry tags.")
	}

	o.io.LogInfof(c.Bold("%s Container registry tags scheduled for deletion. They may remain visible until GitLab finishes the background deletion job.\n"), c.RedCheck())
	return nil
}
