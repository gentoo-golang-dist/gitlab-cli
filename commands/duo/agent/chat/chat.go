package chat

import (
	"encoding/json"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"gitlab.com/gitlab-org/cli/commands/cmdutils"
	"gitlab.com/gitlab-org/cli/commands/duo/agent/api"
	agentapi "gitlab.com/gitlab-org/cli/commands/duo/agent/api"
	"gitlab.com/gitlab-org/cli/commands/duo/agent/ui"
	"gitlab.com/gitlab-org/cli/pkg/iostreams"
)

type ChatOptions struct {
	factory cmdutils.Factory
	IO      *iostreams.IOStreams
}

const chatWorkflowDefinition = "chat"

func NewCmdChat(f cmdutils.Factory) *cobra.Command {
	duoChatCmd := &cobra.Command{
		Use:   "chat",
		Short: "Start an agentic chat session",
		Long:  ``,
		RunE: func(cmd *cobra.Command, args []string) error {

			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}

			apiClient, err := f.HttpClient()
			if err != nil {
				return err
			}

			workflowID, err := agentapi.CreateWorkflow(apiClient, repo.FullName(), "", chatWorkflowDefinition)
			if err != nil {
				return err
			}

			host := apiClient.BaseURL().Host
			config, err := f.Config()
			if err != nil {
				return err
			}

			wfClient, err := agentapi.NewWorkflowClient(host, config, f.IO())
			if err != nil {
				return err
			}

			msgChannel := make(chan string)

			initialModel := ui.InitialModel(msgChannel)
			p := tea.NewProgram(initialModel)

			go runWorkflow(wfClient, workflowID, msgChannel, p)
			if _, err := p.Run(); err != nil {
				fmt.Fprintf(f.IO().StdErr, "Error: %s", err)
				os.Exit(1)
			}
			return nil
		},
	}

	return duoChatCmd
}

func runWorkflow(wfClient *agentapi.WorkflowClient, workflowID int64, msgChannel chan string, tea *tea.Program) {
	for msg := range msgChannel {
		err := wfClient.StartWorkflow(workflowID, msg, "chat", func(ar api.ActionRequest) api.ActionResponseType {
			if ar.NewCheckpoint != nil {
				var checkpoint api.Checkpoint
				err := json.Unmarshal([]byte(ar.NewCheckpoint.Checkpoint), &checkpoint)
				if err != nil {
					tea.Send(ui.AddError(err))
					return api.ActionResponseType{
						PlainTextResponse: &api.ActionResponsePlainTextResponse{
							ResponseText: "Error: invalid checkpoint",
						},
					}
				}

				chatLog := checkpoint.ChannelValues.UIChatLog
				tea.Send(ui.UpdateMessages(chatLog))
			}

			return api.ActionResponseType{
				PlainTextResponse: &api.ActionResponsePlainTextResponse{
					ResponseText: "",
				},
			}

		})
		if err != nil {
			tea.Send(ui.AddError(err))
		}
	}

}
