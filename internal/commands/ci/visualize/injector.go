package visualize

import (
	"bytes"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// injectVariables merges a top-level `variables:` block into the given YAML
func injectVariables(content []byte, vars map[string]string) ([]byte, error) {
	if len(vars) == 0 {
		return content, nil
	}

	var root yaml.Node
	if err := yaml.Unmarshal(content, &root); err != nil {
		return nil, fmt.Errorf("parsing YAML for variable injection: %w", err)
	}

	// An empty document becomes a single mapping with our variables block.
	if root.Kind == 0 || len(root.Content) == 0 {
		doc := &yaml.Node{Kind: yaml.DocumentNode}
		doc.Content = []*yaml.Node{buildMappingWithVariables(vars)}
		return marshalNode(doc)
	}

	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 {
		return nil, fmt.Errorf("unexpected YAML structure: expected a single document")
	}
	top := root.Content[0]
	if top.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("unexpected YAML structure: top level must be a mapping")
	}

	varsNode := findMappingValue(top, "variables")
	if varsNode == nil {
		// No existing variables: prepend a fresh block so it's the first key.
		newVars := buildVariablesNode(vars)
		key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "variables"}
		top.Content = append([]*yaml.Node{key, newVars}, top.Content...)
	} else if varsNode.Kind == yaml.MappingNode {
		mergeIntoMapping(varsNode, vars)
	} else {
		// variables: existed but wasn't a mapping — replace it.
		*varsNode = *buildVariablesNode(vars)
	}

	return marshalNode(&root)
}

// findMappingValue returns the value node for `key` in a mapping, or nil.
func findMappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

// mergeIntoMapping inserts vars into an existing mapping node, overwriting
// keys that already exist so the user's flags win.
func mergeIntoMapping(mapping *yaml.Node, vars map[string]string) {
	existing := make(map[string]int)
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		existing[mapping.Content[i].Value] = i + 1
	}
	for _, k := range sortedKeys(vars) {
		v := vars[k]
		if valIdx, ok := existing[k]; ok {
			mapping.Content[valIdx] = scalarNode(v)
			continue
		}
		mapping.Content = append(mapping.Content, scalarNode(k), scalarNode(v))
	}
}

func buildVariablesNode(vars map[string]string) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, k := range sortedKeys(vars) {
		n.Content = append(n.Content, scalarNode(k), scalarNode(vars[k]))
	}
	return n
}

func buildMappingWithVariables(vars map[string]string) *yaml.Node {
	top := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	top.Content = []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: "variables"},
		buildVariablesNode(vars),
	}
	return top
}

func scalarNode(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func marshalNode(node *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(node); err != nil {
		return nil, fmt.Errorf("re-marshaling YAML: %w", err)
	}
	_ = enc.Close()
	return buf.Bytes(), nil
}
