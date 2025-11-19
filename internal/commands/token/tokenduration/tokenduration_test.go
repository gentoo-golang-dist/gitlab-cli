//go:build !integration

package tokenduration

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{
			name:    "1 day with d suffix",
			input:   "1d",
			want:    24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "7 days with d suffix",
			input:   "7d",
			want:    7 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "30 days with d suffix",
			input:   "30d",
			want:    30 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "365 days with d suffix",
			input:   "365d",
			want:    365 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "1 week with w suffix",
			input:   "1w",
			want:    7 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "2 weeks with w suffix",
			input:   "2w",
			want:    14 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "52 weeks with w suffix",
			input:   "52w",
			want:    52 * 7 * 24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "24 hours with h suffix (backward compat)",
			input:   "24h",
			want:    24 * time.Hour,
			wantErr: false,
		},
		{
			name:    "168 hours with h suffix (1 week)",
			input:   "168h",
			want:    168 * time.Hour,
			wantErr: false,
		},
		{
			name:    "720 hours with h suffix (30 days)",
			input:   "720h",
			want:    720 * time.Hour,
			wantErr: false,
		},
		{
			name:    "invalid: just a number",
			input:   "30",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid: fractional days",
			input:   "1.5d",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid: unsupported unit",
			input:   "1y",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid: month suffix",
			input:   "1M",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid: empty string",
			input:   "",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid: negative duration",
			input:   "-1d",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && time.Duration(got) != tt.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, time.Duration(got), tt.want)
			}
		})
	}
}

func TestTokenDuration_String(t *testing.T) {
	tests := []struct {
		name     string
		duration TokenDuration
		want     string
	}{
		{
			name:     "1 day",
			duration: TokenDuration(24 * time.Hour),
			want:     "1d",
		},
		{
			name:     "7 days (1 week)",
			duration: TokenDuration(7 * 24 * time.Hour),
			want:     "1w",
		},
		{
			name:     "14 days (2 weeks)",
			duration: TokenDuration(14 * 24 * time.Hour),
			want:     "2w",
		},
		{
			name:     "30 days",
			duration: TokenDuration(30 * 24 * time.Hour),
			want:     "30d",
		},
		{
			name:     "365 days (52 weeks + 1 day, should display as days)",
			duration: TokenDuration(365 * 24 * time.Hour),
			want:     "365d",
		},
		{
			name:     "12 hours",
			duration: TokenDuration(12 * time.Hour),
			want:     "12h0m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.duration.String()
			if got != tt.want {
				t.Errorf("TokenDuration.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTokenDuration_Set(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    TokenDuration
		wantErr bool
	}{
		{
			name:    "set 30d",
			value:   "30d",
			want:    TokenDuration(30 * 24 * time.Hour),
			wantErr: false,
		},
		{
			name:    "set 2w",
			value:   "2w",
			want:    TokenDuration(14 * 24 * time.Hour),
			wantErr: false,
		},
		{
			name:    "set 24h",
			value:   "24h",
			want:    TokenDuration(24 * time.Hour),
			wantErr: false,
		},
		{
			name:    "set invalid",
			value:   "invalid",
			want:    TokenDuration(0),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d TokenDuration
			err := d.Set(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("TokenDuration.Set(%q) error = %v, wantErr %v", tt.value, err, tt.wantErr)
				return
			}
			if !tt.wantErr && d != tt.want {
				t.Errorf("TokenDuration.Set(%q) = %v, want %v", tt.value, d, tt.want)
			}
		})
	}
}

func TestTokenDuration_Type(t *testing.T) {
	var d TokenDuration
	if got := d.Type(); got != "duration" {
		t.Errorf("TokenDuration.Type() = %v, want %v", got, "duration")
	}
}

func TestTokenDuration_Duration(t *testing.T) {
	td := TokenDuration(30 * 24 * time.Hour)
	want := 30 * 24 * time.Hour
	if got := td.Duration(); got != want {
		t.Errorf("TokenDuration.Duration() = %v, want %v", got, want)
	}
}
