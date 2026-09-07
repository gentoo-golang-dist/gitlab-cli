package pm

import "gitlab.com/gitlab-org/cli/internal/dependencyfirewall/proxy"

// gemManager runs gem through the proxy. gem routes via HTTP(S)_PROXY and
// trusts the proxy CA via SSL_CERT_FILE; ~/.gemrc is left untouched.
//
// Known gap: a gemrc "http_proxy:" setting outranks the env pins. RubyGems
// builds its fetcher from Gem.configuration[:http_proxy] and only falls back
// to the environment when that is unset (remote_fetcher.rb). Its sources are
// the system gemrc, ~/.gemrc, or a file named by an inherited GEMRC env var —
// none of them repo-committable (there is no cwd .gemrc), so the repo-attacker
// threat this firewall centers on does not reach it; only a compromised CI
// image or pipeline env could route gem traffic around inspection this way,
// and only the --http-proxy flag (forwarded verbatim) outranks it further.
type gemManager struct{}

// Gem returns the gem package manager.
func Gem() PackageManager { return gemManager{} }

func (gemManager) Name() string   { return "gem" }
func (gemManager) Binary() string { return "gem" }

// Environment returns nil: gem routes through the universal HTTP_PROXY and
// HTTPS_PROXY the engine already sets in proxyEnviron, so there is no
// gem-specific routing variable to add.
func (gemManager) Environment(string) []string { return nil }

func (gemManager) CATrustEnviron(caPath string) []string {
	return []string{"SSL_CERT_FILE=" + caPath}
}

func (gemManager) ExistingBundleVars() []string { return []string{"SSL_CERT_FILE"} }

func (gemManager) CleanupCAFiles(string) {}

func (gemManager) Matcher() proxy.Matcher { return proxy.GemMatcher{} }
