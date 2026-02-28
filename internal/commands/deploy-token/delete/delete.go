package delete

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	io           *iostreams.IOStreams
	gitlabClient func() (*gitlab.Client, error)
	baseRepo     func() (glrepo.Interface, error)

	tokenID int64
	group   string
}

func NewCmdDelete(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,
	}
	cmd := &cobra.Command{
		Use:     "delete <token-id>",
		Aliases: []string{"revoke", "remove"},
		Short:   "Delete a deploy token from a project or group.",
		Long:    ``,
		Example: heredoc.Doc(`
			# Delete project deploy token with ID as argument
			$ glab deploy-token delete 1234

			# Delete group deploy token
			$ glab deploy-token delete 1234 --group my-group
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

	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "Delete a group deploy token instead of a project deploy token.")

	return cmd
}

func (o *options) complete(_ *cobra.Command, args []string) error {
	if len(args) == 1 {
		tokenID, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("deploy token ID must be an integer: %s", args[0])
		}
		o.tokenID = int64(tokenID)
	}

	return nil
}

func (o *options) run() error {
	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	if o.group != "" {
		_, err = client.DeployTokens.DeleteGroupDeployToken(o.group, o.tokenID)
		if err != nil {
			return cmdutils.WrapError(err, "deleting group deploy token.")
		}
	} else {
		baseRepo, err := o.baseRepo()
		if err != nil {
			return err
		}

		_, err = client.DeployTokens.DeleteProjectDeployToken(baseRepo.FullName(), o.tokenID)
		if err != nil {
			return cmdutils.WrapError(err, "deleting project deploy token.")
		}
	}

	if o.io.IsOutputTTY() {
		cs := o.io.Color()
		o.io.LogInfof("%s Deploy token deleted.\n", cs.GreenCheck())
	} else {
		o.io.LogInfo("Deploy token deleted.")
	}

	return nil
}
