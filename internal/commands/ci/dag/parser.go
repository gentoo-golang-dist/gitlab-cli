package dag

import (
	"fmt"
	"maps"
	"sort"

	"gopkg.in/yaml.v3"
)

// Pipeline represents the parsed structure of a .gitlab-ci.yml file.
type Pipeline struct {
	Stages []string
	Jobs   []Job
}

// Job represents a single CI job in the pipeline.
type Job struct {
	Name         string
	Stage        string
	Needs        []string
	Dependencies []string
	IsTrigger    bool
}

var defaultStages = []string{".pre", "build", "test", "deploy", ".post"}

var reservedKeys = map[string]bool{
	"stages":        true,
	"variables":     true,
	"include":       true,
	"default":       true,
	"workflow":      true,
	"image":         true,
	"services":      true,
	"before_script": true,
	"after_script":  true,
	"cache":         true,
}

// ParsePipeline parses raw YAML content from a .gitlab-ci.yml file into a Pipeline.
func ParsePipeline(yamlContent []byte) (*Pipeline, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(yamlContent, &raw); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	stages := parseStages(raw)
	jobs := parseJobs(raw, stages)

	// Filter stages to only those that have at least one job.
	usedStages := make(map[string]bool)
	for _, j := range jobs {
		usedStages[j.Stage] = true
	}
	var filteredStages []string
	for _, s := range stages {
		if usedStages[s] {
			filteredStages = append(filteredStages, s)
		}
	}

	return &Pipeline{
		Stages: filteredStages,
		Jobs:   jobs,
	}, nil
}

func parseStages(raw map[string]any) []string {
	stagesRaw, ok := raw["stages"]
	if !ok {
		return defaultStages
	}
	stagesList, ok := stagesRaw.([]any)
	if !ok {
		return defaultStages
	}
	var stages []string
	for _, s := range stagesList {
		if name, ok := s.(string); ok {
			stages = append(stages, name)
		}
	}
	if len(stages) == 0 {
		return defaultStages
	}
	return stages
}

// resolveExtends merges a job map with its extends templates, resolving chains recursively.
// GitLab CI extends uses a "last wins" merge: the job's own keys take precedence
// over the template's keys. For lists of extends, they are applied left to right,
// with each subsequent template overriding the previous, and the job itself winning over all.
func resolveExtends(jobMap map[string]any, raw map[string]any, seen map[string]bool) map[string]any {
	extendsRaw, ok := jobMap["extends"]
	if !ok {
		return jobMap
	}

	// Collect extends targets (can be a single string or a list).
	var targets []string
	switch v := extendsRaw.(type) {
	case string:
		targets = []string{v}
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok {
				targets = append(targets, s)
			}
		}
	}

	if len(targets) == 0 {
		return jobMap
	}

	// Start with an empty base, then layer templates left to right, then the job on top.
	merged := make(map[string]any)

	for _, target := range targets {
		if seen[target] {
			continue // prevent circular extends
		}
		templateRaw, ok := raw[target]
		if !ok {
			continue
		}
		templateMap, ok := templateRaw.(map[string]any)
		if !ok {
			continue
		}

		// Recursively resolve the template's own extends first.
		seen[target] = true
		resolved := resolveExtends(templateMap, raw, seen)

		// Merge resolved template into base (later templates override earlier ones).
		maps.Copy(merged, resolved)
	}

	// The job's own keys override everything from templates.
	for k, v := range jobMap {
		if k == "extends" {
			continue
		}
		merged[k] = v
	}

	return merged
}

func parseJobs(raw map[string]any, stages []string) []Job {
	stageSet := make(map[string]bool)
	for _, s := range stages {
		stageSet[s] = true
	}

	var jobs []Job
	for key, val := range raw {
		if reservedKeys[key] {
			continue
		}
		// Hidden jobs (templates) start with a dot.
		if len(key) > 0 && key[0] == '.' {
			continue
		}

		jobMap, ok := val.(map[string]any)
		if !ok {
			continue
		}

		// Resolve extends chains to inherit stage, needs, etc. from templates.
		resolved := resolveExtends(jobMap, raw, map[string]bool{})

		job := Job{Name: key}

		// Parse stage.
		if stage, ok := resolved["stage"].(string); ok {
			job.Stage = stage
		} else {
			job.Stage = "test"
		}

		// Parse needs.
		if needsRaw, ok := resolved["needs"]; ok {
			job.Needs = parseNeeds(needsRaw)
		}

		// Parse dependencies.
		if depsRaw, ok := resolved["dependencies"]; ok {
			job.Dependencies = parseStringList(depsRaw)
		}

		// Detect trigger/bridge jobs.
		if _, ok := resolved["trigger"]; ok {
			job.IsTrigger = true
		}

		jobs = append(jobs, job)
	}

	// Sort jobs by stage order, then alphabetically within a stage for deterministic output.
	stageOrder := make(map[string]int)
	for i, s := range stages {
		stageOrder[s] = i
	}
	sort.Slice(jobs, func(i, j int) bool {
		si, sj := stageOrder[jobs[i].Stage], stageOrder[jobs[j].Stage]
		if si != sj {
			return si < sj
		}
		return jobs[i].Name < jobs[j].Name
	})

	return jobs
}

func parseNeeds(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	var needs []string
	for _, item := range list {
		switch v := item.(type) {
		case string:
			needs = append(needs, v)
		case map[string]any:
			// Skip cross-pipeline dependencies (those with "pipeline" but no "job").
			jobName, hasJob := v["job"].(string)
			if !hasJob {
				continue
			}
			needs = append(needs, jobName)
		}
	}
	return needs
}

func parseStringList(raw any) []string {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	var result []string
	for _, item := range list {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}
