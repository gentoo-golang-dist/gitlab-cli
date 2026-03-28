package iostreams

import (
	"fmt"
	"io"
	"os"
	"strings"
	"strconv"

	"github.com/mattn/go-colorable"
	"github.com/mgutz/ansi"
	"github.com/muesli/termenv"
)

type ColorPalette struct {
	// Magenta outputs ANSI color if stdout is a tty
	Magenta func(string) string
	// Cyan outputs ANSI color if stdout is a tty
	Cyan func(string) string
	// Red outputs ANSI color if stdout is a tty
	Red func(string) string
	// Yellow outputs ANSI color if stdout is a tty
	Yellow func(string) string
	// Blue outputs ANSI color if stdout is a tty
	Blue func(string) string
	// Green outputs ANSI color if stdout is a tty
	Green func(string) string
	// Gray outputs ANSI color if stdout is a tty
	Gray func(string) string
	// Bold outputs ANSI color if stdout is a tty
	Bold func(string) string
}

func (s *IOStreams) Color() *ColorPalette {
	isColorfulOutput := s.ColorEnabled() && s.IsaTTY
	isDarkBackground := termenv.HasDarkBackground()
	return &ColorPalette{
		Magenta: makeColorFunc(isColorfulOutput, isDarkBackground, "magenta"),
		Cyan:    makeColorFunc(isColorfulOutput, isDarkBackground, "cyan"),
		Red:     makeColorFunc(isColorfulOutput, isDarkBackground, "red"),
		Yellow:  makeColorFunc(isColorfulOutput, isDarkBackground, "yellow"),
		Blue:    makeColorFunc(isColorfulOutput, isDarkBackground, "blue"),
		Green:   makeColorFunc(isColorfulOutput, isDarkBackground, "green"),
		Gray:    makeColorFunc(isColorfulOutput, isDarkBackground, "black+h"),
		Bold:    makeColorFunc(isColorfulOutput, isDarkBackground, "default+b"),
	}
}

// NewColorable returns an output stream that handles ANSI color sequences on Windows
func NewColorable(out io.Writer) io.Writer {
	if outFile, isFile := out.(*os.File); isFile {
		return colorable.NewColorable(outFile)
	}
	return out
}

func makeColorFunc(isColorfulOutput bool, isDarkBackground bool, color string) func(string) string {
	if isColorfulOutput && is256ColorSupported() {
		if r, g, b, ok := gitlabAccessibleColor(color, !isDarkBackground); ok {
			return func(t string) string {
				return fmt.Sprintf("\x1b[38;2;%d;%d;%dm%s\x1b[m", r, g, b, t)
			}
		}

		if color == "black+h" {
			return func(t string) string {
				return fmt.Sprintf("\x1b[38;5;242m%s\x1b[m", t)
			}
		}
	}

	cf := ansi.ColorFunc(color)
	return func(arg string) string {
		if isColorfulOutput {
			return cf(arg)
		}
		return arg
	}
}

func gitlabAccessibleColor(name string, darkVariant bool) (r, g, b uint8, ok bool) {
	type pair struct{ dark, light string }
	palette := map[string]pair{
		"blue":    {"#1068BF", "#4285F4"},
		"green":   {"#217645", "#34D058"},
		"red":     {"#C91C00", "#F97583"},
		"magenta": {"#7759C2", "#A989F5"},
		"cyan":    {"#008B96", "#73D3C3"},
	}
	p, found := palette[name]
	if !found {
			return 0, 0, 0, false
	}
	hex := p.light
	if darkVariant {
			hex = p.dark
	}

	r, g, b, err := hexToRGB(hex)
	return r, g, b, err == nil
}

func hexToRGB(hex string) (r, g, b uint8, err error) {
	// Remove # if present
	if hex[0] == '#' {
		hex = hex[1:]
	}

	// Convert each component to decimal
	r64, err := strconv.ParseUint(hex[0:2], 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}

	g64, err := strconv.ParseUint(hex[2:4], 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}

	b64, err := strconv.ParseUint(hex[4:6], 16, 8)
	if err != nil {
		return 0, 0, 0, err
	}

	return uint8(r64), uint8(g64), uint8(b64), nil
}

// detectIsColorEnabled determines whether color output should be enabled based on environment variables.
// It follows the NO_COLOR specification (https://no-color.org/) with an override mechanism:
//
// - If NO_COLOR environment variable exists (with any value), color is disabled by default
// - If COLOR_ENABLED is set to "1" or "true", it overrides NO_COLOR and forces color to be enabled
// - If NO_COLOR doesn't exist, color is enabled by default
//
// This allows users to disable color globally with NO_COLOR while still providing an escape hatch
// via COLOR_ENABLED for specific use cases.
func detectIsColorEnabled() bool {
	// Check if NO_COLOR environment variable exists (any value disables color)
	_, noColorVarExists := os.LookupEnv("NO_COLOR")

	// If NO_COLOR exists, check if COLOR_ENABLED explicitly overrides it
	if noColorVarExists {
		colorEnabled := os.Getenv("COLOR_ENABLED")
		return colorEnabled == "1" || colorEnabled == "true"
	}

	// If NO_COLOR doesn't exist, color is enabled by default
	return true
}

func is256ColorSupported() bool {
	term := os.Getenv("TERM")
	colorterm := os.Getenv("COLORTERM")

	return strings.Contains(term, "256") ||
		strings.Contains(term, "24bit") ||
		strings.Contains(term, "truecolor") ||
		strings.Contains(colorterm, "256") ||
		strings.Contains(colorterm, "24bit") ||
		strings.Contains(colorterm, "truecolor")
}
