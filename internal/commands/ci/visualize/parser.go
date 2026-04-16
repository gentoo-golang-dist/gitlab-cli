package visualize

import (
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// Pipeline is the minimal topology we need to render a DAG: stage order and
// jobs with their stage and needs: edges. Everything else about a CI config
// (rules, only/except, when, workflow) is the GitLab lint API's problem, not
// ours.
type Pipeline struct {
	Stages []string
	Jobs   []Job
}

// Job is a single CI job. Only fields needed for DAG rendering are populated;
// the rest of the YAML job definition is intentionally ignored.
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

// ParsePipeline parses fully compiled YAML (from the GitLab lint API) into a
// Pipeline. The YAML is assumed to already be flattened: no extends:, no
// include:, no rules: simulation. This parser only extracts what's needed for
// topology and deliberately ignores everything else.
func ParsePipeline(yamlContent []byte) (*Pipeline, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(yamlContent, &raw); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	stages := parseStages(raw)
	jobs := parseJobs(raw, stages)

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

func parseJobs(raw map[string]any, stages []string) []Job {
	var jobs []Job
	for key, val := range raw {
		if reservedKeys[key] {
			continue
		}
		// Hidden jobs (templates) start with a dot. The lint API usually drops
		// these from the compiled output, but filter defensively.
		if len(key) > 0 && key[0] == '.' {
			continue
		}

		jobMap, ok := val.(map[string]any)
		if !ok {
			continue
		}

		job := Job{Name: key}

		if stage, ok := jobMap["stage"].(string); ok {
			job.Stage = stage
		} else {
			job.Stage = "test"
		}

		if needsRaw, ok := jobMap["needs"]; ok {
			job.Needs = parseNeeds(needsRaw)
		}

		if depsRaw, ok := jobMap["dependencies"]; ok {
			job.Dependencies = parseStringList(depsRaw)
		}

		if _, ok := jobMap["trigger"]; ok {
			job.IsTrigger = true
		}

		jobs = append(jobs, job)
	}

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
