package pm

import "gitlab.com/gitlab-org/cli/internal/dependencyfirewall/proxy"

// uvManager runs uv through the proxy. uv uses rustls and reads SSL_CERT_FILE
// for CA trust and HTTPS_PROXY for routing; its index configuration is left
// untouched.
type uvManager struct{}

// Uv returns the uv package manager.
func Uv() PackageManager { return uvManager{} }

func (uvManager) Name() string   { return "uv" }
func (uvManager) Binary() string { return "uv" }

// Environment returns nil: uv routes through the universal HTTPS_PROXY the
// engine already sets in proxyEnviron, so there is no uv-specific routing
// variable to add.
func (uvManager) Environment(string) []string { return nil }

func (uvManager) CATrustEnviron(caPath string) []string {
	return []string{"SSL_CERT_FILE=" + caPath}
}

func (uvManager) ExistingBundleVars() []string { return []string{"SSL_CERT_FILE"} }

func (uvManager) CleanupCAFiles(string) {}

func (uvManager) Matcher() proxy.Matcher { return proxy.PyPIMatcher{} }
