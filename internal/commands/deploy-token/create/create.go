package create

import (
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	gitlabClient func() (*gitlab.Client, error)
	io           *iostreams.IOStreams
	baseRepo     func() (glrepo.Interface, error)

	name         string
	username     string
	expiresAt    string
	scopes       []string
	group        string
	outputFormat string
}

var validScopes = []string{
	"read_repository",
	"read_registry",
	"write_registry",
	"read_package_registry",
	"write_package_registry",
	"read_virtual_registry",
	"write_virtual_registry",
}

func NewCmdCreate(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,
	}
	cmd := &cobra.Command{
		Use:     "create <name>",
		Aliases: []string{"new"},
		Args:    cobra.ExactArgs(1),
		Short:   "Create a deploy token for a project or group.",
		Long: heredoc.Doc(`
		Creates a new deploy token for a project or group.

		The scopes must be at least one of: read_repository, read_registry, write_registry,
		read_package_registry, write_package_registry, read_virtual_registry, or write_virtual_registry.

		The token value is shown only once upon creation and cannot be retrieved later.
		`),
		Example: heredoc.Doc(`
		# Create a project deploy token with read_repository scope
		$ glab deploy-token create my-token --scope read_repository

		# Create a project deploy token with multiple scopes and expiration
		$ glab deploy-token create my-token --scope read_repository --scope read_registry --expires-at 2025-12-31

		# Create a project deploy token with a custom username
		$ glab deploy-token create my-token --scope read_repository --username my-custom-user

		# Create a group deploy token
		$ glab deploy-token create my-token --group my-group --scope read_repository

		# Create a deploy token and output as JSON
		$ glab deploy-token create my-token --scope read_repository --output json
		`),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(cmd, args); err != nil {
				return err
			}

			if err := opts.validate(); err != nil {
				return err
			}

			return opts.run()
		},
	}

	cmd.Flags().StringVarP(&opts.username, "username", "u", "", "Username for the deploy token. Default is gitlab+deploy-token-{n}.")
	cmd.Flags().StringVarP(&opts.expiresAt, "expires-at", "e", "", "Expiration date for the deploy token in ISO 8601 format (YYYY-MM-DD or YYYY-MM-DDTHH:MM:SSZ). Does not expire if not provided.")
	cmd.Flags().StringSliceVarP(&opts.scopes, "scope", "s", []string{}, "Scopes for the deploy token. Can be specified multiple times. At least one scope is required.")
	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "Create a group deploy token instead of a project deploy token.")
	cmd.Flags().StringVarP(&opts.outputFormat, "output", "F", "text", "Format output as 'text' or 'json'.")

	_ = cmd.MarkFlagRequired("scope")

	return cmd
}

func (o *options) complete(_ *cobra.Command, args []string) error {
	o.name = args[0]
	return nil
}

func (o *options) validate() error {
	if len(o.scopes) == 0 {
		return &cmdutils.FlagError{Err: errors.New("at least one scope is required")}
	}

	// Validate scopes
	for _, scope := range o.scopes {
		if !slices.Contains(validScopes, scope) {
			return &cmdutils.FlagError{Err: errors.New("invalid scope: " + scope + ". Valid scopes are: read_repository, read_registry, write_registry, read_package_registry, write_package_registry, read_virtual_registry, write_virtual_registry")}
		}
	}

	// Validate expires-at format if provided
	if o.expiresAt != "" {
		// Try parsing as date only first
		if _, err := time.Parse("2006-01-02", o.expiresAt); err != nil {
			// Try parsing as full datetime
			if _, err := time.Parse(time.RFC3339, o.expiresAt); err != nil {
				return &cmdutils.FlagError{Err: errors.New("invalid expires-at format. Use YYYY-MM-DD or YYYY-MM-DDTHH:MM:SSZ")}
			}
		}
	}

	return nil
}

func (o *options) run() error {
	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	var token *gitlab.DeployToken

	if o.group != "" {
		createOptions := &gitlab.CreateGroupDeployTokenOptions{
			Name:   &o.name,
			Scopes: &o.scopes,
		}

		if o.username != "" {
			createOptions.Username = &o.username
		}

		if o.expiresAt != "" {
			expiresAt, _ := parseExpiresAt(o.expiresAt)
			createOptions.ExpiresAt = expiresAt
		}

		token, _, err = client.DeployTokens.CreateGroupDeployToken(o.group, createOptions)
		if err != nil {
			return cmdutils.WrapError(err, "failed to create group deploy token.")
		}
	} else {
		baseRepo, err := o.baseRepo()
		if err != nil {
			return err
		}

		createOptions := &gitlab.CreateProjectDeployTokenOptions{
			Name:   &o.name,
			Scopes: &o.scopes,
		}

		if o.username != "" {
			createOptions.Username = &o.username
		}

		if o.expiresAt != "" {
			expiresAt, _ := parseExpiresAt(o.expiresAt)
			createOptions.ExpiresAt = expiresAt
		}

		token, _, err = client.DeployTokens.CreateProjectDeployToken(baseRepo.FullName(), createOptions)
		if err != nil {
			return cmdutils.WrapError(err, "failed to create project deploy token.")
		}
	}

	if o.outputFormat == "json" {
		jsonOutput, err := json.Marshal(token)
		if err != nil {
			return cmdutils.WrapError(err, "failed to marshal token to JSON.")
		}
		o.io.LogInfo(string(jsonOutput))
	} else {
		if o.io.IsOutputTTY() {
			cs := o.io.Color()
			o.io.LogInfof("%s Deploy token created successfully.\n", cs.GreenCheck())
			o.io.LogInfof("Token: %s\n", cs.Bold(token.Token))
			o.io.LogInfof("\n%s Make sure to copy the token now. You won't be able to see it again!\n", cs.Yellow("!"))
		} else {
			o.io.LogInfo(token.Token)
		}
	}

	return nil
}

func parseExpiresAt(expiresAt string) (*time.Time, error) {
	// Try parsing as date only first
	if t, err := time.Parse("2006-01-02", expiresAt); err == nil {
		return &t, nil
	}

	// Try parsing as full datetime
	if t, err := time.Parse(time.RFC3339, expiresAt); err == nil {
		return &t, nil
	}

	return nil, errors.New("invalid expires-at format")
}
