package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateTypes(t *testing.T) {
	t.Parallel()
	// Define test cases
	tests := []struct {
		name    string
		types   []string
		wantErr bool
	}{
		{
			name:    "valid types",
			types:   []string{"epic", "issue", "task"},
			wantErr: false,
		},
		{
			name:    "empty slice is valid",
			types:   []string{},
			wantErr: false,
		},
		{
			name:    "empty string in slice",
			types:   []string{"epic", "", "task"},
			wantErr: true,
		},
		{
			name:    "whitespace only",
			types:   []string{"epic", "  ", "task"},
			wantErr: true,
		},
		{
			name:    "valid with extra whitespace",
			types:   []string{"  epic  ", "issue", "task  "},
			wantErr: false,
		},
	}

	// run each test case
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			// call the func
			err := ValidateTypes(tt.types)

			// check if error matches expectation
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
