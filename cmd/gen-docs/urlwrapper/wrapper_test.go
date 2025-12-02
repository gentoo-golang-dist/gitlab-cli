package urlwrapper

import (
	"testing"
)

func TestDetectAndWrap(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "bare HTTP URL",
			input:    "See http://example.com for more info",
			expected: "See [http://example.com](http://example.com) for more info",
		},
		{
			name:     "bare HTTPS URL",
			input:    "Visit https://gitlab.com/docs for documentation",
			expected: "Visit [https://gitlab.com/docs](https://gitlab.com/docs) for documentation",
		},
		{
			name:     "URL with fragment",
			input:    "Read https://github.com/charmbracelet/glamour#styles for styles",
			expected: "Read [https://github.com/charmbracelet/glamour#styles](https://github.com/charmbracelet/glamour#styles) for styles",
		},
		{
			name:     "already wrapped URL",
			input:    "See [documentation](https://gitlab.com/docs) here",
			expected: "See [documentation](https://gitlab.com/docs) here",
		},
		{
			name:     "multiple URLs",
			input:    "See https://gitlab.com and https://github.com",
			expected: "See [https://gitlab.com](https://gitlab.com) and [https://github.com](https://github.com)",
		},
		{
			name:     "URL at end of sentence",
			input:    "For more info see https://docs.gitlab.com/administration/settings/usage_statistics/",
			expected: "For more info see [https://docs.gitlab.com/administration/settings/usage_statistics/](https://docs.gitlab.com/administration/settings/usage_statistics/)",
		},
		{
			name:     "no URLs",
			input:    "This is plain text without any URLs",
			expected: "This is plain text without any URLs",
		},
		{
			name:     "URL in Long description with backticks",
			input:    "You can pass in a token on standard input by using `--stdin`. See https://gitlab.com/docs for more.",
			expected: "You can pass in a token on standard input by using `--stdin`. See [https://gitlab.com/docs](https://gitlab.com/docs) for more.",
		},
		{
			name:     "URL with query parameters",
			input:    "Generate token at https://gitlab.com/-/user_settings/personal_access_tokens?scopes=api for access",
			expected: "Generate token at [https://gitlab.com/-/user_settings/personal_access_tokens?scopes=api](https://gitlab.com/-/user_settings/personal_access_tokens?scopes=api) for access",
		},
		{
			name:     "multiline text with URL",
			input:    "First line\nSecond line with https://example.com\nThird line",
			expected: "First line\nSecond line with [https://example.com](https://example.com)\nThird line",
		},
		{
			name:     "URL inside backticks",
			input:    "If unset, defaults to `https://gitlab.com`.",
			expected: "If unset, defaults to `https://gitlab.com`.",
		},
		{
			name:     "URL inside backticks with text after",
			input:    "The default is `https://gitlab.com` for most users.",
			expected: "The default is `https://gitlab.com` for most users.",
		},
		{
			name:     "multiple URLs, one in backticks, one not",
			input:    "See `https://gitlab.com` or visit https://github.com for more",
			expected: "See `https://gitlab.com` or visit [https://github.com](https://github.com) for more",
		},
		{
			name:     "URL in inline code with surrounding text",
			input:    "Use the `https://api.example.com/v1` endpoint to access the API",
			expected: "Use the `https://api.example.com/v1` endpoint to access the API",
		},
		{
			name:     "backticks before and after, URL outside",
			input:    "Run `glab config` to set https://gitlab.com as default",
			expected: "Run `glab config` to set [https://gitlab.com](https://gitlab.com) as default",
		},
		{
			name:     "duplicate URLs with different contexts",
			input:    "See https://gitlab.com for docs. The default is `https://gitlab.com` for most users.",
			expected: "See [https://gitlab.com](https://gitlab.com) for docs. The default is `https://gitlab.com` for most users.",
		},
		{
			name:     "same URL appears three times",
			input:    "Visit https://example.com or `https://example.com` or https://example.com again",
			expected: "Visit [https://example.com](https://example.com) or `https://example.com` or [https://example.com](https://example.com) again",
		},
		{
			name:     "double backtick code span",
			input:    "Use ``code with `backtick` and https://example.com inside``",
			expected: "Use ``code with `backtick` and https://example.com inside``",
		},
		{
			name:     "unmatched backtick",
			input:    "This has an unmatched ` backtick and https://example.com should be wrapped",
			expected: "This has an unmatched ` backtick and [https://example.com](https://example.com) should be wrapped",
		},
		{
			name:     "escaped backtick",
			input:    "Use \\` for literal backtick and https://example.com for docs",
			expected: "Use \\` for literal backtick and [https://example.com](https://example.com) for docs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectAndWrap(tt.input)
			if result != tt.expected {
				t.Errorf("DetectAndWrap() = %q, want %q", result, tt.expected)
			}
		})
	}
}
