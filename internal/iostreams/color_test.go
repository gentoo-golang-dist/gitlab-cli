//go:build !integration

package iostreams

import (
	"bytes"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/cli/internal/theme"
)

func Test_isColorEnabled(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		got := detectIsColorEnabled()
		assert.True(t, got)
	})

	t.Run("NO_COLOR", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")

		got := detectIsColorEnabled()
		assert.False(t, got)
	})

	t.Run("COLOR_ENABLED == 1", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		t.Setenv("COLOR_ENABLED", "1")

		got := detectIsColorEnabled()
		assert.True(t, got)
	})

	t.Run("COLOR_ENABLED == true", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		t.Setenv("COLOR_ENABLED", "true")

		got := detectIsColorEnabled()
		assert.True(t, got)
	})
}

func Test_makeGrayString(t *testing.T) {
	tests := []struct {
		name    string
		profile termenv.Profile
		want    string
	}{
		{
			name:    "gray 16-color terminal",
			profile: termenv.ANSI,
			want:    "\x1b[90mtext\x1b[0m", // color 8 → bright black
		},
		{
			name:    "gray 256-color terminal",
			profile: termenv.ANSI256,
			want:    "\x1b[38;5;242mtext\x1b[0m",
		},
		{
			name:    "no colors (Ascii profile)",
			profile: termenv.Ascii,
			want:    "text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := termenv.NewOutput(bytes.NewBuffer(nil), termenv.WithProfile(tt.profile))
			fn := makeGrayString(out)
			require.Equal(t, tt.want, fn("text"))
		})
	}
}

func Test_makeStyledString(t *testing.T) {
	brandGreen := lipgloss.Color("#34D058")
	t.Run("cyan_no_brand_ansi", func(t *testing.T) {
		out := termenv.NewOutput(bytes.NewBuffer(nil), termenv.WithProfile(termenv.ANSI))
		require.Equal(t, "\x1b[36mx\x1b[0m", makeStyledString(out, nil, "cyan")("x"))
	})

	t.Run("green_24bit_when_brand_set_and_TrueColor", func(t *testing.T) {
		out := termenv.NewOutput(bytes.NewBuffer(nil), termenv.WithProfile(termenv.TrueColor))
		got := makeStyledString(out, brandGreen, "green")("text")
		require.Equal(t, "\x1b[38;2;52;208;88mtext\x1b[0m", got) // #34D058; reset from [termenv.Output.String]
	})
	t.Run("green_with_brand_falls_back_to_ansi_index_when_not_TrueColor", func(t *testing.T) {
		out := termenv.NewOutput(bytes.NewBuffer(nil), termenv.WithProfile(termenv.ANSI))
		require.Equal(t, "\x1b[32mtext\x1b[0m", makeStyledString(out, brandGreen, "green")("text"))
	})
	t.Run("green_nil_brand_uses_ansi_name_map", func(t *testing.T) {
		out := termenv.NewOutput(bytes.NewBuffer(nil), termenv.WithProfile(termenv.TrueColor))
		require.Equal(t, "\x1b[32mtext\x1b[0m", makeStyledString(out, nil, "green")("text"))
	})
}

func Test_makeBoldString(t *testing.T) {
	tests := []struct {
		name    string
		profile termenv.Profile
		input   string
		want    string
	}{
		{
			name:    "Ascii: no escape sequences",
			profile: termenv.Ascii,
			input:   "text",
			want:    "text",
		},
		{
			name:    "ANSI bold",
			profile: termenv.ANSI,
			input:   "text",
			want:    "\x1b[1mtext\x1b[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := termenv.NewOutput(bytes.NewBuffer(nil), termenv.WithProfile(tt.profile))
			got := makeBoldString(out)(tt.input)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_magentaTrueColor_usesThemeRGB(t *testing.T) {
	// NewGitLabColors(…) uses fixed dark #A989F5 for Purple on dark; LightDark true → dark path.
	lightDark := lipgloss.LightDark(true)
	glc := theme.NewGitLabColors(lightDark)
	out := termenv.NewOutput(bytes.NewBuffer(nil), termenv.WithProfile(termenv.TrueColor))
	fn := makeStyledString(out, glc.Purple, "magenta")
	// #A989F5 → 169, 137, 245
	const want = "\x1b[38;2;169;137;245mrow\x1b[0m"
	require.Equal(t, want, fn("row"))
}
