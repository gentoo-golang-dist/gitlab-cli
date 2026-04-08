package authutils

import "gitlab.com/gitlab-org/cli/internal/config"

// authFieldsToClear is the set of config keys that hold authentication credentials.
// Clearing these fields removes all stored credentials for a host regardless of
// which auth method was previously used. It does not include "use_keyring", which
// is a preference setting rather than a credential and must remain set so that
// ClearAuthFields can correctly delete keyring entries during cleanup.
var authFieldsToClear = []string{
	"token",
	"job_token",
	"is_oauth2",
	"oauth2_refresh_token",
	"oauth2_expiry_date",
}

// ClearAuthFields removes all authentication-related config entries for the
// given hostname. It does not touch non-auth settings such as git_protocol,
// api_host, or api_protocol.
func ClearAuthFields(cfg config.Config, hostname string) error {
	for _, key := range authFieldsToClear {
		if err := cfg.Set(hostname, key, ""); err != nil {
			return err
		}
	}
	return nil
}
