package dag

import (
	"fmt"
	"maps"
	"path/filepath"
)

// RuleClause represents a single entry in a rules: list.
type RuleClause struct {
	If      string
	When    string
	Changes []string
	Exists  []string
}

// OnlyExcept represents the legacy only:/except: configuration.
type OnlyExcept struct {
	Refs      []string
	Variables []string
}

// SimulationConfig holds the configuration for a pipeline simulation.
type SimulationConfig struct {
	Source       string
	Branch       string
	Tag          string
	SourceBranch string
	TargetBranch string
	ExtraVars    map[string]string
}

// BuildVariables creates the CI variable map from simulation flags.
// Variables are inferred from the flags provided rather than hard-coded pipeline types.
func BuildVariables(cfg SimulationConfig) map[string]string {
	vars := map[string]string{
		"CI_DEFAULT_BRANCH": "main",
	}

	// Set source if provided.
	if cfg.Source != "" {
		vars["CI_PIPELINE_SOURCE"] = cfg.Source
	}

	// --tag sets CI_COMMIT_TAG and implies push source.
	if cfg.Tag != "" {
		vars["CI_COMMIT_TAG"] = cfg.Tag
		vars["CI_COMMIT_REF_NAME"] = cfg.Tag
		if cfg.Source == "" {
			vars["CI_PIPELINE_SOURCE"] = "push"
		}
	}

	// --branch sets CI_COMMIT_BRANCH.
	if cfg.Branch != "" {
		vars["CI_COMMIT_BRANCH"] = cfg.Branch
		if vars["CI_COMMIT_REF_NAME"] == "" {
			vars["CI_COMMIT_REF_NAME"] = cfg.Branch
		}
		if cfg.Source == "" && cfg.Tag == "" {
			vars["CI_PIPELINE_SOURCE"] = "push"
		}
	}

	// --source-branch / --target-branch imply MR pipeline.
	if cfg.SourceBranch != "" || cfg.TargetBranch != "" {
		if cfg.Source == "" {
			vars["CI_PIPELINE_SOURCE"] = "merge_request_event"
		}
		if _, ok := vars["CI_MERGE_REQUEST_IID"]; !ok {
			vars["CI_MERGE_REQUEST_IID"] = "1"
		}
		if cfg.SourceBranch != "" {
			vars["CI_MERGE_REQUEST_SOURCE_BRANCH_NAME"] = cfg.SourceBranch
			if vars["CI_COMMIT_REF_NAME"] == "" {
				vars["CI_COMMIT_REF_NAME"] = cfg.SourceBranch
			}
		}
		if cfg.TargetBranch != "" {
			vars["CI_MERGE_REQUEST_TARGET_BRANCH_NAME"] = cfg.TargetBranch
		} else {
			vars["CI_MERGE_REQUEST_TARGET_BRANCH_NAME"] = "main"
		}
	}

	// User-provided variables override everything.
	maps.Copy(vars, cfg.ExtraVars)

	return vars
}

// isMRPipeline returns true if the variables indicate a merge request pipeline.
func isMRPipeline(vars map[string]string) bool {
	return vars["CI_PIPELINE_SOURCE"] == "merge_request_event"
}

// EvalWorkflowRules checks if the pipeline would be created at all.
// Returns nil if the pipeline should proceed, or an error explaining why not.
func EvalWorkflowRules(rules []RuleClause, vars map[string]string) error {
	if len(rules) == 0 {
		return nil
	}

	for _, rule := range rules {
		matched, err := evalRuleClause(rule, vars)
		if err != nil {
			return fmt.Errorf("evaluating workflow rule: %w", err)
		}
		if matched {
			if rule.When == "never" {
				return fmt.Errorf("pipeline would not be created: workflow rule matched with when:never")
			}
			return nil
		}
	}

	return fmt.Errorf("pipeline would not be created: no workflow:rules matched for this pipeline configuration")
}

// FilterJobs returns only the jobs that would run given the simulated variables.
func FilterJobs(jobs []Job, vars map[string]string) []Job {
	var filtered []Job
	for _, job := range jobs {
		if shouldJobRun(job, vars) {
			filtered = append(filtered, job)
		}
	}
	return filtered
}

func shouldJobRun(job Job, vars map[string]string) bool {
	if len(job.Rules) > 0 {
		return evalJobRules(job.Rules, vars)
	}

	if job.Only != nil || job.Except != nil {
		return evalOnlyExcept(job.Only, job.Except, vars)
	}

	// No rules at all: default behavior.
	// In MR pipelines, jobs without rules do NOT run.
	// In branch/tag/schedule/other pipelines, jobs without rules DO run.
	return !isMRPipeline(vars)
}

func evalJobRules(rules []RuleClause, vars map[string]string) bool {
	for _, rule := range rules {
		matched, err := evalRuleClause(rule, vars)
		if err != nil {
			continue
		}
		if matched {
			return rule.When != "never"
		}
	}
	return false
}

func evalRuleClause(rule RuleClause, vars map[string]string) (bool, error) {
	// Evaluate if: expression.
	if rule.If != "" {
		result, err := EvalExpression(rule.If, vars)
		if err != nil {
			return false, err
		}
		if !result {
			return false, nil
		}
	}

	// changes: defaults to true (we can't determine actual file changes locally).
	// A rule with ONLY changes: and no if: will match.

	// Evaluate exists: against local filesystem.
	if len(rule.Exists) > 0 {
		if !evalExists(rule.Exists) {
			return false, nil
		}
	}

	return true, nil
}

func evalExists(patterns []string) bool {
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			return false
		}
	}
	return true
}

func evalOnlyExcept(only *OnlyExcept, except *OnlyExcept, vars map[string]string) bool {
	if only != nil {
		if !matchesOnlyExceptRefs(only.Refs, vars) {
			return false
		}
		for _, expr := range only.Variables {
			ok, _ := EvalExpression(expr, vars)
			if !ok {
				return false
			}
		}
	}

	if except != nil {
		if matchesOnlyExceptRefs(except.Refs, vars) {
			return false
		}
	}

	return true
}

func matchesOnlyExceptRefs(refs []string, vars map[string]string) bool {
	if len(refs) == 0 {
		return true
	}
	branch := vars["CI_COMMIT_BRANCH"]
	tag := vars["CI_COMMIT_TAG"]
	source := vars["CI_PIPELINE_SOURCE"]
	for _, ref := range refs {
		switch ref {
		case "branches":
			if branch != "" {
				return true
			}
		case "tags":
			if tag != "" {
				return true
			}
		case "merge_requests":
			if source == "merge_request_event" {
				return true
			}
		case "schedules":
			if source == "schedule" {
				return true
			}
		default:
			if ref == branch || ref == tag {
				return true
			}
		}
	}
	return false
}
