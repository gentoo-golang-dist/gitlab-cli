//go:build !integration

package cmdutils

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBoolFlagPair(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		args    []string
		wantNil bool
		wantVal bool
		wantErr bool
	}{
		{
			name:    "neither flag set leaves target nil",
			args:    []string{},
			wantNil: true,
		},
		{
			name:    "on-flag without value sets true",
			args:    []string{"--pause"},
			wantVal: true,
		},
		{
			name:    "on-flag with explicit true sets true",
			args:    []string{"--pause=true"},
			wantVal: true,
		},
		{
			name:    "on-flag with explicit false sets false",
			args:    []string{"--pause=false"},
			wantVal: false,
		},
		{
			name:    "off-flag without value sets false",
			args:    []string{"--unpause"},
			wantVal: false,
		},
		{
			name:    "off-flag with explicit true sets false",
			args:    []string{"--unpause=true"},
			wantVal: false,
		},
		{
			name:    "off-flag with explicit false sets false",
			args:    []string{"--unpause=false"},
			wantVal: false,
		},
		{
			name:    "both flags returns error",
			args:    []string{"--pause", "--unpause"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var target *bool
			cmd := &cobra.Command{
				Use: "test",
				RunE: func(cmd *cobra.Command, args []string) error {
					return nil
				},
			}
			NewBoolFlagPair(cmd, &target, "pause", "Pause the runner", "unpause", "Resume a paused runner")

			cmd.SetArgs(tt.args)
			err := cmd.Execute()

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.wantNil {
				assert.Nil(t, target, "expected target to remain nil when no flag is set")
				return
			}

			require.NotNil(t, target, "expected target to be set")
			assert.Equal(t, tt.wantVal, *target)
		})
	}
}

func TestNewBoolFlagPair_RegistersFlags(t *testing.T) {
	t.Parallel()

	var target *bool
	cmd := &cobra.Command{Use: "test"}
	NewBoolFlagPair(cmd, &target, "pause", "Pause the runner", "unpause", "Resume a paused runner")

	onFlag := cmd.Flags().Lookup("pause")
	require.NotNil(t, onFlag, "expected on-flag to be registered")
	assert.Equal(t, "Pause the runner", onFlag.Usage)
	assert.Equal(t, "true", onFlag.NoOptDefVal)

	offFlag := cmd.Flags().Lookup("unpause")
	require.NotNil(t, offFlag, "expected off-flag to be registered")
	assert.Equal(t, "Resume a paused runner", offFlag.Usage)
	assert.Equal(t, "true", offFlag.NoOptDefVal)
}
