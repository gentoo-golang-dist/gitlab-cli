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
