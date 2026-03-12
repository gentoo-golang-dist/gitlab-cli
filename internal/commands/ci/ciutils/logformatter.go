package ciutils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/acarl005/stripansi"
)

// LogFormat represents the output format for job logs.
type LogFormat string

const (
	LogFormatRaw     LogFormat = "raw"
	LogFormatClean   LogFormat = "clean"
	LogFormatCompact LogFormat = "compact"
	LogFormatMinimal LogFormat = "minimal"
)

var (
	// GitLab CI section markers: \033[0Ksection_start:TIMESTAMP:NAME\r\033[0K or similar
	sectionStartRE = regexp.MustCompile(`section_start:\d+:(\S+)`)
	sectionEndRE   = regexp.MustCompile(`section_end:\d+:(\S+)`)

	// Common noisy patterns in CI logs
	progressBarRE    = regexp.MustCompile(`(?m)^.*(\[#+\s*\]|\d+%\|[█▓░\s]+\|).*$`)
	downloadProgressRE = regexp.MustCompile(`^Downloading artifacts \d`)
	ansiEscapeRE     = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\]8;;[^\x1b]*\x1b\\`)

	// Patterns for similar-line grouping
	similarLinePrefixes = []string{
		"Waiting for pod ",
		"ContainersNotInitialized:",
		"ContainersNotReady:",
	}

	// Hardmode: sections that are pure boilerplate and get collapsed to a one-liner
	boilerplateSections = map[string]bool{
		"prepare_executor":       true,
		"prepare_script":         true,
		"get_sources":            true,
		"download_artifacts":     true,
		"cleanup_file_variables": true,
		"resolve_secrets":        true,
		"after_script":           true,
	}

	// Hardmode: line-level noise patterns
	curlStatsRE       = regexp.MustCompile(`^\s*(%\s+Total|Dload\s+Upload|\d+[\s.]+\d+[\s.]+\d+[\s.]+\d+)`)
	gitRemoteBannerRE = regexp.MustCompile(`^remote:`)
	shellSetupRE      = regexp.MustCompile(`^\$\s*(chmod\s|export\s|printf\s.*[>|]|cat\s*>.*<<|test\s+-f\s|echo\s+".*\$|function\s+\w+\(\)|case\s+\$|\.\/monitor_|source\s+|mqp_\w+\(\)|if\s+\[)`)
	secretMappingRE   = regexp.MustCompile(`^\[INFO\]\s+"[A-Z_]+:[A-Z_]+"`)
	runnerMetaRE      = regexp.MustCompile(`^(Running with gitlab-runner|on\s+\S+\s+\S+,\s+system ID:|feature flags:|Resolving secrets)`)
	// Decorative lines: box-drawing chars, repeated special chars, or lines that are only Unicode decoration
	decorativeLineRE  = regexp.MustCompile(`^[\x{2500}-\x{257F}\x{2580}-\x{259F}━─═\-_~*#]{10,}$`)
)

const maxLineLength = 500

// FormatLogMinimal processes raw CI job log output into a clean, structured
// format with aggressive noise filtering. It strips everything that doesn't
// contribute to understanding what happened and why. Boilerplate sections
// (executor setup, source checkout, artifact downloads, cleanup) are collapsed
// to one-liners. Shell setup commands, curl stats, git remote banners, secret
// mappings, and decorative lines are removed.
func FormatLogMinimal(raw string) string {
	return formatLLM(raw, true)
}

// FormatLogCompact applies lighter formatting — strips ANSI, converts sections
// to markdown headings, compresses repeats, and truncates long lines, but
// preserves all sections.
func FormatLogCompact(raw string) string {
	return formatLLM(raw, false)
}

func formatLLM(raw string, hardmode bool) string {
	// First pass: normalize line endings
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	// Strip ANSI codes
	clean := stripansi.Strip(normalized)
	clean = ansiEscapeRE.ReplaceAllString(clean, "")

	lines := strings.Split(clean, "\n")
	var out []string
	var prevLine string
	repeatCount := 0
	inSection := ""
	skipSection := false    // hardmode: skip entire boilerplate section
	inSecretMapping := false // hardmode: skip secret mapping block
	beforeFirstSection := hardmode // hardmode: skip runner metadata before first section

	var similarPrefix string
	similarCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Hardmode: skip runner metadata lines before the first section
		if beforeFirstSection && trimmed != "" {
			if runnerMetaRE.MatchString(trimmed) {
				continue
			}
			// If we hit a non-section, non-runner line, stop skipping —
			// this handles logs without any section markers.
			if !sectionStartRE.MatchString(trimmed) {
				beforeFirstSection = false
			}
		}

		// Skip empty-ish lines from ANSI cleanup
		if trimmed == "" {
			if skipSection {
				continue
			}
			inSecretMapping = false
			flushSimilar(&out, similarPrefix, similarCount)
			similarPrefix = ""
			similarCount = 0
			if prevLine == "" && len(out) > 0 {
				continue
			}
			flushRepeats(&out, prevLine, repeatCount)
			repeatCount = 0
			prevLine = ""
			if len(out) > 0 {
				out = append(out, "")
			}
			continue
		}

		// Handle section start
		if m := sectionStartRE.FindStringSubmatch(trimmed); m != nil {
			beforeFirstSection = false
			flushSimilar(&out, similarPrefix, similarCount)
			similarPrefix = ""
			similarCount = 0
			flushRepeats(&out, prevLine, repeatCount)
			repeatCount = 0
			prevLine = ""
			inSecretMapping = false

			sectionID := m[1]
			sectionName := formatSectionName(sectionID)
			fullMatch := sectionStartRE.FindString(trimmed)
			header := strings.TrimSpace(trimmed[len(fullMatch):])
			if header != "" {
				sectionName = header
			}

			// Hardmode: collapse boilerplate sections
			if hardmode && boilerplateSections[sectionID] {
				skipSection = true
				out = append(out, fmt.Sprintf("[%s: ok]", sectionName))
				continue
			}

			skipSection = false
			out = append(out, "", fmt.Sprintf("## %s", sectionName))
			inSection = sectionID
			continue
		}

		// Handle section end
		if m := sectionEndRE.FindStringSubmatch(trimmed); m != nil {
			if skipSection {
				skipSection = false
				continue
			}
			flushSimilar(&out, similarPrefix, similarCount)
			similarPrefix = ""
			similarCount = 0
			flushRepeats(&out, prevLine, repeatCount)
			repeatCount = 0
			prevLine = ""
			inSecretMapping = false
			if inSection == m[1] {
				inSection = ""
			}
			continue
		}

		// Skip lines in collapsed sections
		if skipSection {
			continue
		}

		// Skip progress bars and download progress lines
		if progressBarRE.MatchString(trimmed) || downloadProgressRE.MatchString(trimmed) {
			continue
		}

		// Hardmode: additional line-level filters
		if hardmode {
			// Skip curl transfer statistics
			if curlStatsRE.MatchString(trimmed) {
				continue
			}
			// Skip git remote banners (maintenance notices etc.)
			if gitRemoteBannerRE.MatchString(trimmed) {
				continue
			}
			// Skip shell setup boilerplate commands
			if shellSetupRE.MatchString(trimmed) {
				continue
			}
			// Skip decorative separator lines (━━━, ═══, ----, etc.)
			if decorativeLineRE.MatchString(trimmed) {
				continue
			}
			// Skip secret mapping lines (block starts with [INFO] SECRETS mapping:)
			if strings.Contains(trimmed, "[INFO] SECRETS mapping:") {
				inSecretMapping = true
				continue
			}
			if inSecretMapping && secretMappingRE.MatchString(trimmed) {
				continue
			}
			if inSecretMapping {
				inSecretMapping = false
			}
			// Skip "Executing step_script stage" boilerplate
			if strings.HasPrefix(trimmed, "Executing \"") && strings.Contains(trimmed, "stage of the job script") {
				continue
			}
		}

		// Truncate very long lines
		if len(trimmed) > maxLineLength {
			trimmed = trimmed[:maxLineLength] + "... [truncated]"
		}

		// Check for similar lines (same prefix pattern)
		if prefix := similarLinePrefix(trimmed); prefix != "" {
			if prefix == similarPrefix {
				similarCount++
				prevLine = trimmed
				continue
			}
			flushSimilar(&out, similarPrefix, similarCount)
			similarPrefix = prefix
			similarCount = 1
			prevLine = trimmed
			continue
		}

		flushSimilar(&out, similarPrefix, similarCount)
		similarPrefix = ""
		similarCount = 0

		// Compress exact repeated lines
		if trimmed == prevLine {
			repeatCount++
			continue
		}

		flushRepeats(&out, prevLine, repeatCount)
		repeatCount = 0
		prevLine = trimmed
		out = append(out, trimmed)
	}

	flushSimilar(&out, similarPrefix, similarCount)
	flushRepeats(&out, prevLine, repeatCount)

	result := strings.Join(out, "\n")
	return strings.TrimSpace(result)
}

// FormatLog applies the specified format to raw log output.
func FormatLog(raw string, format LogFormat) string {
	switch format {
	case LogFormatMinimal:
		return FormatLogMinimal(raw)
	case LogFormatCompact:
		return FormatLogCompact(raw)
	case LogFormatClean:
		return stripansi.Strip(raw)
	default:
		return raw
	}
}

func flushRepeats(out *[]string, line string, count int) {
	if count > 2 {
		// Replace the last line with a summary
		if len(*out) > 0 {
			(*out)[len(*out)-1] = fmt.Sprintf("%s  [... repeated %d more times]", line, count)
		}
	} else {
		for range count {
			*out = append(*out, line)
		}
	}
}

// formatSectionName converts snake_case section names to readable titles
func formatSectionName(name string) string {
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "-", " ")
	if len(name) > 0 {
		return strings.ToUpper(name[:1]) + name[1:]
	}
	return name
}

// similarLinePrefix returns a matching prefix if the line belongs to a
// known repetitive pattern group, or empty string if not.
func similarLinePrefix(line string) string {
	for _, prefix := range similarLinePrefixes {
		if strings.HasPrefix(line, prefix) {
			return prefix
		}
	}
	return ""
}

// flushSimilar writes a summary for a group of similar lines.
func flushSimilar(out *[]string, prefix string, count int) {
	if count == 0 || prefix == "" {
		return
	}
	if count == 1 {
		*out = append(*out, strings.TrimSpace(prefix)+"...")
	} else {
		*out = append(*out, fmt.Sprintf("%s... [%d occurrences]", strings.TrimSpace(prefix), count))
	}
}

