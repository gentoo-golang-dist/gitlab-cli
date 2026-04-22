//go:build !integration

package iostreams

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func Test_makeColorFunc(t *testing.T) {
	tests := []struct {
		name          string
		ansiColorName string
		trueColor     color.Color
		colorEnabled  bool
		term          string
		want          string
	}{
		{
			name:          "gray 16 colors",
			ansiColorName: "black+h",
			trueColor:     nil,
			colorEnabled:  true,
			term:          "xterm-16color",
			want:          "\x1b[0;90mtext\x1b[0m",
		},
		{
			name:          "gray 256 colors",
			ansiColorName: "black+h",
			trueColor:     nil,
			colorEnabled:  true,
			term:          "xterm-256color",
			want:          "\x1b[38;5;242mtext\x1b[m",
		},
		{
			name:          "no colors",
			ansiColorName: "black+h",
			trueColor:     nil,
			colorEnabled:  false,
			term:          "",
			want:          "text",
		},
		{
			name:          "green when truecolor provided",
			ansiColorName: "green",
			trueColor:     lipgloss.Color("#34D058"),
			colorEnabled:  true,
			term:          "xterm-24bit",
			want:          "\x1b[38;2;52;208;88mtext\x1b[m",
		},
		{
			name:          "green when truecolor provided, but no terminal support",
			ansiColorName: "green",
			trueColor:     lipgloss.Color("#34D058"),
			colorEnabled:  true,
			term:          "xterm-256color",
			want:          "\x1b[0;32mtext\x1b[0m",
		},
		{
			name:          "green when no truecolor in palette",
			ansiColorName: "green",
			trueColor:     nil,
			colorEnabled:  true,
			term:          "xterm-24bit",
			want:          "\x1b[0;32mtext\x1b[0m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("COLORTERM", "")
			t.Setenv("TERM", tt.term)

			fn := makeColorFunc(tt.colorEnabled, tt.trueColor, tt.ansiColorName)
			got := fn("text")

			require.Equal(t, tt.want, got)
		})
	}
}
