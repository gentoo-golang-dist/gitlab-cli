package api

import (
	"net/http"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type createWorkflowRequest struct {
	ProjectId          string `json:"project_id"`
	Goal               string `json:"goal"`
	WorkflowDefinition string `json:"workflow_definition"`
	AgentPrivileges    []int  `json:"agent_privileges"`
}

type createWorkflowResponse struct {
	ID int64 `json:"id"`
}

func CreateWorkflow(apiClient *gitlab.Client, projectPath, goal, definition string) (int64, error) {
	reqMessage := createWorkflowRequest{
		ProjectId:          projectPath,
		Goal:               "",
		WorkflowDefinition: "chat",
		AgentPrivileges:    []int{1, 2},
	}

	req, err := apiClient.NewRequest(http.MethodPost, "ai/duo_workflows/workflows", &reqMessage, []gitlab.RequestOptionFunc{})
	if err != nil {
		return -1, err
	}

	var response createWorkflowResponse
	_, err = apiClient.Do(req, &response)
	if err != nil {
		return -1, err
	}

	return response.ID, nil
}
