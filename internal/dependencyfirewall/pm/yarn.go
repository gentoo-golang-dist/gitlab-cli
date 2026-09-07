package pm

import "gitlab.com/gitlab-org/cli/internal/dependencyfirewall/proxy"

type yarnManager struct{}

// Yarn returns the Yarn (Berry) package manager. It routes through the proxy
// via YARN_HTTP(S)_PROXY and trusts the proxy CA via NODE_EXTRA_CA_CERTS.
//
// Suspected gap (unverified): Berry's per-hostname networkSettings map
// (httpProxy/httpsProxy/enableNetwork/caFilePath keyed by host) is documented
// to override the global proxy settings for matching hosts, and a map-valued
// setting has no practical env pin. If that holds, a repo-committed
// ".yarnrc.yml" networkSettings entry for the registry host could route that
// host around the MITM while staying green. This has not been confirmed
// against a Berry install (getNetworkSettings in yarnpkg-core httpUtils);
// treat it as an open item to verify and then either guard or accept.
func Yarn() PackageManager { return yarnManager{} }

func (yarnManager) Name() string   { return "yarn" }
func (yarnManager) Binary() string { return "yarn" }

func (yarnManager) Environment(proxyURL string) []string {
	// Yarn's two major lines read proxy configuration from different places, so
	// both routing families are pinned from the environment tier to keep either
	// from failing open:
	//
	//   - Berry (v2+) ignores the standard HTTP(S)_PROXY variables and honors
	//     only its own YARN_HTTP(S)_PROXY (the httpProxy/httpsProxy .yarnrc.yml
	//     keys). Without these Berry bypasses the inspection proxy entirely.
	//   - Classic (v1) reads .npmrc and, for npm backward compatibility, the
	//     npm_config_* environment variables (verified against Yarn 1.22).
	//     Classic 1.22 has no "noproxy" config key of its own — its only
	//     no-proxy handling is the NO_PROXY/no_proxy env vars, which the engine
	//     already strips — so the npm_config family is pinned mainly to keep a
	//     committed ".npmrc" proxy/https-proxy line from winning. The
	//     npm_config_noproxy pin (non-empty "localhost" sentinel, since an
	//     empty value is treated as unset) is harmless over-coverage that
	//     matches the npm manager rather than a Classic-specific hole.
	//
	// HTTPS_PROXY (uppercase) is already set by the engine's proxyEnviron; it is
	// not duplicated here.
	return []string{
		"YARN_HTTPS_PROXY=" + proxyURL,
		"YARN_HTTP_PROXY=" + proxyURL,
		"npm_config_proxy=" + proxyURL,
		"npm_config_https_proxy=" + proxyURL,
		"npm_config_noproxy=localhost",
	}
}

func (yarnManager) CATrustEnviron(caPath string) []string {
	return []string{"NODE_EXTRA_CA_CERTS=" + caPath}
}

func (yarnManager) ExistingBundleVars() []string { return []string{"NODE_EXTRA_CA_CERTS"} }

func (yarnManager) CleanupCAFiles(string) {}

func (yarnManager) Matcher() proxy.Matcher { return proxy.NPMMatcher{} }
