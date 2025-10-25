package importCmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type options struct {
	apiClient func(repoHost string) (*api.Client, error)
	io        *iostreams.IOStreams
	baseRepo  func() (glrepo.Interface, error)

	group     string
	filePath  string
	fromStdin bool
	update    bool // true => update existing vars, false => error if exists

	variables []gitlab.ProjectVariable
}

func NewCmdImport(f cmdutils.Factory, runE func(opts *options) error) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:     "import",
		Short:   "Import variables from JSON or STDIN into a project or group.",
		Aliases: []string{"im"},
		Example: heredoc.Doc(`
			# Example JSON file format (variables.json)
			[
				{
					"key": "DATABASE_URL",
					"value": "postgres://user:password@host/db",
					"protected": true,
					"masked": false,
					"environment_scope": "*",
					"variable_type": "env_var",
					"description": "Database connection string"
				},
				{
					"key": "API_KEY",
					"value": "secret_key_here",
					"masked": true,
					"masked_and_hidden": true,
					"protected": false,
					"environment_scope": "production",
					"variable_type": "env_var",
					"description": "API key for production services"
				}
			]

			# Import variables from a JSON file into the current project
			$ glab variable import --file variables.json

			# Import and update existing variables if they already exist
			$ glab variable import --file vars.json --update

			# Import variables from standard input
			$ cat variables.json | glab variable import --stdin

			# Import variables into a specific group or subgroup
			$ glab variable import --group mygroup --file group_vars.json
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(); err != nil {
				return err
			}
			return opts.run()
		},
	}

	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "Select a group or subgroup. Ignored if a repository argument is set.")
	cmd.Flags().StringVarP(&opts.filePath, "file", "f", "", "Path to JSON file containing variables.")
	cmd.Flags().BoolVar(&opts.fromStdin, "stdin", false, "Read JSON from standard input.")
	cmd.Flags().BoolVar(&opts.update, "update", false, "Update existing variables instead of throwing an error.")

	cmd.MarkFlagsMutuallyExclusive("file", "stdin")

	return cmd
}

func (o *options) complete() error {
	var input []byte
	var err error

	switch {
	case o.filePath != "":
		input, err = os.ReadFile(o.filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

	case o.fromStdin:
		input, err = io.ReadAll(o.io.In)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		if len(input) == 0 {
			return fmt.Errorf("failed to read from stdin: no data")
		}

	default:
		return fmt.Errorf("no input source provided: use --file or --stdin")
	}

	if err := json.Unmarshal(input, &o.variables); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	return nil
}

func (o *options) run() error {
	var err error

	var repoHost string
	if baseRepo, err := o.baseRepo(); err == nil {
		repoHost = baseRepo.RepoHost()
	}
	apiClient, err := o.apiClient(repoHost)
	if err != nil {
		return err
	}
	client := apiClient.Lab()

	switch {
	case o.group != "":
		return o.importGroupVariables(client, o.variables)

	default:
		repo, err := o.baseRepo()
		if err != nil {
			return err
		}
		return o.importProjectVariables(client, repo.FullName(), o.variables)
	}
}

func (o *options) importProjectVariables(client *gitlab.Client, project string, vars []gitlab.ProjectVariable) error {
	for _, v := range vars {
		_, resp, err := client.ProjectVariables.GetVariable(project, v.Key, nil)
		if err == nil && resp.StatusCode == http.StatusOK {
			if !o.update {
				return fmt.Errorf("variable %q already exists. use --update if you wish to override it", v.Key)
			}
			_, _, err := client.ProjectVariables.UpdateVariable(project, v.Key, &gitlab.UpdateProjectVariableOptions{
				Value:            gitlab.Ptr(v.Value),
				Description:      gitlab.Ptr(v.Description),
				EnvironmentScope: gitlab.Ptr(v.EnvironmentScope),
				Masked:           gitlab.Ptr(v.Masked),
				Protected:        gitlab.Ptr(v.Protected),
				Raw:              gitlab.Ptr(v.Raw),
				VariableType:     gitlab.Ptr(v.VariableType),
			})
			if err != nil {
				return fmt.Errorf("failed to update variable %s: %w", v.Key, err)
			}
			fmt.Fprintf(o.io.StdOut, "Updated variable: %s\n", v.Key)
		} else {
			_, _, err := client.ProjectVariables.CreateVariable(project, &gitlab.CreateProjectVariableOptions{
				Key:              gitlab.Ptr(v.Key),
				Value:            gitlab.Ptr(v.Value),
				Description:      gitlab.Ptr(v.Description),
				EnvironmentScope: gitlab.Ptr(v.EnvironmentScope),
				Masked:           gitlab.Ptr(v.Masked),
				MaskedAndHidden:  gitlab.Ptr(v.Hidden),
				Protected:        gitlab.Ptr(v.Protected),
				Raw:              gitlab.Ptr(v.Raw),
				VariableType:     gitlab.Ptr(v.VariableType),
			})
			if err != nil {
				return fmt.Errorf("failed to create variable %s: %w", v.Key, err)
			}
			fmt.Fprintf(o.io.StdOut, "Created variable: %s\n", v.Key)
		}
	}
	return nil
}

func (o *options) importGroupVariables(client *gitlab.Client, vars []gitlab.ProjectVariable) error {
	for _, v := range vars {
		_, resp, err := client.GroupVariables.GetVariable(o.group, v.Key, nil)
		if err == nil && resp.StatusCode == http.StatusOK {
			if !o.update {
				return fmt.Errorf("variable %q already exists", v.Key)
			}
			_, _, err := client.GroupVariables.UpdateVariable(o.group, v.Key, &gitlab.UpdateGroupVariableOptions{
				Value:            gitlab.Ptr(v.Value),
				Description:      gitlab.Ptr(v.Description),
				EnvironmentScope: gitlab.Ptr(v.EnvironmentScope),
				Masked:           gitlab.Ptr(v.Masked),
				Protected:        gitlab.Ptr(v.Protected),
				Raw:              gitlab.Ptr(v.Raw),
				VariableType:     gitlab.Ptr(v.VariableType),
			})
			if err != nil {
				return fmt.Errorf("failed to update variable %s: %w", v.Key, err)
			}
			fmt.Fprintf(o.io.StdOut, "Updated variable: %s\n", v.Key)
		} else {
			_, _, err := client.GroupVariables.CreateVariable(o.group, &gitlab.CreateGroupVariableOptions{
				Key:              gitlab.Ptr(v.Key),
				Value:            gitlab.Ptr(v.Value),
				Description:      gitlab.Ptr(v.Description),
				EnvironmentScope: gitlab.Ptr(v.EnvironmentScope),
				Masked:           gitlab.Ptr(v.Masked),
				MaskedAndHidden:  gitlab.Ptr(v.Hidden),
				Protected:        gitlab.Ptr(v.Protected),
				VariableType:     gitlab.Ptr(v.VariableType),
				Raw:              gitlab.Ptr(v.Raw),
			})
			if err != nil {
				return fmt.Errorf("failed to create variable %s: %w", v.Key, err)
			}
			fmt.Fprintf(o.io.StdOut, "Created variable: %s\n", v.Key)
		}
	}
	return nil
}
