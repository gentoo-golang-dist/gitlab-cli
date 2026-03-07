package delete

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	gitlabClient func() (*gitlab.Client, error)
	io           *iostreams.IOStreams
	baseRepo     func() (glrepo.Interface, error)

	tokenID int
	group   string
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,
	}
	cmd := &cobra.Command{
		Use:     "delete <token-id>",
		Short:   "Delete a deploy token.",
		Aliases: []string{"revoke", "rm", "remove"},
		Example: heredoc.Doc(`
		  $ glab deploy-token delete 42
		  $ glab deploy-token delete 42 -g mygroup
		`),
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid token ID %q: %w", args[0], err)
			}
			opts.tokenID = id
			return opts.run()
		},
	}

	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "Delete deploy token for a group.")

	return cmd
}

func (o *options) run() error {
	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	c := o.io.Color()

	if o.group != "" {
		_, err = client.DeployTokens.DeleteGroupDeployToken(o.group, int64(o.tokenID))
		if err != nil {
			return cmdutils.WrapError(err, "failed to delete group deploy token")
		}
		fmt.Fprintf(o.io.StdOut, "%s Deploy token %d deleted from group %s.\n", c.RedCheck(), o.tokenID, o.group)
	} else {
		baseRepo, repoErr := o.baseRepo()
		if repoErr != nil {
			return repoErr
		}
		_, err = client.DeployTokens.DeleteProjectDeployToken(baseRepo.FullName(), int64(o.tokenID))
		if err != nil {
			return cmdutils.WrapError(err, "failed to delete project deploy token")
		}
		fmt.Fprintf(o.io.StdOut, "%s Deploy token %d deleted from %s.\n", c.RedCheck(), o.tokenID, baseRepo.FullName())
	}

	return nil
}
