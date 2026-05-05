package text

import (
	"regexp"
)

var mdLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)

// ConvertMarkdownLinks converts markdown [text](url) links.
// Typically linkFormatter is IOStreams.Hyperlink, which
// emits OSC 8 sequences when the terminal supports them and returns plain
// display text otherwise.
func ConvertMarkdownLinks(s string, linkFormatter func(displayText, url string) string) string {
	return mdLinkPattern.ReplaceAllStringFunc(s, func(match string) string {
		parts := mdLinkPattern.FindStringSubmatch(match)
		if len(parts) >= 3 {
			return linkFormatter(parts[1], parts[2])
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
