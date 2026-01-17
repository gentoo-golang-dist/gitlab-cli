package trigger

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/ci/ciutils"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

func parseVarArg(s string) (*gitlab.JobVariableOptions, error) {
	// From https://pkg.go.dev/strings#Split:
	//
	// > If s does not contain sep and sep is not empty,
	// > Split returns a slice of length 1 whose only element is s.
	//
	// Therefore, the function will always return a slice of min length 1.
	v := strings.SplitN(s, ":", 2)
	if len(v) == 1 {
		return nil, fmt.Errorf("invalid argument structure")
	}
	return &gitlab.JobVariableOptions{
		Key:   &v[0],
		Value: &v[1],
	}, nil
}

func extractEnvVar(s string) (*gitlab.JobVariableOptions, error) {
	jvar, err := parseVarArg(s)
	if err != nil {
		return nil, err
	}
	jvar.VariableType = gitlab.Ptr(gitlab.EnvVariableType)
	return jvar, nil
}

func extractFileVar(s string) (*gitlab.JobVariableOptions, error) {
	jvar, err := parseVarArg(s)
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(*jvar.Value)
	if err != nil {
		return nil, err
	}
	content := string(b)
	jvar.VariableType = gitlab.Ptr(gitlab.FileVariableType)
	jvar.Value = &content
	return jvar, nil
}

func resolveJobVars(cmd *cobra.Command) ([]*gitlab.JobVariableOptions, error) {
	jobVars := []*gitlab.JobVariableOptions{}
	for _, flag := range []string{"variables-env", "variables"} {
		if customJobVars, _ := cmd.Flags().GetStringSlice(flag); len(customJobVars) > 0 {
			for _, v := range customJobVars {
				jvar, err := extractEnvVar(v)
				if err != nil {
					return nil, fmt.Errorf("parsing job variable. Expected format KEY:VALUE: %w", err)
				}
				jobVars = append(jobVars, jvar)
			}
		}
	}

	if customJobFileVars, _ := cmd.Flags().GetStringSlice("variables-file"); len(customJobFileVars) > 0 {
		for _, v := range customJobFileVars {
			jvar, err := extractFileVar(v)
			if err != nil {
				return nil, fmt.Errorf("parsing job variable. Expected format KEY:FILENAME: %w", err)
			}
			jobVars = append(jobVars, jvar)
		}
	}

	vf, err := cmd.Flags().GetString("variables-from")
	if err != nil {
		return nil, err
	}

	if vf != "" {
		b, err := os.ReadFile(vf)
		if err != nil {
			return nil, fmt.Errorf("opening variable file: %s", vf)
		}
		var result []*gitlab.JobVariableOptions
		err = json.Unmarshal(b, &result)
		if err != nil {
			return nil, fmt.Errorf("loading job variable values: %w", err)
		}
		jobVars = append(jobVars, result...)
	}

	return jobVars, nil
}

func NewCmdTrigger(f cmdutils.Factory) *cobra.Command {
	pipelineTriggerCmd := &cobra.Command{
		Use:     "trigger <job-id>",
		Short:   `Trigger a manual CI/CD job.`,
		Aliases: []string{},
		Example: heredoc.Doc(`
			# Interactively select a job to trigger
			$ glab ci trigger

			# Trigger manual job with id 224356863
			$ glab ci trigger 224356863

			# Trigger manual job with name lint
			$ glab ci trigger lint

			# Trigger manual job with variables
			$ glab ci trigger 224356863 --variables DEBUG:true
			$ glab ci trigger lint --variables KEY1:value1 --variables KEY2:value2

			# Trigger job with variables from a JSON file
			$ glab ci trigger lint -f variables.json
	`),
		Long: ``,
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error

			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			client, err := f.GitLabClient()
			if err != nil {
				return err
			}

			jobName := ""
			if len(args) != 0 {
				jobName = args[0]
			}
			branch, _ := cmd.Flags().GetString("branch")
			pipelineId, _ := cmd.Flags().GetInt("pipeline-id")
			jobID, err := ciutils.GetJobId(cmd.Context(), &ciutils.JobInputs{
				JobName:         jobName,
				Branch:          branch,
				PipelineId:      pipelineId,
				SelectionPrompt: "Select pipeline job to trigger:",
				SelectionPredicate: func(s *gitlab.Job) bool {
					return s.Status == "manual"
				},
			}, &ciutils.JobOptions{
				Client:     client,
				IO:         f.IO(),
				Repo:       repo,
				BranchFunc: f.Branch,
			})
			if err != nil {
				if jobName != "" {
					fmt.Fprintln(f.IO().StdErr, "invalid job ID:", jobName)
				}
				return err
			}

			if jobID == 0 {
				return nil
			}

			jobVars, err := resolveJobVars(cmd)
			if err != nil {
				return err
			}

			playJobOpts := &gitlab.PlayJobOptions{}
			if len(jobVars) != 0 {
				playJobOpts.JobVariablesAttributes = &jobVars
			}

			job, _, err := client.Jobs.PlayJob(repo.FullName(), jobID, playJobOpts)
			if err != nil {
				return cmdutils.WrapError(err, fmt.Sprintf("Could not trigger job with ID: %d", jobID))
			}
			output := fmt.Sprintf("Triggered job (ID: %d), status: %s, ref: %s, weburl: %s", job.ID, job.Status, job.Ref, job.WebURL)
			fmt.Fprintln(f.IO().StdOut, output)

			return nil
		},
	}

	pipelineTriggerCmd.Flags().StringP("branch", "b", "", "The branch to search for the job. (default current branch)")
	pipelineTriggerCmd.Flags().IntP("pipeline-id", "p", 0, "The pipeline ID to search for the job.")
	pipelineTriggerCmd.Flags().StringSliceP("variables", "", []string{}, "Pass variables to job in format <key>:<value>.")
	pipelineTriggerCmd.Flags().StringSliceP("variables-env", "", []string{}, "Pass variables to job in format <key>:<value>.")
	pipelineTriggerCmd.Flags().StringSliceP("variables-file", "", []string{}, "Pass file contents as a file variable to job in format <key>:<filename>.")
	pipelineTriggerCmd.Flags().StringP("variables-from", "f", "", "JSON file with variables for job execution. Expects array of hashes, each with at least 'key' and 'value'.")
	return pipelineTriggerCmd
}
