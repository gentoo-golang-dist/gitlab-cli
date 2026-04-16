package visualize

import (
	"fmt"
	"strings"
)

// BuildD2Source generates D2 diagram source text from a parsed Pipeline.
// If showStageEdges is true, dashed edges between consecutive stages are included.
func BuildD2Source(p *Pipeline, showStageEdges bool) string {
	var b strings.Builder

	b.WriteString("direction: right\n\n")
	writeClasses(&b)

	// Build a map of job name -> stage key for edge construction.
	jobStage := make(map[string]string)
	for _, j := range p.Jobs {
		jobStage[j.Name] = stageKey(j.Stage)
	}

	// Group jobs by stage.
	stageJobs := make(map[string][]Job)
	for _, j := range p.Jobs {
		stageJobs[j.Stage] = append(stageJobs[j.Stage], j)
	}

	// Write stage containers and their jobs.
	for _, stage := range p.Stages {
		jobs := stageJobs[stage]
		if len(jobs) == 0 {
			continue
		}
		sk := stageKey(stage)
		fmt.Fprintf(&b, "%s: {\n", sk)
		fmt.Fprintf(&b, "  class: stage\n")
		fmt.Fprintf(&b, "  label: %q\n", stage)
		for _, j := range jobs {
			cls := "job"
			if j.IsTrigger {
				cls = "trigger_job"
			}
			fmt.Fprintf(&b, "  %s: {\n", q(j.Name))
			fmt.Fprintf(&b, "    class: %s\n", cls)
			fmt.Fprintf(&b, "    label: %q\n", j.Name)
			fmt.Fprintf(&b, "  }\n")
		}
		fmt.Fprintf(&b, "}\n\n")
	}

	// Write needs edges between specific jobs.
	for _, j := range p.Jobs {
		for _, need := range j.Needs {
			srcStage, ok := jobStage[need]
			if !ok {
				continue // referenced job not found, skip
			}
			dstStage := jobStage[j.Name]
			fmt.Fprintf(&b, "%s.%s -> %s.%s: {class: needs_edge}\n",
				srcStage, q(need), dstStage, q(j.Name))
		}
	}

	// Write implicit stage ordering edges.
	if showStageEdges && len(p.Stages) > 1 {
		b.WriteString("\n")
		for i := 0; i < len(p.Stages)-1; i++ {
			fmt.Fprintf(&b, "%s -> %s: {class: stage_edge}\n",
				stageKey(p.Stages[i]), stageKey(p.Stages[i+1]))
		}
	}

	return b.String()
}

func writeClasses(b *strings.Builder) {
	b.WriteString(`classes: {
  stage: {
    style.fill: "#f0f0f0"
    style.stroke: "#999999"
    style.stroke-width: 2
    style.border-radius: 8
    style.font-size: 20
    style.bold: true
  }
  job: {
    style.fill: "#e5f3ff"
    style.stroke: "#2196f3"
    style.border-radius: 4
  }
  trigger_job: {
    style.fill: "#ffe5cc"
    style.stroke: "#ff9800"
    style.border-radius: 4
  }
  needs_edge: {
    style.stroke: "#2196f3"
    style.stroke-width: 2
  }
  stage_edge: {
    style.stroke: "#cccccc"
    style.stroke-dash: 3
  }
}

`)
}

// stageKey returns a D2-safe key for a stage name.
func stageKey(stage string) string {
	return q(fmt.Sprintf("stage:%s", stage))
}

// q quotes a string for use as a D2 key.
func q(s string) string {
	return fmt.Sprintf("%q", s)
}
