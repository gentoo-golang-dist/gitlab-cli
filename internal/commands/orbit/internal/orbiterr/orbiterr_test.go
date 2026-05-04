//go:build !integration

package orbiterr

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
)

func TestTranslate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		err         error
		wantCode    int
		wantMsg     string
		wantDetails string // substring expected in err.Error() after the headline
	}{
		{
			name: "404 maps to ExitOrbitUnavailable",
			err: &gitlab.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusNotFound},
				Message:  "404 Not Found",
			},
			wantCode:    ExitOrbitUnavailable,
			wantMsg:     "Knowledge Graph endpoint not available",
			wantDetails: "knowledge_graph` feature flag",
		},
		{
			name: "401 maps to ExitUnauthenticated",
			err: &gitlab.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusUnauthorized},
				Message:  "401 Unauthorized",
			},
			wantCode:    ExitUnauthenticated,
			wantMsg:     "not authenticated",
			wantDetails: "glab auth status",
		},
		{
			name: "403 maps to ExitForbidden",
			err: &gitlab.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusForbidden},
				Message:  "No Knowledge Graph enabled namespaces available",
			},
			wantCode:    ExitForbidden,
			wantMsg:     "Knowledge Graph access denied",
			wantDetails: "No Knowledge Graph enabled namespaces",
		},
		{
			name: "429 maps to ExitRateLimited",
			err: &gitlab.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusTooManyRequests},
				Message:  "Too Many Requests",
			},
			wantCode:    ExitRateLimited,
			wantMsg:     "rate limited",
			wantDetails: "Retry-After",
		},
		{
			name: "500 falls through to generic exit code 1",
			err: &gitlab.ErrorResponse{
				Response: &http.Response{StatusCode: http.StatusInternalServerError},
				Message:  "boom",
			},
			wantCode: 1,
			wantMsg:  "Orbit API error (HTTP 500): boom",
		},
		{
			name:     "non-HTTP error falls through to generic exit code 1",
			err:      errors.New("network broken"),
			wantCode: 1,
			wantMsg:  "network broken",
		},
		{
			name: "bare \"404 Not Found\" string also maps to ExitOrbitUnavailable",
			// client-go surfaces an empty-body 404 as a plain error
			// string. Treat it like a typed 404 ErrorResponse so the
			// FF-off guidance reaches the user either way.
			err:         errors.New("404 Not Found"),
			wantCode:    ExitOrbitUnavailable,
			wantMsg:     "Knowledge Graph endpoint not available",
			wantDetails: "knowledge_graph` feature flag",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// WHEN translating the error
			out := Translate(tc.err)

			// THEN it is a *cmdutils.ExitError with the expected code
			require.Error(t, out)
			var exitErr *cmdutils.ExitError
			require.True(t, errors.As(out, &exitErr), "expected *cmdutils.ExitError, got %T", out)
			assert.Equal(t, tc.wantCode, exitErr.Code)

			// AND the headline is present in err.Error()
			assert.Contains(t, out.Error(), tc.wantMsg)

			// AND any troubleshooting details are baked into err.Error()
			// itself, because Fang's DefaultErrorHandler ignores the
			// ExitError.Details field.
			if tc.wantDetails != "" {
				assert.Contains(t, out.Error(), tc.wantDetails,
					"details must be part of err.Error() so Fang renders them")
			}
		})
	}
}

func TestTranslate_NilReturnsNil(t *testing.T) {
	t.Parallel()
	assert.NoError(t, Translate(nil))
}
