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
	LogFormatRaw   LogFormat = "raw"
	LogFormatClean LogFormat = "clean"
	LogFormatLLM   LogFormat = "llm"
)

var (
	// GitLab CI section markers: \033[0Ksection_start:TIMESTAMP:NAME\r\033[0K or similar
	sectionStartRE = regexp.MustCompile(`section_start:\d+:(\S+)`)
	sectionEndRE   = regexp.MustCompile(`section_end:\d+:(\S+)`)

	// Common noisy patterns in CI logs
	progressBarRE = regexp.MustCompile(`(?m)^.*(\[#+\s*\]|\d+%\|[█▓░\s]+\|).*$`)
	ansiEscapeRE  = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\]8;;[^\x1b]*\x1b\\`)
)

// FormatLogForLLM processes raw CI job log output into a clean, structured
// format optimized for LLM consumption. It:
//   - Strips ANSI escape codes
//   - Converts GitLab CI sections into markdown headings
//   - Removes progress bars and spinner lines
//   - Collapses repeated blank lines
//   - Compresses repetitive consecutive lines
func FormatLogForLLM(raw string) string {
	// First pass: normalize line endings. GitLab uses \r\033[0K as line separators
	// within section markers, so we convert those to newlines before stripping ANSI.
	normalized := strings.ReplaceAll(raw, "\r\n", "\n")
	// Replace \r followed by content (carriage return overwrites) with newline
	normalized = strings.ReplaceAll(normalized, "\r", "\n")

	// Strip ANSI codes
	clean := stripansi.Strip(normalized)
	// Also catch any remaining escape sequences
	clean = ansiEscapeRE.ReplaceAllString(clean, "")

	lines := strings.Split(clean, "\n")
	var out []string
	var prevLine string
	repeatCount := 0
	inSection := ""

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip empty-ish lines from ANSI cleanup
		if trimmed == "" {
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
			flushRepeats(&out, prevLine, repeatCount)
			repeatCount = 0
			prevLine = ""

			sectionName := formatSectionName(m[1])
			// Extract the header text after the full section marker match
			fullMatch := sectionStartRE.FindString(trimmed)
			header := strings.TrimSpace(trimmed[len(fullMatch):])
			if header != "" {
				sectionName = header
			}
			out = append(out, "", fmt.Sprintf("## %s", sectionName))
			inSection = m[1]
			continue
		}

		// Handle section end
		if m := sectionEndRE.FindStringSubmatch(trimmed); m != nil {
			flushRepeats(&out, prevLine, repeatCount)
			repeatCount = 0
			prevLine = ""
			if inSection == m[1] {
				inSection = ""
			}
			continue
		}

		// Skip progress bars
		if progressBarRE.MatchString(trimmed) {
			continue
		}

		// Compress repeated lines
		if trimmed == prevLine {
			repeatCount++
			continue
		}

		flushRepeats(&out, prevLine, repeatCount)
		repeatCount = 0
		prevLine = trimmed
		out = append(out, trimmed)
	}

	flushRepeats(&out, prevLine, repeatCount)

	result := strings.Join(out, "\n")
	return strings.TrimSpace(result)
}

// FormatLog applies the specified format to raw log output.
func FormatLog(raw string, format LogFormat) string {
	switch format {
	case LogFormatLLM:
		return FormatLogForLLM(raw)
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
