package get

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

	tokenID      int
	group        string
	outputFormat string
}

func NewCmdGet(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,
	}
	cmd := &cobra.Command{
		Use:   "get <token-id>",
		Short: "Get a deploy token by ID.",
		Example: heredoc.Doc(`
		  $ glab deploy-token get 42
		  $ glab deploy-token get 42 -g mygroup
		`),
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
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

	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "Get deploy token for a group.")
	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)

	return cmd
}

func (o *options) run() error {
	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	var token *gitlab.DeployToken

	if o.group != "" {
		token, _, err = client.DeployTokens.GetGroupDeployToken(o.group, int64(o.tokenID))
	} else {
		baseRepo, repoErr := o.baseRepo()
		if repoErr != nil {
			return repoErr
		}
		token, _, err = client.DeployTokens.GetProjectDeployToken(baseRepo.FullName(), int64(o.tokenID))
	}
	if err != nil {
		return cmdutils.WrapError(err, "failed to get deploy token")
	}

	if o.outputFormat == "json" {
		return o.io.PrintJSON(token)
	}

	cs := o.io.Color()
	fmt.Fprintf(o.io.StdOut, "ID:\t\t%d\n", token.ID)
	fmt.Fprintf(o.io.StdOut, "Name:\t\t%s\n", token.Name)
	fmt.Fprintf(o.io.StdOut, "Username:\t%s\n", token.Username)
	fmt.Fprintf(o.io.StdOut, "Scopes:\t\t%v\n", token.Scopes)
	if token.ExpiresAt != nil {
		fmt.Fprintf(o.io.StdOut, "Expires At:\t%s\n", token.ExpiresAt.String())
	} else {
		fmt.Fprintf(o.io.StdOut, "Expires At:\tNever\n")
	}
	if token.Revoked {
		fmt.Fprintf(o.io.StdOut, "Status:\t\t%s\n", cs.Yellow("Revoked"))
	} else if token.Expired {
		fmt.Fprintf(o.io.StdOut, "Status:\t\t%s\n", cs.Red("Expired"))
	} else {
		fmt.Fprintf(o.io.StdOut, "Status:\t\t%s\n", cs.Green("Active"))
	}

	return nil
}
