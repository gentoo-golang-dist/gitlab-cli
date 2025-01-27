package chat

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/commands/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/pkg/iostreams"
)

const (
	gitlabBaseURL    = "https://gitlab.com"
	gitlabAPIURL     = gitlabBaseURL + "/api/v4"
	gitlabGraphQLURL = gitlabBaseURL + "/api/graphql"
	gitlabWSURL      = "wss://gitlab.com/-/cable"
)

type chatOpts struct {
	IO         *iostreams.IOStreams
	HttpClient func() (*gitlab.Client, error)
	Config     func() (config.Config, error)
}

type DuoChatClient struct {
	token       string
	userID      string
	conn        *websocket.Conn
	responses   chan CompletionResponse
	IO          *iostreams.IOStreams
	debugLogger *log.Logger
	debugFile   *os.File
}

type ActionCableMessage struct {
	Type       string          `json:"type,omitempty"`
	Command    string          `json:"command,omitempty"`
	Identifier string          `json:"identifier,omitempty"`
	Message    json.RawMessage `json:"message,omitempty"`
}

type CompletionResponse struct {
	ID        string   `json:"id"`
	RequestID string   `json:"requestId"`
	Content   string   `json:"content"`
	Errors    []string `json:"errors"`
	Role      string   `json:"role"`
	Timestamp string   `json:"timestamp"`
	Type      *string  `json:"type"`
	ChunkID   *int     `json:"chunkId"`
}

func NewCmdChat(f *cmdutils.Factory) *cobra.Command {
	opts := &chatOpts{
		IO:         f.IO,
		HttpClient: f.HttpClient,
		Config:     f.Config,
	}

	var debug bool

	chatCmd := &cobra.Command{
		Use:   "chat",
		Short: "Start an interactive chat session with GitLab Duo",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runChatSession(opts, debug)
		},
	}

	chatCmd.Flags().BoolVar(&debug, "debug", false, "Enable debug logging")

	return chatCmd
}

func runChatSession(opts *chatOpts, debug bool) error {
	opts.IO.StartSpinner("Connecting to GitLab Duo Chat...")
	defer opts.IO.StopSpinner("")

	cfg, err := opts.Config()
	if err != nil {
		return cmdutils.WrapError(err, "failed to get config")
	}
	token, _ := cfg.Get(gitlabBaseURL, "token")

	client, err := NewDuoChatClient(context.Background(), token, opts.IO, debug)
	if err != nil {
		return cmdutils.WrapError(err, "failed to create Duo Chat client")
	}
	defer client.Close()

	subID := uuid.New().String()
	if err := client.Subscribe(context.Background(), subID); err != nil {
		return cmdutils.WrapError(err, "failed to subscribe")
	}

	opts.IO.StopSpinner("")
	opts.IO.LogInfo("Connected! Type 'exit' or 'quit' to end the session.\n")

	reader := bufio.NewReader(opts.IO.In)
	inputChan := make(chan string, 1)
	errChan := make(chan error, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	inputReady := make(chan struct{}, 1)
	inputReady <- struct{}{} // Initially allow input

	go func() {
		for {
			<-inputReady // Wait for input to be allowed
			opts.IO.LogInfo("\n" + opts.IO.Color().Green("You: "))
			input, err := reader.ReadString('\n')
			if err != nil {
				errChan <- cmdutils.WrapError(err, "failed to read input")
				return
			}
			input = strings.TrimSpace(input)
			if input != "" {
				client.debugLog("Sending input to channel: '%s'", input)
				inputChan <- input
			}
		}
	}()

	for {
		select {
		case input := <-inputChan:
			if strings.ToLower(input) == "exit" || strings.ToLower(input) == "quit" {
				opts.IO.LogInfo("Ending chat session...\n")
				client.debugLog("Exit condition met, returning from runChatSession")
				return nil
			}

			responseCtx, cancelResponse := context.WithTimeout(ctx, 30*time.Second)
			defer cancelResponse()

			if err := client.SendPrompt(responseCtx, input, subID); err != nil {
				client.debugLog("Error sending prompt: %v", err)
				opts.IO.LogInfo(fmt.Sprintf("Error sending message: %v\n", err))
				continue
			}

			client.debugLog("Processing responses...")

			// Disable input while waiting for response
			select {
			case <-inputReady:
				// Input was ready, now disable it
			default:
				// Input was already disabled, do nothing
			}

			opts.IO.StartSpinner("Waiting for GitLab Duo response...")
			opts.IO.LogInfo("\n" + opts.IO.Color().Cyan("GitLab Duo: "))

			responseDone := make(chan struct{})
			go func() {
				defer close(responseDone)
				if err := client.ProcessResponses(responseCtx); err != nil {
					if err != context.Canceled {
						opts.IO.LogInfo(fmt.Sprintf("\nError processing responses: %v\n", err))
						if err := client.reconnect(ctx); err != nil {
							opts.IO.LogInfo(fmt.Sprintf("Failed to reconnect: %v\n", err))
						}
					}
				}
			}()

			select {
			case <-responseDone:
				opts.IO.StopSpinner("")
				opts.IO.LogInfo("\n") // Add a newline after GitLab Duo's response
			case <-responseCtx.Done():
				opts.IO.StopSpinner("Response timed out")
			}

			// Re-enable input after response
			select {
			case inputReady <- struct{}{}:
				// Enable input
			default:
				// Input was already enabled, do nothing
			}

			cancelResponse()
			client.debugLog("Finished processing responses")

		case err := <-errChan:
			return err

		case <-time.After(100 * time.Millisecond):
			// This case prevents the select from blocking indefinitely
			continue
		}
	}
}

func NewDuoChatClient(ctx context.Context, token string, io *iostreams.IOStreams, debug bool) (*DuoChatClient, error) {
	userID, err := fetchUserID(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("fetch user ID: %w", err)
	}

	conn, err := setupWebSocket(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("setup websocket: %w", err)
	}

	var debugLogger *log.Logger
	var debugFile *os.File

	if debug {
		debugFile, err = os.OpenFile("duo_chat_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("create debug log file: %w", err)
		}
		debugLogger = log.New(debugFile, "DEBUG: ", log.Ldate|log.Ltime|log.Lshortfile)
	}

	return &DuoChatClient{
		token:       token,
		userID:      userID,
		conn:        conn,
		responses:   make(chan CompletionResponse, 100),
		IO:          io,
		debugLogger: debugLogger,
		debugFile:   debugFile,
	}, nil
}

func (c *DuoChatClient) Close() error {
	if c.debugFile != nil {
		if err := c.debugFile.Close(); err != nil {
			return fmt.Errorf("close debug file: %w", err)
		}
	}
	return c.conn.Close(websocket.StatusNormalClosure, "")
}

func (c *DuoChatClient) debugLog(format string, v ...interface{}) {
	if c.debugLogger != nil {
		c.debugLogger.Printf(format, v...)
	}
}

func fetchUserID(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gitlabAPIURL+"/user", nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API error: status=%d body=%s", resp.StatusCode, body)
	}

	var userData struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&userData); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	return fmt.Sprintf("gid://gitlab/User/%d", userData.ID), nil
}

func setupWebSocket(ctx context.Context, token string) (*websocket.Conn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	c, _, err := websocket.Dial(dialCtx, gitlabWSURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": {"Bearer " + token},
			"Origin":        {gitlabBaseURL},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("websocket dial: %w", err)
	}

	return c, nil
}

func (c *DuoChatClient) Subscribe(ctx context.Context, subID string) error {
	if err := c.subscribeMain(ctx); err != nil {
		return fmt.Errorf("subscribe main: %w", err)
	}

	if err := c.subscribeStream(ctx, subID); err != nil {
		return fmt.Errorf("subscribe stream: %w", err)
	}

	go c.handleMessages(ctx)
	return nil
}

func (c *DuoChatClient) subscribeMain(ctx context.Context) error {
	query := map[string]interface{}{
		"channel": "GraphqlChannel",
		"query": `subscription aiCompletionResponse($userId: UserID, $aiAction: AiAction, $clientSubscriptionId) {
                aiCompletionResponse(
                    userId: $userId
                    aiAction: $aiAction
                ) {
                    id requestId content errors role timestamp type chunkId
                }
            }`,
		"variables": map[string]interface{}{
			"aiAction": "CHAT",
			"userId":   c.userID,
		},
		"operationName": "aiCompletionResponse",
		"nonce":         uuid.New().String(),
	}

	return c.sendSubscription(ctx, query)
}

func (c *DuoChatClient) subscribeStream(ctx context.Context, subID string) error {
	query := map[string]interface{}{
		"channel": "GraphqlChannel",
		"query": `subscription aiCompletionResponseStream($userId: UserID, $clientSubscriptionId: String) {
                aiCompletionResponse(
                    userId: $userId
                    aiAction: CHAT
                    clientSubscriptionId: $clientSubscriptionId
                ) {
                    id requestId content errors role timestamp type chunkId
                }
            }`,
		"variables": map[string]interface{}{
			"clientSubscriptionId": subID,
			"userId":               c.userID,
		},
		"operationName": "aiCompletionResponseStream",
		"nonce":         uuid.New().String(),
	}

	return c.sendSubscription(ctx, query)
}

func (c *DuoChatClient) sendSubscription(ctx context.Context, query map[string]interface{}) error {
	identifierBytes, err := json.Marshal(query)
	if err != nil {
		return fmt.Errorf("marshal query: %w", err)
	}

	msg := ActionCableMessage{
		Command:    "subscribe",
		Identifier: string(identifierBytes),
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	return c.conn.Write(ctx, websocket.MessageText, msgBytes)
}

func (c *DuoChatClient) SendPrompt(ctx context.Context, prompt, subID string) error {
	mutation := `
        mutation chat($question: String!, $clientSubscriptionId: String) {
            aiAction(
                input: {
                    chat: {
                        content: $question
                    }
                    clientSubscriptionId: $clientSubscriptionId
                }
            ) {
                requestId
                errors
            }
        }
    `

	variables := map[string]interface{}{
		"question":             prompt,
		"clientSubscriptionId": subID,
	}

	body, err := json.Marshal(map[string]interface{}{
		"query":     mutation,
		"variables": variables,
	})
	if err != nil {
		return fmt.Errorf("marshal mutation: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, gitlabGraphQLURL, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GraphQL error: status=%d body=%s", resp.StatusCode, body)
	}

	return nil
}

func (c *DuoChatClient) handleMessages(ctx context.Context) {
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-pingTicker.C:
			if err := c.sendPing(ctx); err != nil {
				c.debugLog("Failed to send ping: %v", err)
			}
		default:
			_, rawMsg, err := c.conn.Read(ctx)
			if err != nil {
				if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
					c.debugLog("WebSocket connection closed normally")
					return
				}
				c.debugLog("WebSocket read error: %v", err)
				if err := c.reconnect(ctx); err != nil {
					c.debugLog("Failed to reconnect: %v", err)
					return
				}
				continue
			}

			var msg ActionCableMessage
			if err := json.Unmarshal(rawMsg, &msg); err != nil {
				c.debugLog("Error unmarshaling message: %v", err)
				continue
			}

			switch msg.Type {
			case "ping":
				continue
			case "welcome":
				c.debugLog("Connected to ActionCable")
			case "confirm_subscription":
				c.debugLog("Subscribed to channel")
			case "reject_subscription":
				c.debugLog("Subscription rejected")
				if err := c.reconnect(ctx); err != nil {
					c.debugLog("Failed to reconnect: %v", err)
					return
				}
			default:
				if err := c.handleGraphQLMessage(msg.Message); err != nil {
					c.debugLog("Handle GraphQL message error: %v", err)
				}
			}
		}
	}
}

func (c *DuoChatClient) handleGraphQLMessage(rawMsg json.RawMessage) error {
	var graphqlMsg struct {
		Result struct {
			Data struct {
				AiCompletionResponse CompletionResponse `json:"aiCompletionResponse"`
			} `json:"data"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rawMsg, &graphqlMsg); err != nil {
		return fmt.Errorf("parse GraphQL message: %w", err)
	}

	c.responses <- graphqlMsg.Result.Data.AiCompletionResponse
	return nil
}

func (c *DuoChatClient) ProcessResponses(ctx context.Context) error {
	finishedRequests := make(map[string]bool)
	pendingResponses := make(map[int]CompletionResponse)
	var currentRequestID string
	maxChunkID := -1
	var buffer strings.Builder

	responseChan := make(chan CompletionResponse)
	errChan := make(chan error)
	doneChan := make(chan struct{})

	go func() {
		for {
			select {
			case response, ok := <-c.responses:
				if !ok {
					errChan <- errors.New("response channel closed")
					return
				}
				responseChan <- response
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		defer close(doneChan)
		lastActivityTime := time.Now()

		for {
			select {
			case response := <-responseChan:
				c.debugLog("Received chunk: ID=%v, Type=%v, Content=%s", response.ChunkID, response.Type, response.Content)
				lastActivityTime = time.Now()

				if response.Role != "ASSISTANT" || finishedRequests[response.RequestID] {
					continue
				}

				if currentRequestID == "" {
					currentRequestID = response.RequestID
				}

				if response.RequestID != currentRequestID {
					continue
				}

				if response.ChunkID != nil && *response.ChunkID > maxChunkID {
					for i := maxChunkID + 1; i <= *response.ChunkID; i++ {
						if pending, ok := pendingResponses[i]; ok {
							buffer.WriteString(pending.Content)
							delete(pendingResponses, i)
						} else if i == *response.ChunkID {
							buffer.WriteString(response.Content)
						}
					}
					maxChunkID = *response.ChunkID
					c.IO.LogInfo(buffer.String())
					buffer.Reset()
				} else if response.ChunkID == nil {
					// This is likely the final message
					c.IO.LogInfo(response.Content)
				} else {
					pendingResponses[*response.ChunkID] = response
				}

				if isResponseComplete(response) {
					c.debugLog("Response complete, ending processing")
					finishedRequests[response.RequestID] = true
					c.IO.LogInfo("\n") // Add a newline after the complete response
					return
				}

			case err := <-errChan:
				c.debugLog("Error processing responses: %v", err)
				return

			case <-ctx.Done():
				c.debugLog("Context cancelled in ProcessResponses")
				return

			case <-time.After(1 * time.Second):
				if time.Since(lastActivityTime) > 5*time.Second && len(buffer.String()) > 0 {
					c.debugLog("No new chunks received for 5 seconds, assuming completion")
					c.IO.LogInfo("\n") // Add a newline after the assume
					c.IO.LogInfo("\n") // Add a newline after the assumed complete response
					return
				}
			}
		}
	}()

	select {
	case <-doneChan:
		return nil
	case <-time.After(30 * time.Second):
		return errors.New("response timeout")
	}
}

func isResponseComplete(response CompletionResponse) bool {
	return response.ChunkID == nil ||
		response.Type != nil && *response.Type == "COMPLETE" ||
		(len(response.Content) > 0 && (strings.HasSuffix(response.Content, ".") || strings.HasSuffix(response.Content, "?") || strings.HasSuffix(response.Content, "!")))
}

func (c *DuoChatClient) sendPing(ctx context.Context) error {
	pingMessage := ActionCableMessage{
		Type:    "ping",
		Message: json.RawMessage("{}"),
	}
	msgBytes, err := json.Marshal(pingMessage)
	if err != nil {
		return fmt.Errorf("marshal ping message: %w", err)
	}
	return c.conn.Write(ctx, websocket.MessageText, msgBytes)
}

func (c *DuoChatClient) reconnect(ctx context.Context) error {
	maxRetries := 5
	backoff := time.Second

	for i := 0; i < maxRetries; i++ {
		c.debugLog("Attempting to reconnect (attempt %d of %d)...", i+1, maxRetries)

		newConn, err := setupWebSocket(ctx, c.token)
		if err != nil {
			c.debugLog("Reconnection attempt failed: %v", err)
			backoff *= 2 // Exponential backoff
			time.Sleep(backoff)
			continue
		}

		c.conn = newConn

		subID := uuid.New().String()
		if err := c.Subscribe(ctx, subID); err != nil {
			c.debugLog("Failed to resubscribe: %v", err)
			c.conn.Close(websocket.StatusInternalError, "")
			backoff *= 2 // Exponential backoff
			time.Sleep(backoff)
			continue
		}

		c.debugLog("Reconnected successfully")
		return nil
	}

	return errors.New("failed to reconnect after maximum retries")
}
