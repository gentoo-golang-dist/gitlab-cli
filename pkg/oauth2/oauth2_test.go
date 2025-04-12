package oauth2

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/cli/pkg/glinstance"
)

func TestRefreshToken(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"access_token": "at",
			"refresh_token": "rt",
			"expiresIn": 60
		}`))
	}))

	cfg := stubConfig{
		hosts: map[string]map[string]string{},
	}

	hostname := strings.Split(svr.URL, "://")[1]
	cfg.hosts[hostname] = map[string]string{
		"is_oauth2":            "true",
		"oauth2_refresh_token": "refresh_token",
		"token":                "access_token",
		"oauth2_code_verifier": "123",
		"oauth2_expiry_date":   "13 Mar 23 15:47 GMT",
		"client_id":            "321",
	}

	err := RefreshToken(hostname, cfg, "http")
	require.Nil(t, err)

	accessToken, err := cfg.Get(hostname, "token")
	require.Nil(t, err)
	assert.Equal(t, "at", accessToken)

	refreshToken, err := cfg.Get(hostname, "oauth2_refresh_token")
	require.Nil(t, err)
	assert.Equal(t, "rt", refreshToken)

	expiryDateString, err := cfg.Get(hostname, "oauth2_expiry_date")
	require.Nil(t, err)
	_, err = time.Parse(time.RFC822, expiryDateString)
	require.Nil(t, err)
}

func TestClientID(t *testing.T) {
	// No changes needed for this test based on oauth2.go updates
	testCasesTable := []struct {
		name             string
		hostname         string
		configClientID   string
		expectedClientID string
		expectError      bool // Added for clarity based on previous version
	}{
		{
			name:             "managed",
			hostname:         glinstance.Default(),
			configClientID:   "",
			expectedClientID: glinstance.DefaultClientID(),
			expectError:      false,
		},
		{
			name:             "self-managed-complete",
			hostname:         "salsa.debian.org",
			configClientID:   "321",
			expectedClientID: "321",
			expectError:      false,
		},
		// Added the error case for completeness, preserving structure
		{
			name:             "invalid self-managed config",
			hostname:         "gitlab.example.org", // Different hostname for clarity
			configClientID:   "",                   // Missing client_id
			expectedClientID: "",
			expectError:      true,
		},
	}

	for _, testCase := range testCasesTable {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := stubConfig{
				hosts: map[string]map[string]string{
					testCase.hostname: {
						"client_id": testCase.configClientID,
					},
				},
			}
			clientID, err := oAuthClientID(cfg, testCase.hostname)

			if testCase.expectError {
				assert.Error(t, err)
				assert.Empty(t, clientID)
				// Optionally check error message contains expected text
				assert.Contains(t, err.Error(), "set 'client_id' first")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expectedClientID, clientID)
			}
		})
	}

	// Original structure had this separate Run - keep it
	t.Run("invalid self-managed config (original structure)", func(t *testing.T) {
		cfg := stubConfig{
			hosts: map[string]map[string]string{
				"salsa.debian.org": {}, // Empty config for this host
			},
		}
		clientID, err := oAuthClientID(cfg, "salsa.debian.org")
		assert.Error(t, err)
		assert.Empty(t, clientID)
		assert.Contains(t, err.Error(), "set 'client_id' first")
	})
}
