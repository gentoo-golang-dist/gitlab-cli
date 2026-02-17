package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"

	"gitlab.com/gitlab-org/cli/internal/glinstance"
)

//go:generate go run gen.go

const (
	defaultGitProtocol  = "ssh"
	defaultGlamourStyle = "dark"
	defaultHostname     = "gitlab.com"
	defaultAPIProtocol  = "https"
)

// keyringEligibleKeys is a map of config keys that can be stored in the keyring
var keyringEligibleKeys = map[string]struct{}{
	"token":                {},
	"oauth2_refresh_token": {},
	"job_token":            {},
}

// A Config reads and writes persistent configuration for glab.
type Config interface {
	Get(string, string) (string, error)
	GetWithSource(string, string, bool) (string, string, error)
	Set(string, string, string) error
	Hosts() ([]string, error)
	Aliases() (*AliasConfig, error)
	Local() (*LocalConfig, error)
	// Write writes to the config.yml file
	Write() error
	// WriteAll saves all the available configuration file types
	WriteAll() error
}

// NotFoundError is returned when a config entry is not found.
type NotFoundError struct {
	error
}

func isNotFoundError(err error) bool {
	var nfe *NotFoundError
	return errors.As(err, &nfe)
}

// HostConfig represents the configuration for a single host.
type HostConfig struct {
	ConfigMap
	Host string
}

// ConfigMap type implements a low-level get/set config that is backed by an in-memory tree of YAML
// nodes. It allows us to interact with a YAML-based config programmatically, preserving any
// comments that were present when the YAML was parsed.
type ConfigMap struct {
	Root *yaml.Node
}

func (cm *ConfigMap) Empty() bool {
	return cm.Root == nil || len(cm.Root.Content) == 0
}

func (cm *ConfigMap) GetStringValue(key string) (string, error) {
	entry, err := cm.FindEntry(key)
	if err != nil {
		return "", err
	}
	return entry.ValueNode.Value, nil
}

func (cm *ConfigMap) SetStringValue(key, value string) error {
	entry, err := cm.FindEntry(key)

	valueNode := entry.ValueNode

	if err != nil && isNotFoundError(err) {
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: key,
		}
		valueNode = &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: "",
		}

		cm.Root.Content = append(cm.Root.Content, keyNode, valueNode)
	} else if err != nil {
		return err
	}

	valueNode.Value = value

	return nil
}

type ConfigEntry struct {
	KeyNode   *yaml.Node
	ValueNode *yaml.Node
	Index     int
}

func (cm *ConfigMap) FindEntry(key string) (*ConfigEntry, error) {
	ce := &ConfigEntry{}

	topLevelKeys := cm.Root.Content
	for i, v := range topLevelKeys {
		if v.Value == key {
			ce.KeyNode = v
			ce.Index = i
			if i+1 < len(topLevelKeys) {
				ce.ValueNode = topLevelKeys[i+1]
			}
			return ce, nil
		}
	}

	return ce, &NotFoundError{errors.New("not found")}
}

func (cm *ConfigMap) RemoveEntry(key string) {
	var newContent []*yaml.Node

	content := cm.Root.Content
	for i := 0; i < len(content); i++ {
		if content[i].Value == key {
			i++ // skip the next node which is this key's value
		} else {
			newContent = append(newContent, content[i])
		}
	}

	cm.Root.Content = newContent
}

func NewConfig(root *yaml.Node) Config {
	return &fileConfig{
		ConfigMap:    ConfigMap{Root: root.Content[0]},
		documentRoot: root,
	}
}

// NewFromString initializes a Config from a yaml string
func NewFromString(str string) Config {
	root, err := parseConfigData([]byte(str))
	if err != nil {
		panic(err)
	}
	return NewConfig(root)
}

// NewBlankConfig initializes a config file pre-populated with comments and default values
func NewBlankConfig() Config {
	return NewConfig(NewBlankRoot())
}

func NewBlankRoot() *yaml.Node {
	return rootConfig()
}

// A fileConfig reads and writes glab configuration to a file on disk.
type fileConfig struct {
	ConfigMap
	documentRoot *yaml.Node
}

func (c *fileConfig) Root() *yaml.Node {
	return c.ConfigMap.Root
}

func (c *fileConfig) Get(hostname, key string) (string, error) {
	val, _, err := c.GetWithSource(hostname, key, true)
	return val, err
}

func (c *fileConfig) GetWithSource(hostname, key string, searchENVVars bool) (string, string, error) {
	if searchENVVars {
		value, source := GetFromEnvWithSource(key)
		if value != "" {
			return value, source, nil
		}
	}

	key = ConfigKeyEquivalence(key)

	var cfgError error

	if hostname != "" {
		hostCfg, err := c.configForHost(hostname)
		if err != nil && !isNotFoundError(err) {
			return "", "", err
		}

		var hostValue string
		if hostCfg != nil {
			// Check if use_keyring field is enabled for token keys
			if isKeyringEligibleKey(key) {
				useKeyring, _ := hostCfg.GetStringValue("use_keyring")

				if useKeyring == "true" {
					// Keyring enabled - retrieve from platform-native secure storage
					token, err := getFromKeyring(hostname, key)
					if err == nil {
						return token, "keyring", nil
					}
					return "", "", fmt.Errorf("%s not found in keyring", key)
				}
			}

			hostValue, err = hostCfg.GetStringValue(key)
			if err != nil && !isNotFoundError(err) {
				return "", "", err
			}

			// Fallback: check keyring if token not in config (backward compat for existing users)
			// Only check for PAT (token) and OAuth2 refresh tokens, not job_token, since job tokens
			// were not commonly stored in keyring historically and the legacy format is ambiguous.
			if (err != nil || hostValue == "") && (key == "token" || key == "oauth2_refresh_token") {
				token, err := getFromKeyring(hostname, key)
				if err == nil {
					return token, "keyring", nil
				}
			}

		}

		if hostValue != "" {
			return hostValue, ConfigFile(), nil
		}
	}

	source := ConfigFile()

	l, _ := c.Local()
	value, err := l.GetStringValue(key)

	if (err != nil && isNotFoundError(err)) || value == "" {
		value, err = c.GetStringValue(key)
		if err != nil && isNotFoundError(err) {
			return defaultFor(key), source, cfgError
		} else if err != nil {
			if hostname != "" {
				err = cfgError
			}
			return "", LocalConfigFile(), err
		}
	} else if value != "" {
		source = LocalConfigFile()
	}

	if value == "" {
		return defaultFor(key), source, cfgError
	}

	return value, source, cfgError
}

// isKeyringEligibleKey returns true if the key can be stored in keyring
func isKeyringEligibleKey(key string) bool {
	_, eligible := keyringEligibleKeys[key]
	return eligible
}

// buildKeyringKey constructs the keyring key for a given hostname and config key
func buildKeyringKey(hostname, key string) string {
	// Always suffix with the key name to avoid collisions
	// e.g., "glab:gitlab.com:token", "glab:gitlab.com:oauth2_refresh_token"
	return "glab:" + hostname + ":" + key
}

// getFromKeyring attempts to retrieve a value from the keyring, trying both
// new and legacy key formats for backward compatibility.
func getFromKeyring(hostname, key string) (string, error) {
	// Try new format first
	keyringKey := buildKeyringKey(hostname, key)
	token, err := keyring.Get(keyringKey, "")
	if err == nil {
		return token, nil
	}

	// Fallback to legacy key format for backward compatibility (if one exists)
	legacyKey := buildLegacyKeyringKey(hostname, key)
	if legacyKey != "" {
		return keyring.Get(legacyKey, "")
	}

	// No legacy format exists for this key type
	return "", err
}

// buildLegacyKeyringKey constructs the old keyring key format for backward compatibility.
// Legacy format used "glab:hostname" for token (PAT only)
// and "glab:hostname:refresh_token" for oauth2_refresh_token.
// Returns empty string for keys that did not have a legacy format (e.g., job_token).
// Note: The old code used "glab:hostname" ambiguously for both token and job_token,
// but we only check it for token (PAT) since that was the common use case for keyring.
func buildLegacyKeyringKey(hostname, key string) string {
	if key == "oauth2_refresh_token" {
		return "glab:" + hostname + ":refresh_token"
	}
	if key == "token" {
		// Only PATs were commonly stored in keyring
		return "glab:" + hostname
	}
	// No legacy format for job_token and other keys
	return ""
}

func (c *fileConfig) Set(hostname, key, value string) error {
	key = ConfigKeyEquivalence(key)

	// Check if this is a keyring-eligible key and keyring is enabled
	if isKeyringEligibleKey(key) && hostname != "" {
		useKeyring, _ := c.Get(hostname, "use_keyring")
		if useKeyring == "true" {
			if value != "" {
				// Store in keyring instead of config file
				keyringKey := buildKeyringKey(hostname, key)
				if err := keyring.Set(keyringKey, "", value); err != nil {
					return err
				}
				// Remove any existing plaintext token from config
				// Set value to empty to trigger removal below
				value = ""
			} else {
				// Delete from keyring when value is empty
				// Try both new and legacy formats for thorough cleanup
				keyringKey := buildKeyringKey(hostname, key)
				_ = keyring.Delete(keyringKey, "")

				legacyKey := buildLegacyKeyringKey(hostname, key)
				_ = keyring.Delete(legacyKey, "")

				// Also remove from config file below
			}
		}
	}

	var cfg interface {
		SetStringValue(string, string) error
		RemoveEntry(string)
	}

	switch hostname {
	case "":
		cfg = c
	default:
		var err error
		cfg, err = c.configForHost(hostname)
		if err != nil {
			if isNotFoundError(err) {
				cfg = c.makeConfigForHost(hostname)
				break
			}
			return err
		}
	}

	switch value {
	case "":
		cfg.RemoveEntry(key)
		return nil
	default:
		return cfg.SetStringValue(key, value)
	}
}

func (c *fileConfig) Write() error {
	mainData := yaml.Node{Kind: yaml.MappingNode}

	nodes := c.documentRoot.Content[0].Content
	for i := 0; i < len(nodes)-1; i += 2 {
		if nodes[i].Value == "aliases" || nodes[i].Value == "local" {
			continue
		} else {
			mainData.Content = append(mainData.Content, nodes[i], nodes[i+1])
		}
	}

	mainBytes, err := yaml.Marshal(&mainData)
	if err != nil {
		return err
	}

	filename := ConfigFile()
	return WriteConfigFile(filename, yamlNormalize(mainBytes))
}

func (c *fileConfig) WriteAll() error {
	err := c.Write()
	if err != nil {
		return err
	}

	aliases, err := c.Aliases()
	if err != nil {
		return err
	}
	return aliases.Write()
}

func yamlNormalize(b []byte) []byte {
	if bytes.Equal(b, []byte("{}\n")) {
		return []byte{}
	}
	return b
}

func (c *fileConfig) Local() (*LocalConfig, error) {
	entry, err := c.FindEntry("local")
	notFound := isNotFoundError(err)
	if err != nil && !notFound {
		return nil, err
	}

	var toInsert []*yaml.Node

	keyNode := entry.KeyNode
	valueNode := entry.ValueNode

	if keyNode == nil {
		keyNode = &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "local",
		}
		toInsert = append(toInsert, keyNode)
	}

	if valueNode == nil || valueNode.Kind != yaml.MappingNode {
		valueNode = &yaml.Node{
			Kind:  yaml.MappingNode,
			Value: "",
		}
		toInsert = append(toInsert, valueNode)
	}

	if len(toInsert) > 0 {
		var newContent []*yaml.Node
		if notFound {
			newContent = append(c.Root().Content, keyNode, valueNode)
		} else {
			for i := 0; i < len(c.Root().Content); i++ {
				if i == entry.Index {
					newContent = append(newContent, keyNode, valueNode)
					i++
				} else {
					newContent = append(newContent, c.Root().Content[i])
				}
			}
		}
		c.Root().Content = newContent
	}
	return &LocalConfig{
		Parent:    c,
		ConfigMap: ConfigMap{Root: valueNode},
	}, nil
}

func (c *fileConfig) Aliases() (*AliasConfig, error) {
	// The complexity here is for dealing with either a missing or empty aliases key. It's something
	// we'll likely want for other config sections at some point.
	entry, err := c.FindEntry("aliases")
	notFound := isNotFoundError(err)
	if err != nil && !notFound {
		return nil, err
	}

	var toInsert []*yaml.Node

	keyNode := entry.KeyNode
	valueNode := entry.ValueNode

	if keyNode == nil {
		keyNode = &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "aliases",
		}
		toInsert = append(toInsert, keyNode)
	}

	if valueNode == nil || valueNode.Kind != yaml.MappingNode {
		valueNode = &yaml.Node{
			Kind:  yaml.MappingNode,
			Value: "",
		}
		toInsert = append(toInsert, valueNode)
	}

	if len(toInsert) > 0 {
		var newContent []*yaml.Node
		if notFound {
			newContent = append(c.Root().Content, keyNode, valueNode)
		} else {
			for i := 0; i < len(c.Root().Content); i++ {
				if i == entry.Index {
					newContent = append(newContent, keyNode, valueNode)
					i++
				} else {
					newContent = append(newContent, c.Root().Content[i])
				}
			}
		}
		c.Root().Content = newContent
	}

	return &AliasConfig{
		Parent:    c,
		ConfigMap: ConfigMap{Root: valueNode},
	}, nil
}

func (c *fileConfig) hostEntries() ([]*HostConfig, error) {
	entry, err := c.FindEntry("hosts")
	if err != nil {
		return nil, fmt.Errorf("could not find hosts config: %w", err)
	}

	hostConfigs, err := c.parseHosts(entry.ValueNode)
	if err != nil {
		return nil, fmt.Errorf("could not parse hosts config: %w", err)
	}

	return hostConfigs, nil
}

// Hosts returns a list of all known hostnames configured in hosts.yml
func (c *fileConfig) Hosts() ([]string, error) {
	entries, err := c.hostEntries()
	if err != nil {
		return nil, err
	}

	var hostnames []string
	for _, entry := range entries {
		hostnames = append(hostnames, entry.Host)
	}

	sort.SliceStable(hostnames, func(i, j int) bool { return hostnames[i] == glinstance.DefaultHostname })

	return hostnames, nil
}

func (c *fileConfig) makeConfigForHost(hostname string) *HostConfig {
	hostRoot := &yaml.Node{Kind: yaml.MappingNode}
	hostCfg := &HostConfig{
		Host:      hostname,
		ConfigMap: ConfigMap{Root: hostRoot},
	}
	hostsEntry, err := c.FindEntry("hosts")
	if isNotFoundError(err) {
		hostsEntry.KeyNode = &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "hosts",
		}
		hostsEntry.ValueNode = &yaml.Node{Kind: yaml.MappingNode}
		root := c.Root()
		root.Content = append(root.Content, hostsEntry.KeyNode, hostsEntry.ValueNode)
	} else if err != nil {
		panic(err)
	}

	hostsEntry.ValueNode.Content = append(hostsEntry.ValueNode.Content,
		&yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: hostname,
		}, hostRoot)

	return hostCfg
}

func (c *fileConfig) parseHosts(hostsEntry *yaml.Node) ([]*HostConfig, error) {
	var hostConfigs []*HostConfig

	for i := 0; i < len(hostsEntry.Content)-1; i = i + 2 {
		hostname := hostsEntry.Content[i].Value
		hostRoot := hostsEntry.Content[i+1]
		hostConfig := HostConfig{
			ConfigMap: ConfigMap{Root: hostRoot},
			Host:      hostname,
		}
		hostConfigs = append(hostConfigs, &hostConfig)
	}

	if len(hostConfigs) == 0 {
		return nil, &NotFoundError{errors.New("could not find any host configurations")}
	}

	return hostConfigs, nil
}

// GetFromEnv is just a wrapper for os.GetEnv but checks for matching names used in previous glab versions and
// retrieves the value of the environment if any of the matching names have been set.
// It returns the value, which will be empty if the variable is not present.
func GetFromEnv(key string) string {
	value, _ := GetFromEnvWithSource(key)
	return value
}

// GetFromEnvWithSource works like GetFromEnv but also returns the name of the environment variable that was
// set as the source.
func GetFromEnvWithSource(key string) (string, string) {
	envEq := EnvKeyEquivalence(key)
	for _, e := range envEq {
		if val := os.Getenv(e); val != "" {
			return val, e
		}
	}
	return "", ""
}
