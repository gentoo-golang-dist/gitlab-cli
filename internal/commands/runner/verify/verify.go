package verify

import (
	"context"

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
	io        *iostreams.IOStreams
	baseRepo  func() (glrepo.Interface, error)
	apiClient func(repoHost string) (*api.Client, error)
	token     string
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		baseRepo:  f.BaseRepo,
		apiClient: f.ApiClient,
	}

	cmd := &cobra.Command{
		Use:   "verify --token <authentication_token>",
		Short: "Verify authentication for a registered runner.",
		Long: heredoc.Doc(`
			Validates authentication credentials for a registered runner.
			Requires the runner's authentication token.
		`),
		Example: heredoc.Doc(`
			# Verify runner authentication using token from flag
			$ glab runner verify --token <authentication_token>

		`),
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return opts.run(cmd.Context())
		},
	}

	cmd.Flags().StringVarP(&opts.token, "token", "t", "", "The runner's authentication token.")
	_ = cmd.MarkFlagRequired("token")
	return cmd
}

func (o *options) run(ctx context.Context) error {
	var repoHost string
	if repo, err := o.baseRepo(); err == nil {
		repoHost = repo.RepoHost()
	}

	apiClient, err := o.apiClient(repoHost)
	if err != nil {
		return err
	}
	client := apiClient.Lab()

	_, err = client.Runners.VerifyRegisteredRunner(
		&gitlab.VerifyRegisteredRunnerOptions{Token: &o.token},
		gitlab.WithContext(ctx),
	)
	if err != nil {
		return cmdutils.WrapError(err, "runner authentication verification failed")
	}

	o.io.LogInfof("Runner authentication verified successfully.\n")
	return nil
}
