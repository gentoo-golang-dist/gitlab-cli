package list

import (
	"fmt"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/container_registry/registryutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/utils"
)

type options struct {
	group             string
	includeTags       bool
	includeTagDetails bool
	includeTagsCount  bool
	page              int
	perPage           int
	outputFormat      string

	io           *iostreams.IOStreams
	gitlabClient func() (*gitlab.Client, error)
	baseRepo     func() (glrepo.Interface, error)
}

type repositoryJSON struct {
	ID                     int64                           `json:"id"`
	Name                   string                          `json:"name"`
	Path                   string                          `json:"path"`
	ProjectID              int64                           `json:"project_id"`
	Location               string                          `json:"location"`
	CreatedAt              *time.Time                      `json:"created_at"`
	CleanupPolicyStartedAt *time.Time                      `json:"cleanup_policy_started_at"`
	Status                 *gitlab.ContainerRegistryStatus `json:"status"`
	TagsCount              *int64                          `json:"tags_count,omitempty"`
	Tags                   []tagJSON                       `json:"tags,omitempty"`
}

type tagJSON struct {
	Name          string     `json:"name"`
	Path          string     `json:"path"`
	Location      string     `json:"location"`
	Revision      string     `json:"revision,omitempty"`
	ShortRevision string     `json:"short_revision,omitempty"`
	Digest        string     `json:"digest,omitempty"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
	TotalSize     *int64     `json:"total_size,omitempty"`
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:     "list [flags]",
		Short:   "List container registry repositories.",
		Long:    "List container registry repositories for a project or group.",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Example: heredoc.Doc(`
			# List container registry repositories for the current project
			glab container-registry repository list

			# List container registry repositories for another project
			glab container-registry repository list -R gitlab-org/cli

			# List container registry repositories for a group
			glab container-registry repository list --group gitlab-org`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.run()
		},
	}

	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "List container registry repositories for a group.")
	cmd.Flags().BoolVar(&opts.includeTags, "include-tags", false, "Include tags in the response. Project repositories only.")
	cmd.Flags().BoolVar(&opts.includeTagDetails, "include-tag-details", false, "Fetch digest, size, and creation time for included tags. Project repositories only. Implies --include-tags.")
	cmd.Flags().BoolVar(&opts.includeTagsCount, "include-tags-count", true, "Include the number of tags in the response. Project repositories only.")
	cmd.Flags().IntVarP(&opts.page, "page", "p", 1, "Page number.")
	cmd.Flags().IntVarP(&opts.perPage, "per-page", "P", 30, "Number of items to list per page.")
	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)

	return cmd
}

func (o *options) run() error {
	if o.group != "" && o.includeTagDetails {
		return &cmdutils.FlagError{Err: fmt.Errorf("--include-tag-details is only available for project repositories")}
	}

	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	listOpts := gitlab.ListOptions{
		Page:    int64(o.page),
		PerPage: int64(o.perPage),
	}

	var repoName string
	var repositories []*gitlab.RegistryRepository
	var resp *gitlab.Response

	if o.group != "" {
		repoName = o.group
		repositories, resp, err = client.ContainerRegistry.ListGroupRegistryRepositories(
			o.group,
			&gitlab.ListGroupRegistryRepositoriesOptions{ListOptions: listOpts},
		)
	} else {
		repo, repoErr := o.baseRepo()
		if repoErr != nil {
			return repoErr
		}
		repoName = repo.FullName()
		opts := &gitlab.ListProjectRegistryRepositoriesOptions{
			ListOptions: listOpts,
		}
		if o.includeTags || o.includeTagDetails {
			opts.Tags = new(true)
		}
		if o.includeTagsCount {
			opts.TagsCount = new(true)
		}
		repositories, resp, err = client.ContainerRegistry.ListProjectRegistryRepositories(
			repo.FullName(),
			opts,
		)
	}
	if err != nil {
		return err
	}

	if o.outputFormat == "json" {
		showTagsCount := o.group == "" && o.includeTagsCount
		if o.includeTagDetails {
			if err := o.fetchRepositoryTagDetails(client, repoName, repositories); err != nil {
				return err
			}
		}

		return o.io.PrintJSON(newRepositoryJSONList(repositories, o.includeTagDetails, showTagsCount))
	}

	title := utils.NewListTitle("container registry repository")
	title.RepoName = repoName
	title.Page = o.page
	title.CurrentPageTotal = len(repositories)
	title.EmptyMessage = fmt.Sprintf("No container registry repositories available on %s.", repoName)
	if resp != nil {
		title.Total = int(resp.TotalItems)
	}

	fmt.Fprintf(o.io.StdOut, "%s\n", title.Describe())
	if len(repositories) > 0 {
		fmt.Fprintf(o.io.StdOut, "%s\n", registryutils.DisplayRepositories(o.io, repositories, o.group == "" && o.includeTagsCount))
	}

	return nil
}

func (o *options) fetchRepositoryTagDetails(client *gitlab.Client, repoName string, repositories []*gitlab.RegistryRepository) error {
	for _, repository := range repositories {
		detailedTags := make([]*gitlab.RegistryRepositoryTag, 0, len(repository.Tags))
		for _, tag := range repository.Tags {
			detailedTag, _, err := client.ContainerRegistry.GetRegistryRepositoryTagDetail(
				repoName,
				repository.ID,
				tag.Name,
			)
			if err != nil {
				return cmdutils.WrapError(err, fmt.Sprintf("failed to fetch container registry tag %q from repository %d.", tag.Name, repository.ID))
			}
			detailedTags = append(detailedTags, detailedTag)
		}
		repository.Tags = detailedTags
	}

	return nil
}

func newRepositoryJSONList(repositories []*gitlab.RegistryRepository, includeTagDetails bool, showTagsCount bool) []repositoryJSON {
	output := make([]repositoryJSON, 0, len(repositories))
	for _, repository := range repositories {
		var tagsCount *int64
		if showTagsCount {
			tagsCount = new(repository.TagsCount)
		}
		output = append(output, repositoryJSON{
			ID:                     repository.ID,
			Name:                   repository.Name,
			Path:                   repository.Path,
			ProjectID:              repository.ProjectID,
			Location:               repository.Location,
			CreatedAt:              repository.CreatedAt,
			CleanupPolicyStartedAt: repository.CleanupPolicyStartedAt,
			Status:                 repository.Status,
			TagsCount:              tagsCount,
			Tags:                   newTagJSONList(repository.Tags, includeTagDetails),
		})
	}

	return output
}

func newTagJSONList(tags []*gitlab.RegistryRepositoryTag, includeDetails bool) []tagJSON {
	if len(tags) == 0 {
		return nil
	}

	output := make([]tagJSON, 0, len(tags))
	for _, tag := range tags {
		tagOutput := tagJSON{
			Name:     tag.Name,
			Path:     tag.Path,
			Location: tag.Location,
		}
		if includeDetails {
			tagOutput.Revision = tag.Revision
			tagOutput.ShortRevision = tag.ShortRevision
			tagOutput.Digest = tag.Digest
			tagOutput.CreatedAt = tag.CreatedAt
			tagOutput.TotalSize = new(tag.TotalSize)
		}
		output = append(output, tagOutput)
	}

	return output
}
