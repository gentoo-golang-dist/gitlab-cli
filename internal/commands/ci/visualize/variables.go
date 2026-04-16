package visualize

import "maps"

// All real evaluation (rules, only/except, workflow) happens on the GitLab
// lint API side; this struct just collects what the user passed.
type SimulationConfig struct {
	Source       string
	Branch       string
	Tag          string
	SourceBranch string
	TargetBranch string
	ExtraVars    map[string]string
}


// The shape mirrors what GitLab sets for each pipeline type at runtime, so
// `rules: if: '$CI_PIPELINE_SOURCE == "merge_request_event"'` etc. match the
// way they would in a real pipeline.
func BuildVariables(cfg SimulationConfig) map[string]string {
	vars := map[string]string{}

	if cfg.Source != "" {
		vars["CI_PIPELINE_SOURCE"] = cfg.Source
	}

	if cfg.Tag != "" {
		vars["CI_COMMIT_TAG"] = cfg.Tag
		vars["CI_COMMIT_REF_NAME"] = cfg.Tag
		if cfg.Source == "" {
			vars["CI_PIPELINE_SOURCE"] = "push"
		}
	}

	if cfg.Branch != "" {
		vars["CI_COMMIT_BRANCH"] = cfg.Branch
		if vars["CI_COMMIT_REF_NAME"] == "" {
			vars["CI_COMMIT_REF_NAME"] = cfg.Branch
		}
		if cfg.Source == "" && cfg.Tag == "" {
			vars["CI_PIPELINE_SOURCE"] = "push"
		}
	}

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

	// User-provided variables win over anything derived from sugar flags.
	maps.Copy(vars, cfg.ExtraVars)

	return vars
}
