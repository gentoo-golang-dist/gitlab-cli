package get

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/ci/ciutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/tableprinter"
)

const NoVariablesInPipelineMessage = "No variables found in pipeline."

type PipelineBridge struct {
	Bridge    *gitlab.Bridge             `json:"bridge"`
	Pipeline  *gitlab.Pipeline           `json:"pipeline"`
	Jobs      []*gitlab.Job              `json:"jobs"`
	Variables []*gitlab.PipelineVariable `json:"variables"`
}

type PipelineMergedResponse struct {
	*gitlab.Pipeline
	Jobs      []*gitlab.Job              `json:"jobs"`
	Bridges   []PipelineBridge           `json:"bridges"`
	Variables []*gitlab.PipelineVariable `json:"variables"`
}

func NewCmdGet(f cmdutils.Factory) *cobra.Command {
	pipelineGetCmd := &cobra.Command{
		Use:     "get [flags]",
		Short:   `Get JSON of a running CI/CD pipeline on the current or other specified branch.`,
		Aliases: []string{"stats"},
		Example: heredoc.Doc(`
			glab ci get
			glab ci -R some/project -p 12345`),
		Long: ``,
		Args: cobra.ExactArgs(0),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			c := f.IO().Color()

			client, err := f.GitLabClient()
			if err != nil {
				return err
			}

			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}

			// Parse arguments into local vars
			branch, _ := cmd.Flags().GetString("branch")
			pipelineId, err := cmd.Flags().GetInt("pipeline-id")
			if err != nil {
				return err
			}

			var msgNotFound string
			if pipelineId != 0 {
				msgNotFound = fmt.Sprintf("No pipeline with the given ID: %d", pipelineId)
			} else {
				// Use enhanced branch resolution that supports API fallback
				branch = ciutils.GetBranch(branch, func() (string, error) {
					return f.Branch()
				}, repo, client)

				commit, _, err := client.Commits.GetCommit(repo.FullName(), branch, nil)
				if err != nil {
					redCheck := c.Red("✘")
					fmt.Fprintf(f.IO().StdOut, "%s %v\n", redCheck, err)
					return err
				}

				// The latest commit on the branch won't work with a merged
				// result pipeline
				if commit.LastPipeline == nil {
					mr, _, err := mrutils.MRFromArgs(cmd.Context(), f, args, "any")
					if err != nil {
						return err
					}

					if mr.HeadPipeline == nil {
						return fmt.Errorf("no pipeline found. It might not exist yet. If this problem continues, check your pipeline configuration")
					} else {
						pipelineId = int(mr.HeadPipeline.ID)
					}

				} else {
					pipelineId = int(commit.LastPipeline.ID)
				}
				msgNotFound = fmt.Sprintf("No pipelines running or available on branch: %s", branch)
			}

			pipeline, _, err := client.Pipelines.GetPipeline(repo.FullName(), int64(pipelineId))
			if err != nil {
				redCheck := c.Red("✘")
				fmt.Fprintf(f.IO().StdOut, "%s %s\n", redCheck, msgNotFound)
				return err
			}

			jobs, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.Job, *gitlab.Response, error) {
				return client.Jobs.ListPipelineJobs(repo.FullName(), int64(pipelineId), &gitlab.ListJobsOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}, p)
			})
			if err != nil {
				return err
			}

			showJobDetails, _ := cmd.Flags().GetBool("with-job-details")
			showVariables, _ := cmd.Flags().GetBool("with-variables")
			withDownstreamPipelines, _ := cmd.Flags().GetBool("with-downstream-pipelines")
			var pipelineBridges []PipelineBridge

			if withDownstreamPipelines {
				bridges, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.Bridge, *gitlab.Response, error) {
					return client.Jobs.ListPipelineBridges(repo.FullName(), int64(pipelineId), &gitlab.ListJobsOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}, p)
				})
				if err != nil {
					return err
				}

				// Build a filtered list of bridges that actually have downstream pipelines.
				var filteredBridges []*gitlab.Bridge
				for _, bridge := range bridges {
					if bridge.DownstreamPipeline != nil {
						filteredBridges = append(filteredBridges, bridge)
					}
				}

				results := make([]PipelineBridge, len(filteredBridges))

				g, ctx := errgroup.WithContext(cmd.Context())
				sem := semaphore.NewWeighted(int64(runtime.GOMAXPROCS(0)))

				for i, bridge := range filteredBridges {
					if err := sem.Acquire(ctx, 1); err != nil {
						// If context is cancelled or acquire fails, stop and return error.
						return err
					}

					g.Go(func() error {
						// Ensure the token is released when the worker finishes.
						defer sem.Release(1)

						pb, err := fetchDownstreamPipeline(ctx, client, bridge, showVariables)
						if err != nil {
							// Provide context about which downstream pipeline failed, including a link when possible.
							dp := bridge.DownstreamPipeline
							baseMsg := fmt.Sprintf(
								"failed to fetch downstream pipeline for parent_pipeline_id=%d downstream_project_id=%d downstream_pipeline_id=%d web_url=%s",
								pipelineId,
								dp.ProjectID,
								dp.ID,
								dp.WebURL,
							)
							return fmt.Errorf("%s: %w", baseMsg, err)
						}
						results[i] = pb
						return nil
					})
				}

				// Wait for all workers and return any error.
				if err := g.Wait(); err != nil {
					return err
				}

				// Assign collected downstream pipelines
				pipelineBridges = results
			}

			var variables []*gitlab.PipelineVariable
			if showVariables {
				variables, _, err = client.Pipelines.GetPipelineVariables(pipeline.ProjectID, int64(pipelineId))
				if err != nil {
					return err
				}
			}

			mergedPipelineObject := &PipelineMergedResponse{
				Pipeline:  pipeline,
				Jobs:      jobs,
				Variables: variables,
				Bridges:   pipelineBridges,
			}

			outputFormat, _ := cmd.Flags().GetString("output-format")
			output, _ := cmd.Flags().GetString("output")
			if output == "json" || outputFormat == "json" {
				return f.IO().PrintJSON(*mergedPipelineObject)
			}
			printTable(*mergedPipelineObject, f.IO().StdOut, showJobDetails, withDownstreamPipelines)
			return nil
		},
	}

	pipelineGetCmd.Flags().StringP("branch", "b", "", "Check pipeline status for a branch. (default current branch)")
	pipelineGetCmd.Flags().IntP("pipeline-id", "p", 0, "Provide pipeline ID.")
	pipelineGetCmd.Flags().StringP("output", "F", "text", "Format output. Options: text, json.")
	pipelineGetCmd.Flags().StringP("output-format", "o", "text", "Use output.")
	_ = pipelineGetCmd.Flags().MarkHidden("output-format")
	_ = pipelineGetCmd.Flags().MarkDeprecated("output-format", "Deprecated. Use 'output' instead.")
	pipelineGetCmd.Flags().BoolP("with-job-details", "d", false, "Show extended job information.")
	pipelineGetCmd.Flags().Bool("with-variables", false, "Show variables in pipeline. Requires the Maintainer role.")
	pipelineGetCmd.Flags().Bool("with-downstream-pipelines", false, "Show child pipelines.")

	return pipelineGetCmd
}

func printTable(p PipelineMergedResponse, dest io.Writer, showJobDetails bool, withChildPipelines bool) {
	printPipelineTable(p.Pipeline, dest)

	if showJobDetails {
		printJobTable(p.Jobs, dest)
	} else {
		printJobText(p.Jobs, dest)
	}

	printVariables(p.Variables, dest)

	if withChildPipelines {
		for idx, bridge := range p.Bridges {
			normal_idx := idx + 1
			printPipelineTable(bridge.Pipeline, dest, normal_idx)
			if showJobDetails {
				printJobTable(bridge.Jobs, dest, normal_idx)
			} else {
				printJobText(bridge.Jobs, dest, normal_idx)
			}

			printVariables(bridge.Variables, dest, normal_idx)
		}
	}
}

func printPipelineTable(p *gitlab.Pipeline, dest io.Writer, isChild ...int) {
	if len(isChild) > 0 {
		fmt.Fprintf(dest, "# Child %d pipeline :\n", isChild[0])
	} else {
		fmt.Fprint(dest, "# Pipeline:\n")
	}
	pipelineTable := tableprinter.NewTablePrinter()
	pipelineTable.AddRow("id:", strconv.FormatInt(p.ID, 10))
	pipelineTable.AddRow("status:", p.Status)
	pipelineTable.AddRow("source:", p.Source)
	pipelineTable.AddRow("ref:", p.Ref)
	pipelineTable.AddRow("sha:", p.SHA)
	pipelineTable.AddRow("tag:", p.Tag)
	pipelineTable.AddRow("yaml Errors:", p.YamlErrors)
	pipelineTable.AddRow("user:", p.User.Username)
	pipelineTable.AddRow("created:", p.CreatedAt)
	pipelineTable.AddRow("started:", p.StartedAt)
	pipelineTable.AddRow("updated:", p.UpdatedAt)
	fmt.Fprintln(dest, pipelineTable.String())
}

func printJobTable(p []*gitlab.Job, dest io.Writer, isChild ...int) {
	if len(isChild) > 0 {
		fmt.Fprintf(dest, "# Child %d jobs :\n", isChild[0])
	} else {
		fmt.Fprint(dest, "# Jobs:\n")
	}
	jobTable := tableprinter.NewTablePrinter()
	jobTable.AddRow("ID", "Name", "Status", "Duration", "Failure reason")
	for _, j := range p {
		jobTable.AddRow(j.ID, j.Name, j.Status, j.Duration, j.FailureReason)
	}
	fmt.Fprintln(dest, jobTable.String())
}

func printJobText(p []*gitlab.Job, dest io.Writer, isChild ...int) {
	if len(isChild) > 0 {
		fmt.Fprintf(dest, "# Child %d jobs :\n", isChild[0])
	} else {
		fmt.Fprint(dest, "# Jobs:\n")
	}
	jobTable := tableprinter.NewTablePrinter()
	for _, j := range p {
		jobTable.AddRow(j.Name+":", j.Status)
	}
	fmt.Fprintln(dest, jobTable.String())
}

func printVariables(vars []*gitlab.PipelineVariable, dest io.Writer, isChild ...int) {
	if vars != nil {
		if len(isChild) > 0 {
			fmt.Fprintf(dest, "# Child %d variables :\n", isChild[0])
		} else {
			fmt.Fprint(dest, "# Variables:\n")
		}
		if len(vars) == 0 {
			fmt.Fprint(dest, NoVariablesInPipelineMessage)
		}

		varTable := tableprinter.NewTablePrinter()
		for _, v := range vars {
			varTable.AddRow(v.Key+":", v.Value)
		}
		fmt.Fprintln(dest, varTable.String())
	}
}

func fetchDownstreamPipeline(ctx context.Context, apiClient *gitlab.Client, br *gitlab.Bridge, showVariables bool) (PipelineBridge, error) {
	// Get the downstream pipeline
	childPipeline, _, err := apiClient.Pipelines.GetPipeline(br.DownstreamPipeline.ProjectID, br.DownstreamPipeline.ID, gitlab.WithContext(ctx))
	if err != nil {
		return PipelineBridge{}, err
	}

	// Get jobs for the downstream pipeline
	childJobs, err := gitlab.ScanAndCollect(func(p gitlab.PaginationOptionFunc) ([]*gitlab.Job, *gitlab.Response, error) {
		return apiClient.Jobs.ListPipelineJobs(br.DownstreamPipeline.ProjectID, childPipeline.ID, &gitlab.ListJobsOptions{ListOptions: gitlab.ListOptions{PerPage: 100}}, p, gitlab.WithContext(ctx))
	})
	if err != nil {
		return PipelineBridge{}, err
	}

	// Optionally fetch variables
	var childVariables []*gitlab.PipelineVariable
	if showVariables {
		childVariables, _, err = apiClient.Pipelines.GetPipelineVariables(br.DownstreamPipeline.ProjectID, childPipeline.ID, gitlab.WithContext(ctx))
		if err != nil {
			return PipelineBridge{}, err
		}
	}

	return PipelineBridge{
		Pipeline:  childPipeline,
		Bridge:    br,
		Jobs:      childJobs,
		Variables: childVariables,
	}, nil
}
