package delete

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	io        *iostreams.IOStreams
	apiClient func(repoHost string) (*api.Client, error)
	baseRepo  func() (glrepo.Interface, error)

	tokenID int
	group   string
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:     "delete <token-id>",
		Short:   "Delete a deploy token.",
		Aliases: []string{"revoke", "rm", "remove"},
		Example: heredoc.Doc(`
		  $ glab deploy-token delete 42
		  $ glab deploy-token delete 42 -g mygroup
		  $ glab deploy-token delete 42 -R owner/repo
		`),
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(cmd, args); err != nil {
				return err
			}
			return opts.run()
		},
	}

	cmdutils.EnableRepoOverride(cmd, f)

	fl := cmd.Flags()
	fl.StringVarP(&opts.group, "group", "g", "", "Delete deploy token for a group. Ignored if -R/--repo is set.")

	cmd.MarkFlagsMutuallyExclusive("group", "repo")

	return cmd
}

func (o *options) complete(cmd *cobra.Command, args []string) error {
	id, err := strconv.Atoi(args[0])
	if err != nil {
		return cmdutils.FlagError{Err: fmt.Errorf("invalid token ID %q: %w", args[0], err)}
	}
	o.tokenID = id

	group, err := cmdutils.GroupOverride(cmd)
	if err != nil {
		return err
	}
	o.group = group

	return nil
}

func (o *options) run() error {
	repo, repoErr := o.baseRepo()
	var repoHost string
	if repoErr == nil {
		repoHost = repo.RepoHost()
	}
	apiClient, err := o.apiClient(repoHost)
	if err != nil {
		return err
	}
	client := apiClient.Lab()

	c := o.io.Color()

	switch {
	case o.group != "":
		_, err = client.DeployTokens.DeleteGroupDeployToken(o.group, int64(o.tokenID))
		if err != nil {
			return cmdutils.WrapError(err, "failed to delete group deploy token")
		}
		fmt.Fprintf(o.io.StdOut, "%s Deploy token %d deleted from group %s.\n", c.RedCheck(), o.tokenID, o.group)
	default:
		if repoErr != nil {
			return repoErr
		}
		_, err = client.DeployTokens.DeleteProjectDeployToken(repo.FullName(), int64(o.tokenID))
		if err != nil {
			return cmdutils.WrapError(err, "failed to delete project deploy token")
		}
		fmt.Fprintf(o.io.StdOut, "%s Deploy token %d deleted from %s.\n", c.RedCheck(), o.tokenID, repo.FullName())
	}

	return nil
}
