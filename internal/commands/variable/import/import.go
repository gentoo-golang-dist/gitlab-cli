package importCmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	clientgo "gitlab.com/gitlab-org/api/client-go"
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
}

func NewCmdImport(f cmdutils.Factory, runE func(opts *options) error) *cobra.Command {
	opts := &options{
		io:        f.IO(),
		apiClient: f.ApiClient,
		baseRepo:  f.BaseRepo,
	}

	cmd := &cobra.Command{
		Use:     "import",
		Short:   "Import variables from JSON into a project or group.",
		Aliases: []string{"im"},
		Example: heredoc.Doc(`
			$ glab variable import --file variables.json
			$ glab variable import --file vars.json --update
			$ cat variables.json | glab variable import --stdin
			$ glab variable import --group mygroup --file group_vars.json
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if runE != nil {
				return runE(opts)
			}
			return opts.run()
		},
	}

	cmd.Flags().StringVarP(&opts.group, "group", "g", "", "Select a group or subgroup. Ignored if a repository argument is set.")
	cmd.Flags().StringVarP(&opts.filePath, "file", "f", "", "Path to JSON file containing variables.")
	cmd.Flags().BoolVar(&opts.fromStdin, "stdin", false, "Read JSON from standard input.")
	cmd.Flags().BoolVar(&opts.update, "update", false, "Update existing variables instead of throwing an error.")

	return cmd
}

func (o *options) run() error {
	var input []byte
	var err error

	if o.filePath != "" {
		input, err = os.ReadFile(o.filePath)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
	} else if o.fromStdin {
		input, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
		if len(input) == 0 {
			return fmt.Errorf("failed to read from stdin: no data")
		}
	} else {
		return fmt.Errorf("no input source provided: use --file or --stdin")
	}

	var variables []clientgo.ProjectVariable
	if err := json.Unmarshal(input, &variables); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	var repoHost string
	if baseRepo, err := o.baseRepo(); err == nil {
		repoHost = baseRepo.RepoHost()
	}
	apiClient, err := o.apiClient(repoHost)
	if err != nil {
		return err
	}
	client := apiClient.Lab()

	if o.group != "" {
		return o.importGroupVariables(client, variables)
	}

	repo, err := o.baseRepo()
	if err != nil {
		return err
	}
	return o.importProjectVariables(client, repo.FullName(), variables)
}

func (o *options) importProjectVariables(client *clientgo.Client, project string, vars []clientgo.ProjectVariable) error {
	for _, v := range vars {
		_, resp, err := client.ProjectVariables.GetVariable(project, v.Key, nil)
		if err == nil && resp.StatusCode == 200 {
			if !o.update {
				return fmt.Errorf("variable %q already exists. use --update if you wish to override it", v.Key)
			}
			_, _, err := client.ProjectVariables.UpdateVariable(project, v.Key, &clientgo.UpdateProjectVariableOptions{
				Value:            &v.Value,
				Protected:        &v.Protected,
				Masked:           &v.Masked,
				EnvironmentScope: &v.EnvironmentScope,
				VariableType:     &v.VariableType,
				Description:      &v.Description,
				Raw:              &v.Raw,
			})
			if err != nil {
				return fmt.Errorf("failed to update variable %s: %w", v.Key, err)
			}
			fmt.Fprintf(o.io.StdOut, "Updated variable: %s\n", v.Key)
		} else {
			_, _, err := client.ProjectVariables.CreateVariable(project, &clientgo.CreateProjectVariableOptions{
				Key:              &v.Key,
				Value:            &v.Value,
				Protected:        &v.Protected,
				Masked:           &v.Masked,
				EnvironmentScope: &v.EnvironmentScope,
				VariableType:     &v.VariableType,
				Description:      &v.Description,
				Raw:              &v.Raw,
			})
			if err != nil {
				return fmt.Errorf("failed to create variable %s: %w", v.Key, err)
			}
			fmt.Fprintf(o.io.StdOut, "Created variable: %s\n", v.Key)
		}
	}
	return nil
}

func (o *options) importGroupVariables(client *clientgo.Client, vars []clientgo.ProjectVariable) error {
	for _, v := range vars {
		_, resp, err := client.GroupVariables.GetVariable(o.group, v.Key, nil)
		if err == nil && resp.StatusCode == 200 {
			if !o.update {
				return fmt.Errorf("variable %q already exists", v.Key)
			}
			_, _, err := client.GroupVariables.UpdateVariable(o.group, v.Key, &clientgo.UpdateGroupVariableOptions{
				Value:            &v.Value,
				Protected:        &v.Protected,
				Masked:           &v.Masked,
				EnvironmentScope: &v.EnvironmentScope,
				VariableType:     &v.VariableType,
				Description:      &v.Description,
				Raw:              &v.Raw,
			})
			if err != nil {
				return fmt.Errorf("failed to update variable %s: %w", v.Key, err)
			}
			fmt.Fprintf(o.io.StdOut, "Updated variable: %s\n", v.Key)
		} else {
			_, _, err := client.GroupVariables.CreateVariable(o.group, &clientgo.CreateGroupVariableOptions{
				Key:              &v.Key,
				Value:            &v.Value,
				Protected:        &v.Protected,
				Masked:           &v.Masked,
				EnvironmentScope: &v.EnvironmentScope,
				VariableType:     &v.VariableType,
				Description:      &v.Description,
				Raw:              &v.Raw,
			})
			if err != nil {
				return fmt.Errorf("failed to create variable %s: %w", v.Key, err)
			}
			fmt.Fprintf(o.io.StdOut, "Created variable: %s\n", v.Key)
		}
	}
	return nil
}
