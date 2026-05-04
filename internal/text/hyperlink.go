package text

import (
	"fmt"
	"regexp"
)

var mdLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)

// ConvertMarkdownLinksToOSC8 converts markdown [text](url) syntax to OSC 8 hyperlinks.

func ConvertMarkdownLinksToOSC8(s string) string {
	return mdLinkPattern.ReplaceAllStringFunc(s, func(match string) string {
		parts := mdLinkPattern.FindStringSubmatch(match)
		if len(parts) >= 3 {
			displayText := parts[1]
			url := parts[2]
			return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, displayText)
		}
		return match
	})
}

// Assumes URL and link text don't contain \x1b (safe for human-authored content).
var osc8Pattern = regexp.MustCompile(`\x1b\]8;;([^\x1b]+)\x1b\\([^\x1b]+)\x1b\]8;;\x1b\\`)

// ConvertOSC8ToMarkdown converts OSC 8 hyperlinks back to markdown [text](url) syntax.
func ConvertOSC8ToMarkdown(s string) string {
	return osc8Pattern.ReplaceAllString(s, "[$2]($1)")
}
