package credentialhelper

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/oauth2"

	gitlab "gitlab.com/gitlab-org/api/client-go"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

const (
	tokenGracePeriod = 5 * time.Minute
	// skipWrapperDetectionEnv is used to prevent infinite recursion when re-invoking
	// glab through a shell wrapper (e.g., 1Password's "op plugin run -- glab")
	skipWrapperDetectionEnv = "GLAB_CREDENTIAL_HELPER_SKIP_WRAPPER"
)

type responseType any

type errorResponseType struct{}

func (errorResponseType) MarshalJSON() ([]byte, error) {
	return []byte(`"error"`), nil
}

type successResponseType struct{}

func (successResponseType) MarshalJSON() ([]byte, error) {
	return []byte(`"success"`), nil
}

type response struct {
	Type        successResponseType `json:"type"` // always evaluates to "success"
	InstanceURL string              `json:"instance_url"`
	Token       token               `json:"token"`
}

type token struct {
	Type            string    `json:"type"`
	Token           string    `json:"token"`
	ExpiryTimestamp time.Time `json:"expiry_timestamp,omitzero"`
}

type errorResponse struct {
	Type    errorResponseType `json:"type"` // always evaluates to "error"
	Message string            `json:"message"`
}

type options struct {
	baseRepo  func() (glrepo.Interface, error)
	apiClient func(repoHost string) (*api.Client, error)
}

func NewCmd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		baseRepo:  f.BaseRepo,
		apiClient: f.ApiClient,
	}

	writeResponse := func(resp responseType) error {
		io := f.IO()
		enc := json.NewEncoder(io.StdOut)
		if err := enc.Encode(resp); err != nil {
			errEnc := json.NewEncoder(io.StdErr)
			if err := errEnc.Encode(errorResponse{Message: err.Error()}); err != nil {
				return err
			}
			return cmdutils.SilentError
		}
		return nil
	}

	cmd := &cobra.Command{
		Use:    "credential-helper [flags]",
		Args:   cobra.NoArgs,
		Short:  "Implements a generic credential helper.",
		Hidden: true,
		Annotations: map[string]string{
			mcpannotations.Exclude: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			resp := opts.run()

			return writeResponse(resp)
		},
	}

	cmdutils.EnableRepoOverride(cmd, f)
	// NOTE: this is a hack to ensure the JSON protocol for the hook that EnableRepoOverride added.
	repoOverridePersistentPreRunE := cmd.PersistentPreRunE
	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		err := repoOverridePersistentPreRunE(cmd, args)
		if err != nil {
			_ = writeResponse(errorResponse{Message: err.Error()})
			// We need to signal cobra that we want to error, but that cobra shouldn't log anything else.
			// This silent error is the way to go here.
			return cmdutils.SilentError
		}
		return nil
	}

	return cmd
}

// detectGlabWrapper checks if a credential wrapper (like 1Password) is available
// and returns the wrapper command if found.
// Returns empty string if no wrapper is detected.
func detectGlabWrapper() string {
	if has1PasswordGlabPlugin() {
		return "op plugin run -- glab"
	}
	return ""
}

// has1PasswordGlabPlugin checks if the 1Password CLI is available and
// the GitLab plugin is configured.
func has1PasswordGlabPlugin() bool {
	// Skip on Windows for now (different command semantics)
	if runtime.GOOS == "windows" {
		return false
	}

	// Check if 'op' command exists
	cmd := exec.Command("op", "--version")
	if err := cmd.Run(); err != nil {
		return false // op not installed or not in PATH
	}

	// Check if GitLab plugin is configured by listing plugins
	cmd = exec.Command("op", "plugin", "list")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Check if gitlab (or glab) appears in the plugin list
	outputStr := strings.ToLower(string(output))
	return strings.Contains(outputStr, "gitlab") || strings.Contains(outputStr, "glab")
}

// tryWrappedInvocation attempts to invoke the credential helper through a credential
// wrapper (like 1Password's "op plugin run -- glab"). This allows glab to work with
// credential providers that inject tokens via environment variables.
func (o *options) tryWrappedInvocation() responseType {
	// Detect if a credential wrapper is available
	wrapperCmd := detectGlabWrapper()
	if wrapperCmd == "" {
		return errorResponse{Message: "glab is not authenticated. Use glab auth login to authenticate"}
	}

	// Build the command to invoke through the wrapper (e.g., "op plugin run -- glab auth credential-helper")
	cmdStr := wrapperCmd + " auth credential-helper"

	// Set environment variable to prevent infinite recursion
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Env = append(os.Environ(), skipWrapperDetectionEnv+"=1")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// If the wrapped invocation fails, include stderr for debugging
		errMsg := fmt.Sprintf("failed to get credentials through wrapper: %v", err)
		if stderr.Len() > 0 {
			errMsg += fmt.Sprintf(": %s", stderr.String())
		}
		return errorResponse{Message: errMsg}
	}

	// Parse the JSON response from the wrapped invocation
	// We need to parse into a generic map first since the response type has custom unmarshaling
	var rawResp map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &rawResp); err != nil {
		return errorResponse{
			Message: fmt.Sprintf("failed to parse credentials from wrapper: %v", err),
		}
	}

	// Check if it's an error response
	if respType, ok := rawResp["type"].(string); ok && respType == "error" {
		if msg, ok := rawResp["message"].(string); ok {
			return errorResponse{Message: msg}
		}
		return errorResponse{Message: "unknown error from wrapper"}
	}

	// Extract the fields we need from the success response
	instanceURL, _ := rawResp["instance_url"].(string)
	tokenData, ok := rawResp["token"].(map[string]any)
	if !ok {
		return errorResponse{Message: "malformed response from wrapper: missing token data"}
	}

	tokenType, ok := tokenData["type"].(string)
	if !ok || tokenType == "" {
		return errorResponse{Message: "malformed response from wrapper: invalid token type"}
	}

	tokenValue, ok := tokenData["token"].(string)
	if !ok || tokenValue == "" {
		return errorResponse{Message: "malformed response from wrapper: invalid token value"}
	}

	return response{
		InstanceURL: instanceURL,
		Token: token{
			Type:  tokenType,
			Token: tokenValue,
		},
	}
}

func (o *options) run() responseType {
	baseRepo, err := o.baseRepo()
	host := "" // NOTE: an empty host is the default configured host
	if err == nil {
		host = baseRepo.RepoHost()
	}

	apiClient, err := o.apiClient(host)
	if err != nil {
		return errorResponse{Message: err.Error()}
	}

	// NOTE: the API client ensures this suffix via glinstance.APIEndpoint().
	instanceURL := strings.TrimSuffix(apiClient.BaseURL(), "/api/v4/")

	switch as := apiClient.AuthSource().(type) {
	case gitlab.OAuthTokenSource:
		// Trying to refresh access token
		tokenSource := oauth2.ReuseTokenSourceWithExpiry(nil, as.TokenSource, tokenGracePeriod)
		oauth2Token, err := tokenSource.Token()
		if err != nil {
			return errorResponse{Message: fmt.Sprintf("failed to refresh token for %q: %v", host, err)}
		}

		return response{
			InstanceURL: instanceURL,
			Token: token{
				Type:            "oauth2",
				Token:           oauth2Token.AccessToken,
				ExpiryTimestamp: oauth2Token.Expiry.UTC(),
			},
		}
	case gitlab.JobTokenAuthSource:
		return response{
			InstanceURL: instanceURL,
			Token: token{
				Type:  "job-token",
				Token: as.Token,
			},
		}
	case gitlab.AccessTokenAuthSource:
		return response{
			InstanceURL: instanceURL,
			Token: token{
				Type:  "pat",
				Token: as.Token,
			},
		}
	case gitlab.Unauthenticated:
		// If unauthenticated and we haven't tried wrapper detection yet,
		// attempt to invoke through a credential wrapper (e.g., 1Password)
		if os.Getenv(skipWrapperDetectionEnv) != "1" {
			return o.tryWrappedInvocation()
		}
		return errorResponse{Message: "glab is not authenticated. Use glab auth login to authenticate"}
	default:
		return errorResponse{Message: "unable to determine token"}
	}
}
