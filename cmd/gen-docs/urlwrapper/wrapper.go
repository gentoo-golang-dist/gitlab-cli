package urlwrapper

import (
	"fmt"

	"mvdan.cc/xurls/v2"
)

// DetectAndWrap detects bare URLs in text and wraps them in markdown link syntax.
// It avoids double-wrapping URLs that are already in markdown format.
// URLs inside backticks are left as-is since they typically represent literal values.
func DetectAndWrap(text string) string {
	xurlsStrict := xurls.Strict()
	insideBackticks := buildBacktickMap(text)

	offset := 0
	result := text

	for {
		match := xurlsStrict.FindStringIndex(result[offset:])
		if match == nil {
			break
		}

		startIdx := offset + match[0]
		endIdx := offset + match[1]
		url := result[startIdx:endIdx]

		if shouldWrapURL(result, startIdx, insideBackticks) {
			wrapped := fmt.Sprintf("[%s](%s)", url, url)
			result = result[:startIdx] + wrapped + result[endIdx:]
			insideBackticks = buildBacktickMap(result)
			offset = startIdx + len(wrapped)
		} else {
			offset = endIdx
		}
	}

	return result
}

// shouldWrapURL determines if a URL at the given position should be wrapped in markdown link syntax.
func shouldWrapURL(text string, startIdx int, insideBackticks map[int]bool) bool {
	// Don't wrap if already part of a markdown link: [text](URL)
	if startIdx >= 2 && text[startIdx-2:startIdx] == "](" {
		return false
	}

	// Don't wrap if inside backticks (literal values)
	if insideBackticks[startIdx] {
		return false
	}

	return true
}

// buildBacktickMap creates a map of character positions that are inside backtick code spans.
// It properly handles paired backticks and ignores escaped backticks.
func buildBacktickMap(text string) map[int]bool {
	insideCode := make(map[int]bool)
	i := 0

	for i < len(text) {
		if isEscapedBacktick(text, i) {
			i++
			continue
		}

		if text[i] == '`' {
			backtickCount := countConsecutiveBackticks(text, i)
			startPos := i
			i += backtickCount

			closingPos := findClosingBackticks(text, i, backtickCount)
			if closingPos != -1 {
				markRangeAsCode(insideCode, startPos, closingPos+backtickCount)
				i = closingPos + backtickCount
			}
			continue
		}
		i++
	}

	return insideCode
}

// isEscapedBacktick checks if the backtick at position i is escaped with a backslash.
func isEscapedBacktick(text string, i int) bool {
	return i > 0 && text[i-1] == '\\' && text[i] == '`'
}

// countConsecutiveBackticks counts how many backticks appear consecutively starting at position i.
func countConsecutiveBackticks(text string, i int) int {
	count := 0
	for i < len(text) && text[i] == '`' {
		count++
		i++
	}
	return count
}

// markRangeAsCode marks all positions in the given range as being inside code.
func markRangeAsCode(insideCode map[int]bool, start, end int) {
	for pos := start; pos < end; pos++ {
		insideCode[pos] = true
	}
}

// findClosingBackticks finds the position of closing backticks that match the opening count.
// Returns -1 if no matching closing backticks are found.
func findClosingBackticks(text string, startPos int, count int) int {
	i := startPos
	for i < len(text) {
		if isEscapedBacktick(text, i) {
			i++
			continue
		}

		if text[i] == '`' {
			closingCount := countConsecutiveBackticks(text, i)
			if closingCount == count {
				return i
			}
			i += closingCount
			continue
		}
		i++
	}
	return -1
}
