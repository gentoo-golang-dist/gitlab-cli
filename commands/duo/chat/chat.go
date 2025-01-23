package chat

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/hasura/go-graphql-client"
	"github.com/spf13/cobra"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/commands/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/pkg/iostreams"
)

type chatOpts struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*gitlab.Client, error)
	Config     func() (config.Config, error)
}

type aiCompletionResponse struct {
	ID        string    `json:"id"`
	RequestID string    `json:"requestId"`
	Content   string    `json:"content"`
	Errors    []string  `json:"errors"`
	Role      string    `json:"role"`
	Timestamp time.Time `json:"timestamp"`
	ChunkID   int       `json:"chunkId"`
}

const (
	spinnerText       = "Connecting to GitLab Duo Chat..."
	apiUnreachableErr = "Error: API is unreachable."
	wsConnectErr      = "Error: Failed to connect to WebSocket."
	wsReadErr         = "Error: Failed to read WebSocket message."
	wsSendErr         = "Error: Failed to send WebSocket message."
	maxRetries        = 5
	retryDelay        = 5 * time.Second
)

func NewCmdChat(f *cmdutils.Factory) *cobra.Command {
	opts := &chatOpts{
		IO:         f.IO,
		HttpClient: f.HttpClient,
		Config:     f.Config,
	}

	chatCmd := &cobra.Command{
		Use:   "chat",
		Short: "Start an interactive chat session with GitLab Duo",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runChatSession(opts)
		},
	}

	return chatCmd
}

func runChatSession(opts *chatOpts) error {
	opts.IO.StartSpinner(spinnerText)
	defer opts.IO.StopSpinner("")

	client, err := opts.HttpClient()
	if err != nil {
		return cmdutils.WrapError(err, "failed to get HTTP client")
	}

	baseURL := client.BaseURL()
	wsURL := *baseURL
	wsURL.Path = "/-/cable"
	wsURL.Scheme = "wss"

	opts.IO.LogInfo(fmt.Sprintf("Attempting to connect to WebSocket URL: %s\n", wsURL.String()))

	// Try making an HTTP request to the WebSocket URL
	httpURL := wsURL
	httpURL.Scheme = "https"
	resp, err := http.Get(httpURL.String())
	if err != nil {
		opts.IO.LogInfo(fmt.Sprintf("Error making HTTP request to WebSocket URL: %v\n", err))
	} else {
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		opts.IO.LogInfo(fmt.Sprintf("HTTP response status: %s\n", resp.Status))
		opts.IO.LogInfo(fmt.Sprintf("HTTP response body: %s\n", string(body)))
	}

	cfg, err := opts.Config()
	if err != nil {
		return cmdutils.WrapError(err, "failed to get config")
	}
	token, _ := cfg.Get(baseURL.Host, "token")

	opts.IO.LogInfo(fmt.Sprintf("Using token: %s\n", maskToken(token)))

	headers := http.Header{
		"Origin":                 {baseURL.String()},
		"Sec-WebSocket-Protocol": {"actioncable-v1-json", "actioncable-unsupported"},
		"Authorization":          {"Bearer " + token},
	}

	opts.IO.LogInfo("WebSocket Headers:")
	for k, v := range headers {
		opts.IO.LogInfo(fmt.Sprintf("%s: %s\n", k, v))
	}

	subscriptionClient := graphql.NewSubscriptionClient(wsURL.String()).
		WithConnectionParams(map[string]interface{}{
			"headers": headers,
		}).
		WithLog(func(args ...interface{}) {
			opts.IO.LogInfo(fmt.Sprintf("WebSocket Log: %v\n", args))
		}).
		WithWebSocketOptions(graphql.WebsocketOptions{
			HTTPHeader: headers,
		}).
		WithRetryTimeout(time.Minute).
		WithRetryStatusCodes("4000-4999")

	opts.IO.StopSpinner("")
	opts.IO.LogInfo("Attempting to establish WebSocket connection...\n")

	subscriptionId := generateUniqueID()

	var subscription struct {
		AiCompletionResponse struct {
			ID        string
			RequestID string
			Content   string
			Errors    []string
			Role      string
			Timestamp time.Time
			Type      string
			ChunkID   int
			Extras    struct {
				Sources  []string
				TypeName string `json:"__typename"`
			}
			TypeName string `json:"__typename"`
		} `graphql:"aiCompletionResponse(userId: $userId, aiAction: $aiAction, clientSubscriptionId: $clientSubscriptionId)"`
	}

	variables := map[string]interface{}{
		"userId":               graphql.ID(subscriptionId),
		"aiAction":             "CHAT",
		"clientSubscriptionId": subscriptionId,
		"htmlResponse":         true,
	}

	subID, err := subscriptionClient.Subscribe(&subscription, variables, func(data []byte, err error) error {
		if err != nil {
			opts.IO.LogInfo(fmt.Sprintf("Subscription error: %v\n", err))
			return nil
		}

		opts.IO.LogInfo(fmt.Sprintf("Received data: %s\n", string(data)))

		var response struct {
			AiCompletionResponse aiCompletionResponse
		}
		if err := json.Unmarshal(data, &response); err != nil {
			opts.IO.LogInfo(fmt.Sprintf("Error unmarshaling response: %v\n", err))
			return nil
		}

		displayFormattedResponse(opts, response.AiCompletionResponse.Content)
		return nil
	})

	if err != nil {
		return cmdutils.WrapError(err, "failed to set up subscription")
	}

	opts.IO.LogInfo("Connected! Type 'exit' or 'quit' to end the session.\n")

	errChan := make(chan error, 1)
	go func() {
		for i := 0; i < maxRetries; i++ {
			if err := subscriptionClient.Run(); err != nil {
				opts.IO.LogInfo(fmt.Sprintf("Subscription client error: %v\n", err))
				opts.IO.LogInfo(fmt.Sprintf("Retrying in %v seconds...\n", retryDelay.Seconds()))
				time.Sleep(retryDelay)
			} else {
				break
			}
		}
		errChan <- fmt.Errorf("max retries reached, subscription client stopped")
	}()

	reader := bufio.NewReader(opts.IO.In)
	for {
		select {
		case err := <-errChan:
			opts.IO.LogInfo(fmt.Sprintf("Subscription client stopped: %v\n", err))
			return err
		default:
			opts.IO.LogInfo(opts.IO.Color().Bold("You: "))
			input, err := reader.ReadString('\n')
			if err != nil {
				return cmdutils.WrapError(err, "failed to read input")
			}

			input = strings.TrimSpace(input)
			if input == "exit" || input == "quit" {
				subscriptionClient.Unsubscribe(subID)
				subscriptionClient.Close()
				return nil
			}

			if input == "" {
				continue
			}

			if err := sendChatMessage(subscriptionClient, input, subscriptionId); err != nil {
				opts.IO.LogInfo(fmt.Sprintf("Error sending message: %v\n", err))
			} else {
				opts.IO.LogInfo("Message sent successfully\n")
			}
		}
	}
}

func sendChatMessage(client *graphql.SubscriptionClient, content string, subscriptionId string) error {
	mutation := `
        mutation chat($input: AiActionInput!) {
            aiAction(input: $input) {
                requestId
                errors
                __typename
            }
        }
    `

	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"chat": map[string]interface{}{
				"content": content,
			},
			"clientSubscriptionId": subscriptionId,
			"conversationType":     "DUO_CHAT",
			"platformOrigin":       "cli",
		},
	}

	var response struct {
		AiAction struct {
			RequestID string   `json:"requestId"`
			Errors    []string `json:"errors"`
			TypeName  string   `json:"__typename"`
		} `json:"aiAction"`
	}

	_, err := client.Exec(mutation, variables, func(message []byte, err error) error {
		if err != nil {
			return fmt.Errorf("error in Exec callback: %v", err)
		}
		return json.Unmarshal(message, &response)
	})

	if err != nil {
		return fmt.Errorf("failed to execute mutation: %v", err)
	}

	if len(response.AiAction.Errors) > 0 {
		return fmt.Errorf("mutation errors: %v", response.AiAction.Errors)
	}

	return nil
}

func displayFormattedResponse(opts *chatOpts, content string) {
	color := opts.IO.Color()
	opts.IO.LogInfo(color.Bold("GitLab Duo: "))

	paragraphs := strings.Split(content, "\n\n")
	for _, paragraph := range paragraphs {
		paragraph = strings.ReplaceAll(paragraph, "```", color.Cyan("```"))
		paragraph = strings.ReplaceAll(paragraph, "`", color.Cyan("`"))

		for _, pattern := range []string{"Note:", "Important:", "Warning:"} {
			paragraph = strings.ReplaceAll(paragraph, pattern, color.Bold(pattern))
		}

		opts.IO.LogInfo(paragraph + "\n")
	}
	opts.IO.LogInfo("\n")
}

func generateUniqueID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func maskToken(token string) string {
	if len(token) > 8 {
		return token[:4] + "..." + token[len(token)-4:]
	}
	return "****"
}
