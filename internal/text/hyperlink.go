package text

import (
	"fmt"
	"regexp"
)

var mdLinkPattern = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// ConvertMarkdownLinksToOSC8 converts markdown [text](url) syntax to OSC 8 hyperlinks.
// This allows markdown links in help text to display as clickable terminal links
// in terminals that support OSC 8 (iTerm2, Kitty, WezTerm, Windows Terminal, etc.).
// In terminals that don't support OSC 8, the sequences are invisible and only the
// link text is shown.
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

var osc8Pattern = regexp.MustCompile(`\x1b\]8;;([^\x1b]+)\x1b\\([^\x1b]+)\x1b\]8;;\x1b\\`)

// ConvertOSC8ToMarkdown converts OSC 8 hyperlinks back to markdown [text](url) syntax.
// This is used by gen-docs to ensure any terminal hyperlink sequences are converted
// to proper markdown links suitable for web documentation.
func ConvertOSC8ToMarkdown(s string) string {
	return osc8Pattern.ReplaceAllString(s, "[$2]($1)")
}

