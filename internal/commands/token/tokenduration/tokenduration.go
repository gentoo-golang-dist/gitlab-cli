package tokenduration

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// TokenDuration wraps time.Duration to provide custom parsing for token lifetimes.
// It supports parsing duration strings with h (hours), d (days), and w (weeks) suffixes.
// This type implements the pflag.Value interface for use with Cobra commands.
type TokenDuration time.Duration

// ParseDuration parses a duration string with support for hours (h), days (d), and weeks (w).
// It accepts formats like: "24h", "30d", "2w", "720h".
// The duration must be a whole number of days when converted.
func ParseDuration(s string) (TokenDuration, error) {
	// Try standard Go duration first (for backward compatibility with "24h", "720h", etc.)
	if d, err := time.ParseDuration(s); err == nil {
		return TokenDuration(d), nil
	}

	// Try day suffix: 1d, 30d, 365d
	re := regexp.MustCompile(`^(\d+)d$`)
	if matches := re.FindStringSubmatch(s); matches != nil {
		days, _ := strconv.Atoi(matches[1])
		return TokenDuration(time.Duration(days) * 24 * time.Hour), nil
	}

	// Try week suffix: 1w, 2w, 52w
	re = regexp.MustCompile(`^(\d+)w$`)
	if matches := re.FindStringSubmatch(s); matches != nil {
		weeks, _ := strconv.Atoi(matches[1])
		return TokenDuration(time.Duration(weeks) * 7 * 24 * time.Hour), nil
	}

	return 0, fmt.Errorf("invalid duration format: %s (expected formats: 24h, 30d, 2w)", s)
}

// String returns the duration formatted as a string.
// The String() function is required for Cobra to display the default value.
func (d TokenDuration) String() string {
	duration := time.Duration(d)
	hours := duration.Hours()

	// Display in the most readable format
	if hours >= 24 && int(hours)%(24*7) == 0 {
		// Display as weeks if it's a whole number of weeks
		weeks := int(hours) / (24 * 7)
		return fmt.Sprintf("%dw", weeks)
	} else if hours >= 24 && int(hours)%24 == 0 {
		// Display as days if it's a whole number of days
		days := int(hours) / 24
		return fmt.Sprintf("%dd", days)
	}
	// Otherwise display as hours
	return duration.String()
}

// Set parses the given value and sets this TokenDuration.
// The Set() function is required for Cobra to parse command line arguments.
func (d *TokenDuration) Set(value string) error {
	parsed, err := ParseDuration(value)
	if err != nil {
		return err
	}
	*d = parsed
	return nil
}

// Type returns the type name for help text.
// The Type() function is required for Cobra to display type information in help.
func (d *TokenDuration) Type() string {
	return "duration"
}

// Duration returns the underlying time.Duration value.
func (d TokenDuration) Duration() time.Duration {
	return time.Duration(d)
}
