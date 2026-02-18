package config

import (
	"fmt"
	"net/url"
	"strings"
)

// resolveHostProxy retrieves the proxy value for a given host, if set.
func (c *fileConfig) resolveHostProxy(hostname string) (*url.URL, error) {
	hostCfg, err := c.configForHost(hostname)
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	value, err := hostCfg.GetStringValue("proxy")
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, err
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy URL %q: %w", trimmed, err)
	}

	return parsed, nil
}

// ResolveHostProxy exposes per-host proxy resolution for consumers.
func ResolveHostProxy(cfg Config, hostname string) (*url.URL, error) {
	fc, ok := cfg.(*fileConfig)
	if !ok {
		return nil, fmt.Errorf("unexpected config type: %T, expected *fileConfig", cfg)
	}

	return fc.resolveHostProxy(hostname)
}
