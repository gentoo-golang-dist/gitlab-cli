package credentialhelper

import (
	"encoding/json"
	"fmt"
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

const tokenGracePeriod = 5 * time.Minute

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
		return errorResponse{Message: "glab is not authenticated. Use glab auth login to authenticate"}
	default:
		return errorResponse{Message: "unable to determine token"}
	}
}
