package pm

import "gitlab.com/gitlab-org/cli/internal/dependencyfirewall/proxy"

// twineManager runs twine (an upload tool) through the proxy. twine reads
// TWINE_CERT for CA trust and HTTPS_PROXY for routing; its repository URL and
// credentials are left to the user's environment.
type twineManager struct{}

// Twine returns the twine package manager.
func Twine() PackageManager { return twineManager{} }

func (twineManager) Name() string   { return "twine" }
func (twineManager) Binary() string { return "twine" }

// Environment returns nil: twine (via requests) routes through the universal
// HTTPS_PROXY the engine already sets in proxyEnviron, so there is no
// twine-specific routing variable to add.
func (twineManager) Environment(string) []string { return nil }

func (twineManager) CATrustEnviron(caPath string) []string {
	return []string{"TWINE_CERT=" + caPath}
}

// ExistingBundleVars lists both TWINE_CERT and REQUESTS_CA_BUNDLE: twine passes
// TWINE_CERT to its requests session as the CA bundle, which takes precedence
// over REQUESTS_CA_BUNDLE. Setting only TWINE_CERT would therefore drop a
// user's pre-set REQUESTS_CA_BUNDLE, so both are merged into the bundle
// TWINE_CERT points at.
func (twineManager) ExistingBundleVars() []string {
	return []string{"TWINE_CERT", "REQUESTS_CA_BUNDLE"}
}

func (twineManager) CleanupCAFiles(string) {}

func (twineManager) Matcher() proxy.Matcher { return proxy.PyPIMatcher{} }
