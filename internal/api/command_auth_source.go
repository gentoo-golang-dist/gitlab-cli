package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/google/shlex"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

var _ gitlab.AuthSource = (*CommandTokenAuthSource)(nil)

// commandTimeout is the maximum time allowed for a token_command to complete.
// Set to 2 minutes to accommodate interactive flows such as browser-based auth.
const commandTimeout = 2 * time.Minute

// CommandTokenAuthSource implements gitlab.AuthSource by executing an external
// command to obtain a token on every request. The command is responsible for
// any caching, expiry management, and token refresh — glab does none of that.
type CommandTokenAuthSource struct {
	command  string
	args     []string
	hostname string
}

// commandTokenResponse is the JSON format expected from the token command's stdout.
type commandTokenResponse struct {
	Type  string `json:"type"`  // "pat", "oauth2", or "job-token"
	Token string `json:"token"` // the token value
}

// NewCommandTokenAuthSource parses commandStr using shell-aware splitting and
// returns a CommandTokenAuthSource. The first token is the executable; the rest
// are arguments. No shell is involved in the split or in execution.
func NewCommandTokenAuthSource(commandStr, hostname string) (*CommandTokenAuthSource, error) {
	parts, err := shlex.Split(commandStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token_command %q: %w", commandStr, err)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("token_command is empty")
	}
	return &CommandTokenAuthSource{
		command:  parts[0],
		args:     parts[1:],
		hostname: hostname,
	}, nil
}

func (as *CommandTokenAuthSource) Init(context.Context, *gitlab.Client) error {
	return nil
}

// Header executes the token command and returns the appropriate auth header.
// The command must write a JSON object to stdout with "type" and "token" fields.
// Valid types are "pat" (PRIVATE-TOKEN), "oauth2" (Authorization: Bearer), and
// "job-token" (JOB-TOKEN). Stderr is captured and included in error messages.
func (as *CommandTokenAuthSource) Header(ctx context.Context) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, commandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, as.command, as.args...) //nolint:gosec // command comes from user config, not user input
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = nil

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if stderrStr != "" {
			return "", "", fmt.Errorf("token_command for %q failed: %w\nstderr: %s", as.hostname, err, stderrStr)
		}
		return "", "", fmt.Errorf("token_command for %q failed: %w", as.hostname, err)
	}

	var resp commandTokenResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return "", "", fmt.Errorf("token_command for %q produced invalid JSON: %w", as.hostname, err)
	}
	if resp.Token == "" {
		return "", "", fmt.Errorf("token_command for %q returned an empty token", as.hostname)
	}

	switch resp.Type {
	case "pat":
		return "PRIVATE-TOKEN", resp.Token, nil
	case "oauth2":
		return "Authorization", "Bearer " + resp.Token, nil
	case "job-token":
		return "JOB-TOKEN", resp.Token, nil
	default:
		return "", "", fmt.Errorf("token_command for %q returned unknown token type %q (expected: pat, oauth2, job-token)", as.hostname, resp.Type)
	}
}
