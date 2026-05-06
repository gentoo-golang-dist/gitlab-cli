// Package orbiterr translates errors returned by the OrbitService
// into structured `*cmdutils.ExitError` values with Orbit-specific
// exit codes and human-readable troubleshooting messages.
//
// It is internal to the `glab orbit` command family; the exit codes
// it defines are part of the public CLI contract and are documented
// in the `glab orbit` long help.
package orbiterr

import (
	"errors"
	"fmt"
	"net/http"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
)

// Exit codes for `glab orbit` commands.
//
// 1 is the generic glab default and is reserved for unexpected errors.
// 2..5 are Orbit-specific and map directly to HTTP status codes that
// have a stable user-facing meaning, so scripting agents can branch on
// them without parsing stderr.
const (
	// ExitOrbitUnavailable is returned when the Knowledge Graph endpoint
	// returns 404. The most common cause is the `knowledge_graph`
	// feature flag being off for the user; a typo in the path produces
	// the same status.
	ExitOrbitUnavailable = 2

	// ExitUnauthenticated is returned when the request is rejected
	// with HTTP 401 — typically an expired or missing token.
	ExitUnauthenticated = 3

	// ExitForbidden is returned when the user is authenticated but
	// not authorized — for `orbit/query`, the most common cause is
	// "no Knowledge Graph enabled namespaces available".
	ExitForbidden = 4

	// ExitRateLimited is returned when the request is rejected with
	// HTTP 429. Inspect the `Retry-After` response header and back off.
	ExitRateLimited = 5
)

// Translate maps an error returned by the OrbitService to a
// `*cmdutils.ExitError` with a human-readable message and the
// appropriate Orbit-specific exit code. Non-HTTP errors fall through
// to a generic exit code 1.
//
// The phrasings mirror the Orbit skill's `references/troubleshooting.md`
// so users see the same messages whether they invoke the API via
// `glab orbit` or via the skill's `glab api` recipes.
func Translate(err error) error {
	if err == nil {
		return nil
	}

	var errResp *gitlab.ErrorResponse
	if !errors.As(err, &errResp) || errResp.Response == nil {
		// client-go sometimes returns a bare error with the status text
		// when the response body is empty. Treat the canonical "404 Not
		// Found" string the same way as a typed 404 ErrorResponse so
		// users get the same FF-off guidance.
		if err.Error() == "404 Not Found" {
			return notFoundError()
		}
		return cmdutils.WrapError(err, "Orbit request failed")
	}

	body := errResp.Message

	switch errResp.Response.StatusCode {
	case http.StatusNotFound:
		return notFoundError()
	case http.StatusUnauthorized:
		return wrapWithDetails(
			"not authenticated",
			ExitUnauthenticated,
			"The Orbit API rejected the request with HTTP 401. Run `glab auth status`\n"+
				"to check your token, then `glab auth login` if it has expired.",
		)
	case http.StatusForbidden:
		return wrapWithDetails(
			"Knowledge Graph access denied",
			ExitForbidden,
			fmt.Sprintf("The Orbit API rejected the request with HTTP 403%s.\n"+
				"If the message mentions \"No Knowledge Graph enabled namespaces\",\n"+
				"an Owner of a top-level group you belong to must enable Orbit via\n"+
				"Orbit > Configuration in the GitLab UI.", suffix(body)),
		)
	case http.StatusTooManyRequests:
		return wrapWithDetails(
			"rate limited",
			ExitRateLimited,
			"The Orbit API rejected the request with HTTP 429. Inspect the\n"+
				"`Retry-After` response header and back off, or batch via aggregation\n"+
				"if you are running many small queries.",
		)
	}

	return cmdutils.WrapError(
		fmt.Errorf("Orbit API error (HTTP %d)%s",
			errResp.Response.StatusCode, suffix(body)),
		"",
	)
}

// notFoundError builds the user-facing error returned when an Orbit
// endpoint reports HTTP 404, regardless of whether client-go surfaced
// it as a typed `*gitlab.ErrorResponse` or a bare `404 Not Found`
// string.
func notFoundError() error {
	return wrapWithDetails(
		"Knowledge Graph endpoint not available",
		ExitOrbitUnavailable,
		"The `/api/v4/orbit/*` endpoints returned 404. The most likely cause is\n"+
			"that the `knowledge_graph` feature flag is disabled for your user on this\n"+
			"instance. Contact an instance administrator to enable it.",
	)
}

// wrapWithDetails returns a `*cmdutils.ExitError` whose underlying
// error message is `headline\n\ndetails`. We bake details into the
// error itself because Fang's `DefaultErrorHandler` only renders
// `err.Error()` and ignores `ExitError.Details`, so any troubleshooting
// text stored only in Details would be silently dropped.
func wrapWithDetails(headline string, code int, details string) *cmdutils.ExitError {
	msg := headline
	if details != "" {
		msg = headline + "\n\n" + details
	}
	return cmdutils.WrapErrorWithCode(errors.New(msg), code, "")
}

// suffix prepends ": " to a non-empty body so it can be appended to a
// status message without producing trailing punctuation when the body
// is empty.
func suffix(body string) string {
	if body == "" {
		return ""
	}
	return ": " + body
}
