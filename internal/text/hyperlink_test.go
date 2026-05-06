//go:build !integration

package text

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// osc8Formatter mimics IOStreams.Hyperlink when hyperlinks are enabled.
func osc8Formatter(displayText, url string) string {
	return fmt.Sprintf("\x1b]8;;%s\x1b\\%s\x1b]8;;\x1b\\", url, displayText)
}

// plainFormatter mimics IOStreams.Hyperlink when hyperlinks are disabled.
func plainFormatter(displayText, _ string) string {
	return displayText
}

func TestConvertMarkdownLinks(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		formatter func(string, string) string
		expected  string
	}{
		{
			name:      "single link",
			input:     "See [docs](https://example.com) for info",
			formatter: osc8Formatter,
			expected:  "See \x1b]8;;https://example.com\x1b\\docs\x1b]8;;\x1b\\ for info",
		},
		{
			name:      "multiple links",
			input:     "[Link 1](https://one.com) and [Link 2](https://two.com)",
			formatter: osc8Formatter,
			expected:  "\x1b]8;;https://one.com\x1b\\Link 1\x1b]8;;\x1b\\ and \x1b]8;;https://two.com\x1b\\Link 2\x1b]8;;\x1b\\",
		},
		{
			name:      "no links",
			input:     "Just plain text",
			formatter: osc8Formatter,
			expected:  "Just plain text",
		},
		{
			name:      "link with URL fragment",
			input:     "See [API docs](https://example.com/api#section) here",
			formatter: osc8Formatter,
			expected:  "See \x1b]8;;https://example.com/api#section\x1b\\API docs\x1b]8;;\x1b\\ here",
		},
		{
			name:      "link with query string",
			input:     "See [results](https://example.com/search?q=test&lang=go) here",
			formatter: osc8Formatter,
			expected:  "See \x1b]8;;https://example.com/search?q=test&lang=go\x1b\\results\x1b]8;;\x1b\\ here",
		},
		{
			name:      "empty string",
			input:     "",
			formatter: osc8Formatter,
			expected:  "",
		},
		{
			name:      "link at start of string",
			input:     "[docs](https://example.com) has more info",
			formatter: osc8Formatter,
			expected:  "\x1b]8;;https://example.com\x1b\\docs\x1b]8;;\x1b\\ has more info",
		},
		{
			name:      "link at end of string",
			input:     "More info at [docs](https://example.com)",
			formatter: osc8Formatter,
			expected:  "More info at \x1b]8;;https://example.com\x1b\\docs\x1b]8;;\x1b\\",
		},
		{
			name:      "multiword link text",
			input:     "See [personal access token scopes](https://docs.gitlab.com/tokens) for details",
			formatter: osc8Formatter,
			expected:  "See \x1b]8;;https://docs.gitlab.com/tokens\x1b\\personal access token scopes\x1b]8;;\x1b\\ for details",
		},
		{
			name:      "URL with closing parenthesis is truncated (known limitation)",
			input:     "[See](https://en.wikipedia.org/wiki/Foo_(bar)) here",
			formatter: osc8Formatter,
			expected:  "\x1b]8;;https://en.wikipedia.org/wiki/Foo_(bar\x1b\\See\x1b]8;;\x1b\\) here",
		},
		{
			name:      "plain formatter returns display text only",
			input:     "See [docs](https://example.com) for info",
			formatter: plainFormatter,
			expected:  "See docs for info",
		},
		{
			name:      "plain formatter strips all links",
			input:     "[Link 1](https://one.com) and [Link 2](https://two.com)",
			formatter: plainFormatter,
			expected:  "Link 1 and Link 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertMarkdownLinks(tt.input, tt.formatter)
			assert.Equal(t, tt.expected, result)
		})
	}
}
