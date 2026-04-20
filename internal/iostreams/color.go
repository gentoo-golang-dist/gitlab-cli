package iostreams

import (
	"fmt"
	"image/color"
	"io"
	"os"

	"charm.land/lipgloss/v2"
	"github.com/mattn/go-colorable"
	"github.com/muesli/termenv"

	"gitlab.com/gitlab-org/cli/internal/theme"
)

// ANSI index strings for [termenv.Output.Profile] (mgutz-style names → xterm index).
// "black+h" and "default+b" are not listed: gray and bold are handled by [makeGrayString] and [makeBoldString].
var ansiNameToIndex = map[string]string{
	"magenta": "5",
	"cyan":    "6",
	"red":     "1",
	"yellow":  "3",
	"blue":    "4",
	"green":   "2",
}

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

// Color returns style helpers. It does not cache: s.StdOut may change (e.g. when a pager starts).
func (s *IOStreams) Color() *ColorPalette {
	noop := func(s string) string { return s }
	identity := &ColorPalette{
		Magenta: noop, Cyan: noop, Red: noop, Yellow: noop,
		Blue: noop, Green: noop, Gray: noop, Bold: noop,
	}

	if !s.ColorEnabled() || !s.IsaTTY {
		return identity
	}

	out := termenv.NewOutput(NewColorable(s.StdOut))

	var isDark bool
	switch s.BackgroundColor() { // "none" means not yet resolved: detect
	case "dark":
		isDark = true
	case "light":
		isDark = false
	default: // "none" — same [termenv.Output] as colors; only probe background when we already allow color+TTY
		isDark = s.ColorEnabled() && s.IsaTTY && out.HasDarkBackground()
	}
	lightDark := lipgloss.LightDark(isDark)
	glc := theme.NewGitLabColors(lightDark) // 24-bit RGB when terminal is TrueColor-capable

	return &ColorPalette{
		Magenta: makeStyledString(out, glc.Purple, "magenta"),
		Cyan:    makeStyledString(out, nil, "cyan"), // not in brand palette, ANSI/termenv
		Red:     makeStyledString(out, glc.Red, "red"),
		Yellow:  makeStyledString(out, nil, "yellow"), // not in brand palette, ANSI/termenv
		Blue:    makeStyledString(out, glc.Blue, "blue"),
		Green:   makeStyledString(out, glc.Green, "green"),
		Gray:    makeGrayString(out),
		Bold:    makeBoldString(out),
	}
}

// NewColorable returns an output stream that handles ANSI color sequences on Windows
func NewColorable(out io.Writer) io.Writer {
	if outFile, isFile := out.(*os.File); isFile {
		return colorable.NewColorable(outFile)
	}
	return out
}

// makeStyledString applies either GitLab theme RGB (TrueColor) or a fixed ANSI name via [termenv].
// Brand hex and index colors are resolved once when the palette is built, not on every cell.
//
// 24-bit theme RGB is only for TrueColor: ANSI256/ANSI get [ansiNameToIndex] (good fallback, not
// a downgrade bug). (termenv iota is not monotonic in “capability”; TrueColor=0 … Ascii=3 — do
// not use >= on [termenv.Profile] for 24-bit.)
func makeStyledString(out *termenv.Output, brand color.Color, ansiName string) func(string) string {
	if brand != nil && out.Profile == termenv.TrueColor {
		r16, g16, b16, _ := brand.RGBA()
		r, g, b := uint8(r16>>8), uint8(g16>>8), uint8(b16>>8)
		// One hex + [termenv.RGBColor] per palette field; closure only applies style per string.
		fg := termenv.RGBColor(fmt.Sprintf("#%02x%02x%02x", r, g, b))
		return func(s string) string {
			return out.String(s).Foreground(fg).String()
		}
	}

	idx, ok := ansiNameToIndex[ansiName]
	if !ok {
		return func(s string) string { return s }
	}
	c := out.Profile.Color(idx)
	return func(s string) string {
		return out.String(s).Foreground(c).String()
	}
}

// makeGrayString maps “subtle gray”: 242 on 256+ / true-color terminals, else dim (index 8).
// termenv iota: TrueColor(0), ANSI256(1), ANSI(2), Ascii(3) — 242 only for TrueColor or ANSI256.
func makeGrayString(out *termenv.Output) func(string) string {
	return func(s string) string {
		if out.Profile == termenv.TrueColor || out.Profile == termenv.ANSI256 {
			return out.String(s).Foreground(out.Profile.Color("242")).String()
		}
		return out.String(s).Foreground(out.Profile.Color("8")).String()
	}
}

// makeBoldString is bold (or plain on Ascii profile) via [termenv].
func makeBoldString(out *termenv.Output) func(string) string {
	return func(s string) string {
		return out.String(s).Bold().String()
	}
}

// detectIsColorEnabled determines whether color output should be enabled based on environment variables.
// It follows the NO_COLOR specification (https://no-color.org/) with an override mechanism:
//
//   - If NO_COLOR environment variable exists (with any value), color is disabled by default
//   - If COLOR_ENABLED is set to "1" or "true", it overrides NO_COLOR and forces color to be enabled
//   - If NO_COLOR doesn't exist, color is enabled by default
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
