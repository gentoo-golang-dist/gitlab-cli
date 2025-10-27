package config

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"syscall"

	"github.com/adrg/xdg"
	"gopkg.in/yaml.v3"
)

var (
	cachedConfig Config
	configError  error
)

// ConfigDir returns the config directory for writing configuration.
// It respects GLAB_CONFIG_DIR as the highest priority override,
// otherwise uses XDG_CONFIG_HOME (defaulting to ~/.config).
func ConfigDir() string {
	glabDir := os.Getenv("GLAB_CONFIG_DIR")
	if glabDir != "" {
		return glabDir
	}
	return filepath.Join(xdg.ConfigHome, "glab-cli")
}

// ConfigFile returns the config file path.
// It respects GLAB_CONFIG_DIR as the highest priority override,
// otherwise returns the XDG-compliant user config file path.
// This function only determines the path without creating directories.
func ConfigFile() string {
	return filepath.Join(ConfigDir(), "config.yml")
}

// SearchConfigFile searches for an existing config file across all XDG config paths.
// It respects GLAB_CONFIG_DIR as the highest priority override.
// Search order:
// 1. $GLAB_CONFIG_DIR/config.yml (if GLAB_CONFIG_DIR is set)
// 2. $XDG_CONFIG_HOME/glab-cli/config.yml (default: ~/.config/glab-cli/config.yml)
// 3. $XDG_CONFIG_DIRS/glab-cli/config.yml (default: /etc/xdg/glab-cli/config.yml)
//
// Returns the path to the first config file found, or an error if none exist.
func SearchConfigFile() (string, error) {
	// HIGHEST PRIORITY: GLAB_CONFIG_DIR completely bypasses XDG
	if os.Getenv("GLAB_CONFIG_DIR") != "" {
		configPath := ConfigFile()
		if _, err := os.Stat(configPath); err == nil {
			return configPath, nil
		}
		// If GLAB_CONFIG_DIR is set but file doesn't exist,
		// still return this path (don't fall through to XDG)
		return configPath, os.ErrNotExist
	}

	// XDG search: user config → system configs
	configPath, err := xdg.SearchConfigFile("glab-cli/config.yml")
	if err != nil {
		return "", err
	}
	return configPath, nil
}

// SearchAllConfigFiles searches for all existing config files across XDG config paths.
// It returns config files in order from lowest to highest priority (system → user),
// so later configs should override earlier ones during merging.
// Returns nil if GLAB_CONFIG_DIR is set (no merging in that mode).
//
// Search order:
// 1. System configs from XDG_CONFIG_DIRS (e.g., /etc/xdg/glab-cli/config.yml)
// 2. User config from XDG_CONFIG_HOME (e.g., ~/.config/glab-cli/config.yml)
func SearchAllConfigFiles() []string {
	// When GLAB_CONFIG_DIR is set, we don't merge - return nil
	if os.Getenv("GLAB_CONFIG_DIR") != "" {
		return nil
	}

	var configFiles []string

	// First, check system-wide XDG config directories (lowest priority)
	// xdg.ConfigDirs includes both XDG_CONFIG_DIRS and XDG_CONFIG_HOME
	// We need to iterate through them and exclude the user config home
	for _, dir := range xdg.ConfigDirs {
		// Skip if this is the user config home directory
		if dir == xdg.ConfigHome {
			continue
		}
		configPath := filepath.Join(dir, "glab-cli", "config.yml")
		if _, err := os.Stat(configPath); err == nil {
			configFiles = append(configFiles, configPath)
		}
	}

	// Finally, check user config (highest priority)
	userConfigPath := filepath.Join(xdg.ConfigHome, "glab-cli", "config.yml")
	if _, err := os.Stat(userConfigPath); err == nil {
		configFiles = append(configFiles, userConfigPath)
	}

	return configFiles
}

// Init initialises and returns the cached configuration
func Init() (Config, error) {
	if cachedConfig != nil || configError != nil {
		return cachedConfig, configError
	}
	cachedConfig, configError = ParseDefaultConfig()

	if os.IsNotExist(configError) {
		if err := cachedConfig.WriteAll(); err != nil {
			return nil, err
		}
		configError = nil
	}
	return cachedConfig, configError
}

func ParseDefaultConfig() (Config, error) {
	// When GLAB_CONFIG_DIR is set, use single-file logic (no merging)
	if os.Getenv("GLAB_CONFIG_DIR") != "" {
		configPath, err := SearchConfigFile()
		if err != nil {
			// No config found, use default writable location
			configPath = ConfigFile()
		}
		return ParseConfig(configPath)
	}

	// XDG mode: search for all config files to merge
	configFiles := SearchAllConfigFiles()
	if len(configFiles) == 0 {
		// No configs found, use default writable location
		return ParseConfig(ConfigFile())
	}

	if len(configFiles) == 1 {
		// Only one config file, no need to merge
		return ParseConfig(configFiles[0])
	}

	// Multiple config files found, merge them
	return parseAndMergeConfigs(configFiles)
}

// parseAndMergeConfigs loads multiple config files and merges them.
// Files are processed in order (system → user), with later files taking precedence.
// Merging rules:
// - Global keys: later value completely replaces earlier value
// - "hosts" key: merge host entries, combining settings for matching hosts
func parseAndMergeConfigs(configFiles []string) (Config, error) {
	if len(configFiles) == 0 {
		return nil, fmt.Errorf("no config files to merge")
	}

	// Parse the first (base) config file
	_, baseRoot, err := ParseConfigFile(configFiles[0])
	if err != nil {
		if os.IsNotExist(err) {
			baseRoot = NewBlankRoot()
		} else {
			return nil, fmt.Errorf("failed to parse base config %s: %w", configFiles[0], err)
		}
	}

	// Merge each subsequent config file into the base
	for i := 1; i < len(configFiles); i++ {
		_, nextRoot, err := ParseConfigFile(configFiles[i])
		if err != nil {
			if os.IsNotExist(err) {
				continue // Skip non-existent files
			}
			return nil, fmt.Errorf("failed to parse config %s: %w", configFiles[i], err)
		}

		// Merge nextRoot into baseRoot
		baseRoot = mergeYAMLNodes(baseRoot, nextRoot)
	}

	// Now load local config and aliases as usual
	// Load local config file
	if _, localRoot, err := ParseConfigFile(LocalConfigFile()); err == nil {
		if len(localRoot.Content[0].Content) > 0 {
			newContent := []*yaml.Node{
				{Value: "local"},
				localRoot.Content[0],
			}
			restContent := baseRoot.Content[0].Content
			baseRoot.Content[0].Content = append(newContent, restContent...)
		}
	}

	// Load aliases config file
	if _, aliasesRoot, err := ParseConfigFile(aliasesConfigFile()); err == nil {
		if len(aliasesRoot.Content[0].Content) > 0 {
			newContent := []*yaml.Node{
				{Value: "aliases"},
				aliasesRoot.Content[0],
			}
			restContent := baseRoot.Content[0].Content
			baseRoot.Content[0].Content = append(newContent, restContent...)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	return NewConfig(baseRoot), nil
}

// mergeYAMLNodes merges two YAML document nodes.
// For most keys, newer values completely replace base values.
// For the "hosts" key, we merge host entries intelligently.
func mergeYAMLNodes(base, newer *yaml.Node) *yaml.Node {
	if base == nil || len(base.Content) == 0 {
		return newer
	}
	if newer == nil || len(newer.Content) == 0 {
		return base
	}

	// Work with the mapping node (first content of document)
	baseMap := base.Content[0]
	newerMap := newer.Content[0]

	if baseMap.Kind != yaml.MappingNode || newerMap.Kind != yaml.MappingNode {
		// If either is not a mapping, just use the newer one
		return newer
	}

	// Create a map of keys in base for quick lookup
	baseKeys := make(map[string]int) // key -> index in Content slice
	for i := 0; i < len(baseMap.Content); i += 2 {
		if i+1 < len(baseMap.Content) {
			key := baseMap.Content[i].Value
			baseKeys[key] = i
		}
	}

	// Process keys from newer config
	for i := 0; i < len(newerMap.Content); i += 2 {
		if i+1 >= len(newerMap.Content) {
			continue
		}

		keyNode := newerMap.Content[i]
		valueNode := newerMap.Content[i+1]
		key := keyNode.Value

		if baseIdx, exists := baseKeys[key]; exists {
			// Key exists in both configs
			if key == "hosts" {
				// Special merging for hosts
				baseMap.Content[baseIdx+1] = mergeHostsNode(baseMap.Content[baseIdx+1], valueNode)
			} else {
				// For other keys, newer value replaces base value
				baseMap.Content[baseIdx+1] = valueNode
			}
		} else {
			// Key only in newer config, add it to base
			baseMap.Content = append(baseMap.Content, keyNode, valueNode)
		}
	}

	return base
}

// mergeHostsNode merges two "hosts" mapping nodes.
// Hosts that appear in both configs have their settings merged.
// Hosts that appear in only one config are included as-is.
func mergeHostsNode(base, newer *yaml.Node) *yaml.Node {
	if base == nil || base.Kind != yaml.MappingNode {
		return newer
	}
	if newer == nil || newer.Kind != yaml.MappingNode {
		return base
	}

	// Create a map of host names in base
	baseHosts := make(map[string]int) // hostname -> index in Content slice
	for i := 0; i < len(base.Content); i += 2 {
		if i+1 < len(base.Content) {
			hostname := base.Content[i].Value
			baseHosts[hostname] = i
		}
	}

	// Process hosts from newer config
	for i := 0; i < len(newer.Content); i += 2 {
		if i+1 >= len(newer.Content) {
			continue
		}

		hostnameNode := newer.Content[i]
		hostSettingsNode := newer.Content[i+1]
		hostname := hostnameNode.Value

		if baseIdx, exists := baseHosts[hostname]; exists {
			// Host exists in both configs, merge settings
			base.Content[baseIdx+1] = mergeHostSettings(base.Content[baseIdx+1], hostSettingsNode)
		} else {
			// Host only in newer config, add it
			base.Content = append(base.Content, hostnameNode, hostSettingsNode)
		}
	}

	return base
}

// mergeHostSettings merges settings for a specific host.
// Settings in newer config override settings in base config.
func mergeHostSettings(base, newer *yaml.Node) *yaml.Node {
	if base == nil || base.Kind != yaml.MappingNode {
		return newer
	}
	if newer == nil || newer.Kind != yaml.MappingNode {
		return base
	}

	// Create a map of settings in base
	baseSettings := make(map[string]int) // setting key -> index in Content slice
	for i := 0; i < len(base.Content); i += 2 {
		if i+1 < len(base.Content) {
			settingKey := base.Content[i].Value
			baseSettings[settingKey] = i
		}
	}

	// Process settings from newer config
	for i := 0; i < len(newer.Content); i += 2 {
		if i+1 >= len(newer.Content) {
			continue
		}

		settingKeyNode := newer.Content[i]
		settingValueNode := newer.Content[i+1]
		settingKey := settingKeyNode.Value

		if baseIdx, exists := baseSettings[settingKey]; exists {
			// Setting exists in both, newer value overrides
			base.Content[baseIdx+1] = settingValueNode
		} else {
			// Setting only in newer config, add it
			base.Content = append(base.Content, settingKeyNode, settingValueNode)
		}
	}

	return base
}

var ReadConfigFile = func(filename string) ([]byte, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, pathError(err)
	}

	return data, nil
}

var WriteConfigFile = func(filename string, data []byte) error {
	err := os.MkdirAll(path.Dir(filename), 0o750)
	if err != nil {
		return pathError(err)
	}
	_, err = os.ReadFile(filename)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	err = WriteFile(filename, data, 0o600)
	return err
}

func ParseConfigFile(filename string) ([]byte, *yaml.Node, error) {
	stat, err := os.Stat(filename)
	// we want to check if there actually is a file, sometimes
	// configs are just passed via stubs
	if err == nil {
		if !HasSecurePerms(stat.Mode().Perm()) {
			return nil, nil,
				fmt.Errorf("%s has the permissions %o, but glab requires 600.\nConsider running `chmod 600 %s`",
					filename,
					stat.Mode(),
					filename,
				)
		}
	}

	data, err := ReadConfigFile(filename)
	if err != nil {
		return nil, nil, err
	}

	root, err := parseConfigData(data)
	if err != nil {
		return nil, nil, err
	}
	return data, root, err
}

func parseConfigData(data []byte) (*yaml.Node, error) {
	var root yaml.Node
	err := yaml.Unmarshal(data, &root)
	if err != nil {
		return nil, err
	}

	if len(root.Content) == 0 {
		return &yaml.Node{
			Kind:    yaml.DocumentNode,
			Content: []*yaml.Node{{Kind: yaml.MappingNode}},
		}, nil
	}
	if root.Content[0].Kind != yaml.MappingNode {
		return &root, fmt.Errorf("expected a top level map")
	}
	return &root, nil
}

func ParseConfig(filename string) (Config, error) {
	_, root, err := ParseConfigFile(filename)
	var confError error
	if err != nil {
		if os.IsNotExist(err) {
			root = NewBlankRoot()
			confError = os.ErrNotExist
		} else {
			return nil, err
		}
	}

	// Load local config file
	if _, localRoot, err := ParseConfigFile(LocalConfigFile()); err == nil {
		if len(localRoot.Content[0].Content) > 0 {
			newContent := []*yaml.Node{
				{Value: "local"},
				localRoot.Content[0],
			}
			restContent := root.Content[0].Content
			root.Content[0].Content = append(newContent, restContent...)
		}
	}

	// Load aliases config file
	if _, aliasesRoot, err := ParseConfigFile(aliasesConfigFile()); err == nil {
		if len(aliasesRoot.Content[0].Content) > 0 {
			newContent := []*yaml.Node{
				{Value: "aliases"},
				aliasesRoot.Content[0],
			}
			restContent := root.Content[0].Content
			root.Content[0].Content = append(newContent, restContent...)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	return NewConfig(root), confError
}

func pathError(err error) error {
	var pathError *os.PathError
	if errors.As(err, &pathError) && errors.Is(pathError.Err, syscall.ENOTDIR) {
		if p := findRegularFile(pathError.Path); p != "" {
			return fmt.Errorf("remove or rename regular file `%s` (must be a directory)", p)
		}
	}
	return err
}

func findRegularFile(p string) string {
	for {
		if s, err := os.Stat(p); err == nil && s.Mode().IsRegular() {
			return p
		}
		newPath := path.Dir(p)
		if newPath == p || newPath == "/" || newPath == "." {
			break
		}
		p = newPath
	}
	return ""
}
