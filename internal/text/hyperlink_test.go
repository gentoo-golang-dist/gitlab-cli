//go:build !integration

package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertMarkdownLinksToOSC8(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single link",
			input:    "See [docs](https://example.com) for info",
			expected: "See \x1b]8;;https://example.com\x1b\\docs\x1b]8;;\x1b\\ for info",
		},
		{
			name:     "multiple links",
			input:    "[Link 1](https://one.com) and [Link 2](https://two.com)",
			expected: "\x1b]8;;https://one.com\x1b\\Link 1\x1b]8;;\x1b\\ and \x1b]8;;https://two.com\x1b\\Link 2\x1b]8;;\x1b\\",
		},
		{
			name:     "no links",
			input:    "Just plain text",
			expected: "Just plain text",
		},
		{
			name:     "link with URL fragment",
			input:    "See [API docs](https://example.com/api#section) here",
			expected: "See \x1b]8;;https://example.com/api#section\x1b\\API docs\x1b]8;;\x1b\\ here",
		},
		{
			name:     "link with query string",
			input:    "See [results](https://example.com/search?q=test&lang=go) here",
			expected: "See \x1b]8;;https://example.com/search?q=test&lang=go\x1b\\results\x1b]8;;\x1b\\ here",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "link at start of string",
			input:    "[docs](https://example.com) has more info",
			expected: "\x1b]8;;https://example.com\x1b\\docs\x1b]8;;\x1b\\ has more info",
		},
		{
			name:     "link at end of string",
			input:    "More info at [docs](https://example.com)",
			expected: "More info at \x1b]8;;https://example.com\x1b\\docs\x1b]8;;\x1b\\",
		},
		{
			name:     "multiword link text",
			input:    "See [personal access token scopes](https://docs.gitlab.com/tokens) for details",
			expected: "See \x1b]8;;https://docs.gitlab.com/tokens\x1b\\personal access token scopes\x1b]8;;\x1b\\ for details",
		},
		{
			name:     "URL with closing parenthesis is truncated (known limitation)",
			input:    "[See](https://en.wikipedia.org/wiki/Foo_(bar)) here",
			expected: "\x1b]8;;https://en.wikipedia.org/wiki/Foo_(bar\x1b\\See\x1b]8;;\x1b\\) here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertMarkdownLinksToOSC8(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertOSC8ToMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single OSC 8 link",
			input:    "See \x1b]8;;https://example.com\x1b\\docs\x1b]8;;\x1b\\ for info",
			expected: "See [docs](https://example.com) for info",
		},
		{
			name:     "multiple OSC 8 links",
			input:    "\x1b]8;;https://one.com\x1b\\Link 1\x1b]8;;\x1b\\ and \x1b]8;;https://two.com\x1b\\Link 2\x1b]8;;\x1b\\",
			expected: "[Link 1](https://one.com) and [Link 2](https://two.com)",
		},
		{
			name:     "no links",
			input:    "Just plain text",
			expected: "Just plain text",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "link with URL fragment",
			input:    "See \x1b]8;;https://example.com/api#section\x1b\\API docs\x1b]8;;\x1b\\ here",
			expected: "See [API docs](https://example.com/api#section) here",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertOSC8ToMarkdown(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMarkdownToOSC8RoundTrip(t *testing.T) {
	originals := []string{
		"See [docs](https://example.com) for more [info](https://gitlab.com)",
		"[Link 1](https://one.com) and [Link 2](https://two.com)",
		"The [token scopes documentation](https://docs.gitlab.com/ee/user/profile/personal_access_tokens#personal-access-token-scopes) has details.",
	}

	for _, original := range originals {
		t.Run(original, func(t *testing.T) {
			osc8 := ConvertMarkdownLinksToOSC8(original)
			assert.NotEqual(t, original, osc8, "OSC 8 output should differ from input")

			result := ConvertOSC8ToMarkdown(osc8)
			assert.Equal(t, original, result, "Round-trip should reproduce the original markdown")
		})
	}
}
