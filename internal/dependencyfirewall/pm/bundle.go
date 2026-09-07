package pm

import "gitlab.com/gitlab-org/cli/internal/dependencyfirewall/proxy"

// bundleManager runs Bundler through the proxy. Bundler honors the lowercase
// proxy variables and SSL_CERT_FILE/BUNDLE_SSL_CA_CERT for CA trust; the
// Gemfile is left untouched.
//
// Known gap (same as gem): a gemrc "http_proxy:" setting outranks the env
// pins. Bundler applies Gem.configuration[:http_proxy] on top of its :ENV
// proxy (2.5.22 fetcher.rb). Its sources are the system gemrc, ~/.gemrc, or a
// file named by an inherited GEMRC env var — none repo-committable, so only a
// compromised CI image or pipeline env, not a committed repo file, can route
// bundle traffic around inspection this way.
type bundleManager struct{}

// Bundle returns the bundle package manager.
func Bundle() PackageManager { return bundleManager{} }

func (bundleManager) Name() string   { return "bundle" }
func (bundleManager) Binary() string { return "bundle" }

// Environment adds the lowercase proxy variables. The engine already sets
// uppercase HTTP_PROXY/HTTPS_PROXY in proxyEnviron, but Bundler resolves the
// proxy through net-http-persistent constructed with proxy: :ENV, which reads
// the lowercase http_proxy/https_proxy names (it does check both cases, so the
// lowercase pair is defense-in-depth rather than the sole routing path). A
// repo-local .bundle/config BUNDLE_HTTP_PROXY does not override these: Bundler
// never consults Bundler.settings for the proxy at all — its fetcher only uses
// the :ENV mode — so a committed BUNDLE_HTTP_PROXY is simply inert (verified
// against Bundler 2.5.22 fetcher.rb). Only the lowercase pair is added; the
// uppercase names the engine sets are not duplicated.
func (bundleManager) Environment(proxyURL string) []string {
	return []string{
		"https_proxy=" + proxyURL,
		"http_proxy=" + proxyURL,
	}
}

// CATrustEnviron trusts the proxy CA via both the generic OpenSSL SSL_CERT_FILE
// (honored by RubyGems/Net::HTTP) and Bundler's BUNDLE_SSL_CA_CERT. A repo-local
// .bundle/config could set BUNDLE_SSL_CA_CERT or BUNDLE_SSL_VERIFY_MODE. Even
// in the worst case where such local config took effect, it would swap in a
// different CA and fail the TLS handshake (fail closed), not route around the
// proxy — so CA trust cannot silently fail open (verified against Bundler 2.5).
func (bundleManager) CATrustEnviron(caPath string) []string {
	return []string{
		"SSL_CERT_FILE=" + caPath,
		"BUNDLE_SSL_CA_CERT=" + caPath,
	}
}

// ExistingBundleVars lists both CA vars CATrustEnviron overwrites so a user
// who set only BUNDLE_SSL_CA_CERT keeps their anchors, not just one who set
// SSL_CERT_FILE.
func (bundleManager) ExistingBundleVars() []string {
	return []string{"SSL_CERT_FILE", "BUNDLE_SSL_CA_CERT"}
}

func (bundleManager) CleanupCAFiles(string) {}

func (bundleManager) Matcher() proxy.Matcher { return proxy.GemMatcher{} }
