package api

import (
	"encoding/json"
	"strconv"

	"github.com/gorilla/websocket"
	"gitlab.com/gitlab-org/cli/api"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/pkg/iostreams"
)

type WorkflowEvent struct {
	StartRequest   *StartRequest   `json:"startRequest,omitempty"`
	ActionResponse *ActionResponse `json:"actionResponse,omitempty"`
}

type StartRequest struct {
	WorkflowID         string `json:"workflowID"`
	ClientVersion      string `json:"clientVersion"`
	WorkflowDefinition string `json:"workflowDefinition"`
	Goal               string `json:"goal"`
}

type ActionRequest struct {
	RequestID     string         `json:"requestID"`
	NewCheckpoint *NewCheckpoint `json:"newCheckpoint,omitempty"`
}

type NewCheckpoint struct {
	Status     string `json:"status"`
	Checkpoint string `json:"checkpoint"`
}

type ActionResponse struct {
	RequestID    string             `json:"requestID"`
	Response     string             `json:"response"`
	ResponseType ActionResponseType `json:"responseType"`
}

type ActionResponseType struct {
	PlainTextResponse *ActionResponsePlainTextResponse `json:"plainTextResponse,omitempty"`
	HttpResponse      *ActionResponseHttpResponse      `json:"httpResponse,omitempty"`
}

type ActionResponsePlainTextResponse struct {
	ResponseText string `json:"responseText"`
	Error        string `json:"error,omitempty"`
}

type ActionResponseHttpResponse struct {
	Headers    map[string]string `json:"headers"`
	StatusCode int32             `json:"statusCode"`
	Body       string            `json:"body"`
	Error      string            `json:"error,omitempty"`
}

type WorkflowClient struct {
	baseClient *api.Client
	io         *iostreams.IOStreams
}

func NewWorkflowClient(host string, cfg config.Config, io *iostreams.IOStreams) (*WorkflowClient, error) {
	baseClient, err := api.NewClientWithCfg(host, cfg, false)
	if err != nil {
		return nil, err
	}

	return &WorkflowClient{baseClient: baseClient, io: io}, nil
}

func (c *WorkflowClient) StartWorkflow(workflowID int64, goal string, definition string, callback func(ActionRequest) ActionResponseType) error {
	wsConnection, err := c.baseClient.NewWebSocketConnection()
	if err != nil {
		return err
	}
	defer wsConnection.Close()

	// Convert int64 to string using strconv.FormatInt
	workflowIDString := strconv.FormatInt(workflowID, 10)

	event := WorkflowEvent{
		StartRequest: &StartRequest{
			WorkflowID:         workflowIDString,
			ClientVersion:      "1.0.0",
			WorkflowDefinition: definition,
			Goal:               goal,
		},
	}

	eventJSON, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = wsConnection.WriteMessage(websocket.TextMessage, eventJSON)
	if err != nil {
		return err
	}

	for {
		_, message, err := wsConnection.ReadMessage()
		if err != nil {
			return err
		}

		action := ActionRequest{}
		err = json.Unmarshal(message, &action)
		if err != nil {
			return err
		}

		toolResponse := callback(action)

		response := ActionResponse{
			RequestID:    action.RequestID,
			ResponseType: toolResponse,
		}

		responseBytes, err := json.Marshal(&response)
		if err != nil {
			return err
		}

		err = wsConnection.WriteMessage(websocket.TextMessage, responseBytes)
		if err != nil {
			return err
		}
	}
}
