//go:build !integration

package oauth2

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStartDeviceFlow_missingSelfHostedClientID(t *testing.T) {
	cfg := stubConfig{
		hosts: map[string]map[string]string{
			"salsa.debian.org": {},
		},
	}

	token, err := StartDeviceFlow(t.Context(), cfg, io.Discard, http.DefaultClient, "salsa.debian.org")

	assert.Empty(t, token)
	assert.ErrorContains(t, err, "set 'client_id' first")
}
