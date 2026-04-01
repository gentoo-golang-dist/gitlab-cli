package create

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
	apiClient func(repoHost string) (*api.Client, error)
	baseRepo  func() (glrepo.Interface, error)

	runnerType      string
	groupID         int64
	projectID       int64
	description     string
	tagList         []string
	accessLevel     string
	maintenanceNote string

	paused         bool
	locked         bool
	runUntagged    bool
	maximumTimeout int64

	outputFormat string
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:   "create [flags]",
		Short: "Create a CI/CD runner linked to your user.",
		Long: heredoc.Doc(`
			Creates a new runner linked to your user. The new runner is
			tied to your user for ownership and audit.
		`),
		Args: cobra.NoArgs,
		Example: heredoc.Doc(`
			# Instance runner (admin)
			glab runner create --runner-type instance_type --description "shared runner"

			# Group runner
			glab runner create --runner-type group_type --group-id 123 --description "group ci"

			# Project runner with tags and JSON output
			glab runner create --runner-type project_type --project-id 456 --tag docker --tag linux --output json`),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.validate(); err != nil {
				return err
			}
			return opts.run(cmd.Context(), cmd)
		},
	}

	cmdutils.EnableRepoOverride(cmd, f)
	cmdutils.EnableJSONOutput(cmd, &opts.outputFormat)

	fl := cmd.Flags()
	fl.StringVar(&opts.runnerType, "runner-type", "", `Runner scope: "instance_type", "group_type", or "project_type".`)
	fl.Int64Var(&opts.groupID, "group-id", 0, "Group ID (required when runner-type is group_type).")
	fl.Int64Var(&opts.projectID, "project-id", 0, "Project ID (required when runner-type is project_type).")
	fl.StringVarP(&opts.description, "description", "d", "", "Description of the runner.")
	fl.BoolVarP(&opts.paused, "paused", "p", false, "When set, the runner ignores new jobs.")
	fl.BoolVarP(&opts.locked, "locked", "l", false, "When set, the runner is locked to the current project.")
	fl.BoolVarP(&opts.runUntagged, "run-untagged", "u", false, "When set, the runner processes jobs without tags.")
	fl.StringSliceVar(&opts.tagList, "tag", nil, "Runner tags. For multiple tags, repeat the flag or use a comma-separated list.")
	fl.StringVar(&opts.accessLevel, "access-level", "", `Access level: "not_protected" or "ref_protected".`)
	fl.StringVar(&opts.maintenanceNote, "maintenance-note", "", "Maintenance note (up to 1024 characters).")
	fl.Int64Var(&opts.maximumTimeout, "maximum-timeout", 0, "Maximum job duration in seconds for this runner.")

	_ = cmd.MarkFlagRequired("runner-type")

	return cmd
}

func (o *options) validate() error {
	rt := strings.TrimSpace(o.runnerType)
	if rt == "" {
		return cmdutils.FlagError{Err: errors.New("the required flag '--runner-type' must not be empty")}
	}
	switch rt {
	case "instance_type", "group_type", "project_type":
	default:
		return cmdutils.FlagError{Err: fmt.Errorf("invalid --runner-type %q: must be instance_type, group_type, or project_type", o.runnerType)}
	}
	switch rt {
	case "group_type":
		if o.groupID == 0 {
			return cmdutils.FlagError{Err: errors.New("runner-type group_type requires --group-id")}
		}
	case "project_type":
		if o.projectID == 0 {
			return cmdutils.FlagError{Err: errors.New("runner-type project_type requires --project-id")}
		}
	case "instance_type":
		if o.groupID != 0 || o.projectID != 0 {
			return cmdutils.FlagError{Err: errors.New("runner-type instance_type cannot be used with --group-id or --project-id")}
		}
	}
	return nil
}

func (o *options) run(ctx context.Context, cmd *cobra.Command) error {
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

	createOpts := o.buildCreateUserRunnerOptions(cmd)
	runner, _, err := client.Users.CreateUserRunner(createOpts, gitlab.WithContext(ctx))
	if err != nil {
		return cmdutils.WrapError(err, "failed to create runner")
	}

	switch o.outputFormat {
	case "json":
		return o.io.PrintJSON(runner)
	default:
		o.io.LogInfof("Created runner %d\n", runner.ID)
		if runner.Token != "" {
			o.io.LogInfof("Authentication token: %s\n", runner.Token)
		}
		if runner.TokenExpiresAt != nil {
			o.io.LogInfof("Token expires at: %s\n", runner.TokenExpiresAt.Format("2006-01-02T15:04:05Z07:00"))
		}
		return nil
	}
}

func (o *options) buildCreateUserRunnerOptions(cmd *cobra.Command) *gitlab.CreateUserRunnerOptions {
	rt := o.runnerType
	opt := &gitlab.CreateUserRunnerOptions{
		RunnerType: &rt,
	}
	if o.groupID != 0 {
		gid := o.groupID
		opt.GroupID = &gid
	}
	if o.projectID != 0 {
		pid := o.projectID
		opt.ProjectID = &pid
	}
	if o.description != "" {
		opt.Description = &o.description
	}
	if f := cmd.Flags().Lookup("paused"); f != nil && f.Changed {
		v := o.paused
		opt.Paused = &v
	}
	if f := cmd.Flags().Lookup("locked"); f != nil && f.Changed {
		v := o.locked
		opt.Locked = &v
	}
	if f := cmd.Flags().Lookup("run-untagged"); f != nil && f.Changed {
		v := o.runUntagged
		opt.RunUntagged = &v
	}
	if len(o.tagList) > 0 {
		tags := o.tagList
		opt.TagList = &tags
	}
	if o.accessLevel != "" {
		opt.AccessLevel = &o.accessLevel
	}
	if o.maintenanceNote != "" {
		opt.MaintenanceNote = &o.maintenanceNote
	}
	if f := cmd.Flags().Lookup("maximum-timeout"); f != nil && f.Changed {
		v := o.maximumTimeout
		opt.MaximumTimeout = &v
	}

	return opt
}
