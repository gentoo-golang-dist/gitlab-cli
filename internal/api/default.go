package api

import (
	"net/http"
	"net/url"
	"strings"

	"gitlab.com/gitlab-org/cli/internal/config"
)

var DefaultListLimit int64 = 30

// MaxPerPage is the maximum number of items per page supported by the GitLab API.
// https://docs.gitlab.com/api/rest/#offset-based-pagination
const MaxPerPage = 100

// IsTokenConfigured checks if a token is configured (non-empty after trimming whitespace)
func IsTokenConfigured(token string) bool {
	return strings.TrimSpace(token) != ""
}

// ProxyFromConfig returns an http.Proxy func using a configured proxy when provided.
func ProxyFromConfig(cfg config.Config, repoHost string) (func(*http.Request) (*url.URL, error), error) {
	proxyURL, err := config.ResolveHostProxy(cfg, repoHost)
	if err != nil {
		return nil, err
	}
	if proxyURL == nil {
		return http.ProxyFromEnvironment, nil
	}
	return http.ProxyURL(proxyURL), nil
}
