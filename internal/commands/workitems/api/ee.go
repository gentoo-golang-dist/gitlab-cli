package api

import (
	"context"
	"errors"
	"strings"
	"sync"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"
)

// eeWidgetTypeNames are the GraphQL type names that appear in
// validation errors on CE instances. Substring-matching these against
// the error message is cheaper than an introspection round-trip.
var eeWidgetTypeNames = []string{
	"WorkItemWidgetStatus",
	"WorkItemWidgetLinkedItems",
	"WorkItemWidgetHealthStatus",
}

// capabilityState tracks whether EE widgets work on this instance.
// Process-wide: one probe serves every subsequent invocation.
type capabilityState struct {
	mu            sync.RWMutex
	eeUnsupported bool
}

var caps = &capabilityState{}

func eeSupported() bool {
	caps.mu.RLock()
	defer caps.mu.RUnlock()
	return !caps.eeUnsupported
}

// markEEUnsupported flips the flag after a CE probe fires.
func markEEUnsupported() {
	caps.mu.Lock()
	defer caps.mu.Unlock()
	caps.eeUnsupported = true
}

// resetCapabilities exists for tests. Production code never needs it.
func resetCapabilities() {
	caps.mu.Lock()
	defer caps.mu.Unlock()
	caps.eeUnsupported = false
}

// isEECapabilityError matches GraphQL validation errors that name one
// of our EE widget types. We match on the type name itself so wording
// drift ("Fragment on undefined type ...", "No such type ...") stays
// covered.
func isEECapabilityError(err error) bool {
	var gerr *gitlab.GraphQLResponseError
	if !errors.As(err, &gerr) {
		return false
	}
	for _, e := range gerr.Errors.Errors {
		for _, name := range eeWidgetTypeNames {
			if strings.Contains(e.Message, name) {
				return true
			}
		}
	}
	return false
}

// doWithEEFallback runs a GraphQL query. On a capability error
// naming an EE widget, it flips the process-wide flag and retries
// with EE spreads stripped. The response is zeroed between attempts
// so a partial decode can't leak through.
func doWithEEFallback[T any](
	ctx context.Context,
	client *gitlab.Client,
	response *T,
	build func(includeEE bool) gitlab.GraphQLQuery,
) error {
	includeEE := eeSupported()
	_, err := client.GraphQL.Do(build(includeEE), response, gitlab.WithContext(ctx))
	if err == nil {
		return nil
	}
	if !includeEE || !isEECapabilityError(err) {
		return err
	}

	markEEUnsupported()
	var zero T
	*response = zero

	_, err = client.GraphQL.Do(build(false), response, gitlab.WithContext(ctx))
	return err
}
