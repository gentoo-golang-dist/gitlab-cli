//go:build !integration

package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

// resetCapsForTest wipes the package-level capability flag between
// cases so tests don't leak state.
func resetCapsForTest(t *testing.T) {
	t.Helper()
	resetCapabilities()
	t.Cleanup(resetCapabilities)
}

// newGraphQLTestClient stands up an httptest server that runs handler
// on every POST to /api/graphql and wires a real gitlab.Client at it.
// That's enough to exercise SDK error wrapping end-to-end.
func newGraphQLTestClient(t *testing.T, handler http.HandlerFunc) *gitlab.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/graphql" {
			http.NotFound(w, r)
			return
		}
		handler(w, r)
	}))
	t.Cleanup(srv.Close)

	client, err := gitlab.NewClient("test-token", gitlab.WithBaseURL(srv.URL+"/api/v4"))
	require.NoError(t, err)
	return client
}

// eeUndefinedTypeError is the body GitLab returns when a fragment
// names a missing type, e.g. WorkItemWidgetStatus on CE.
const eeUndefinedTypeError = `{
	"errors": [
		{"message": "Fragment on undefined type 'WorkItemWidgetStatus'"}
	]
}`

const successResponse = `{
	"data": {
		"group": {
			"workItems": {
				"nodes": [],
				"pageInfo": { "endCursor": "", "hasNextPage": false }
			}
		}
	}
}`

// TestEEFallback_RetriesAndMarksUnsupported simulates CE: the
// first call rejects an EE widget, the retry (stripped) succeeds,
// and the capability flag stays flipped.
func TestEEFallback_RetriesAndMarksUnsupported(t *testing.T) {
	resetCapsForTest(t)

	var callCount atomic.Int32
	client := newGraphQLTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		n := callCount.Add(1)
		bodyBytes, _ := io.ReadAll(r.Body)
		bodyStr := string(bodyBytes)

		// First call: should include the EE widget spread. Reject it.
		// Second call: the EE spread should have been stripped.
		if n == 1 {
			assert.Contains(t, bodyStr, "WorkItemWidgetStatus",
				"first attempt should ask for the EE widget")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(eeUndefinedTypeError))
			return
		}
		assert.NotContains(t, bodyStr, "WorkItemWidgetStatus",
			"retry should strip the EE widget")
		_, _ = w.Write([]byte(successResponse))
	})

	var response WorkItemsResponse
	err := doWithEEFallback(t.Context(), client, &response, func(includeEE bool) gitlab.GraphQLQuery {
		return gitlab.GraphQLQuery{
			Query: groupWorkItemsQuery(includeEE),
			Variables: map[string]any{
				"groupPath": "gitlab-org",
			},
		}
	})
	require.NoError(t, err)
	assert.EqualValues(t, 2, callCount.Load(), "exactly one retry expected")
	assert.False(t, eeSupported(), "capability flag should be flipped after the probe")
}

// TestEEFallback_StaysOnCEAfterFirstProbe confirms the flipped flag
// sticks: later calls skip the EE spread with no retry.
func TestEEFallback_StaysOnCEAfterFirstProbe(t *testing.T) {
	resetCapsForTest(t)
	markEEUnsupported()

	var callCount atomic.Int32
	client := newGraphQLTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		bodyBytes, _ := io.ReadAll(r.Body)
		assert.NotContains(t, string(bodyBytes), "WorkItemWidgetStatus",
			"after the probe, EE spread should be pre-stripped")
		_, _ = w.Write([]byte(successResponse))
	})

	var response WorkItemsResponse
	err := doWithEEFallback(t.Context(), client, &response, func(includeEE bool) gitlab.GraphQLQuery {
		return gitlab.GraphQLQuery{
			Query:     groupWorkItemsQuery(includeEE),
			Variables: map[string]any{"groupPath": "gitlab-org"},
		}
	})
	require.NoError(t, err)
	assert.EqualValues(t, 1, callCount.Load(), "no retry expected once flag is set")
}

// TestEEFallback_NonCapabilityErrorPropagates confirms unrelated
// errors (auth, missing scope, field validation) surface as-is
// without flipping the EE flag.
func TestEEFallback_NonCapabilityErrorPropagates(t *testing.T) {
	resetCapsForTest(t)

	unrelatedError := `{"errors": [{"message": "Unauthorized: token invalid"}]}`
	var callCount atomic.Int32
	client := newGraphQLTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(unrelatedError))
	})

	var response WorkItemsResponse
	err := doWithEEFallback(t.Context(), client, &response, func(includeEE bool) gitlab.GraphQLQuery {
		return gitlab.GraphQLQuery{
			Query:     groupWorkItemsQuery(includeEE),
			Variables: map[string]any{"groupPath": "gitlab-org"},
		}
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Unauthorized")
	assert.EqualValues(t, 1, callCount.Load(), "unrelated errors should not trigger retry")
	assert.True(t, eeSupported(), "unrelated errors must not flip the EE flag")
}

// TestIsEECapabilityError covers the error-pattern matcher directly.
func TestIsEECapabilityError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		err    error
		expect bool
	}{
		{
			"undefined Status widget type",
			&gitlab.GraphQLResponseError{
				Err: assertAnError(),
				Errors: gitlab.GenericGraphQLErrors{Errors: []struct {
					Message string `json:"message"`
				}{{Message: "Fragment on undefined type 'WorkItemWidgetStatus'"}}},
			},
			true,
		},
		{
			"undefined LinkedItems type",
			&gitlab.GraphQLResponseError{
				Err: assertAnError(),
				Errors: gitlab.GenericGraphQLErrors{Errors: []struct {
					Message string `json:"message"`
				}{{Message: "No such type WorkItemWidgetLinkedItems"}}},
			},
			true,
		},
		{
			"unrelated validation error",
			&gitlab.GraphQLResponseError{
				Err: assertAnError(),
				Errors: gitlab.GenericGraphQLErrors{Errors: []struct {
					Message string `json:"message"`
				}{{Message: "Field 'foo' doesn't exist on type 'Project'"}}},
			},
			false,
		},
		{
			"plain non-graphql error",
			assertAnError(),
			false,
		},
		{
			"nil",
			nil,
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expect, isEECapabilityError(tc.err))
		})
	}
}

// assertAnError returns a stable non-GraphQL error for the matcher
// test table. Using assert.AnError pulls testify into the ee.go
// production path unnecessarily; this small helper avoids that.
func assertAnError() error { return errWrap("generic failure") }

type errWrap string

func (e errWrap) Error() string { return string(e) }
