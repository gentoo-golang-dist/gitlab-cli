//go:build !integration

package cmdutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBoolPtrFlag(t *testing.T) {
	var target *bool
	f := NewBoolPtrFlag(&target, true)
	require.NotNil(t, f)
	assert.Nil(t, target)

	err := f.Set("")
	require.NoError(t, err)
	require.NotNil(t, target)
	assert.True(t, *target)
}

func TestBoolPtrFlag_Set_TrueFlag(t *testing.T) {
	var target *bool
	f := NewBoolPtrFlag(&target, true)

	for _, val := range []string{"true", ""} {
		t.Run("Set_"+val, func(t *testing.T) {
			target = nil
			err := f.Set(val)
			require.NoError(t, err)
			require.NotNil(t, target)
			assert.True(t, *target)
		})
	}
}

func TestBoolPtrFlag_Set_FalseFlag(t *testing.T) {
	var target *bool
	f := NewBoolPtrFlag(&target, false)

	err := f.Set("")
	require.NoError(t, err)
	require.NotNil(t, target)
	assert.False(t, *target)

	err = f.Set("true")
	require.NoError(t, err)
	assert.False(t, *target)
}

func TestBoolPtrFlag_Set_ExplicitFalse(t *testing.T) {
	var target *bool
	f := NewBoolPtrFlag(&target, true)

	err := f.Set("false")
	require.NoError(t, err)
	require.NotNil(t, target)
	assert.False(t, *target)
}

func TestBoolPtrFlag_Set_InvalidValue(t *testing.T) {
	var target *bool
	f := NewBoolPtrFlag(&target, true)

	err := f.Set("invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid value for flag")
	assert.Contains(t, err.Error(), "invalid")
}

func TestBoolPtrFlag_String(t *testing.T) {
	var target *bool
	f := NewBoolPtrFlag(&target, true)

	assert.Equal(t, "", f.String(), "unset target gives empty string")

	_ = f.Set("true")
	assert.Equal(t, "true", f.String())

	target = nil
	assert.Equal(t, "", f.String())

	fFalse := NewBoolPtrFlag(&target, false)
	_ = fFalse.Set("false")
	assert.Equal(t, "false", fFalse.String())
}

func TestBoolPtrFlag_Type(t *testing.T) {
	var target *bool
	f := NewBoolPtrFlag(&target, true)
	assert.Equal(t, "bool", f.Type())
}

func TestBoolPtrFlag_IsBoolFlag(t *testing.T) {
	var target *bool
	f := NewBoolPtrFlag(&target, true)
	assert.True(t, f.IsBoolFlag())
}

func TestBoolPtrFlag_TwoFlagsShareTarget(t *testing.T) {
	var target *bool
	pauseFlag := NewBoolPtrFlag(&target, true)
	unpauseFlag := NewBoolPtrFlag(&target, false)

	require.NoError(t, pauseFlag.Set(""))
	require.NotNil(t, target)
	assert.True(t, *target)
	assert.Equal(t, "true", pauseFlag.String())

	require.NoError(t, unpauseFlag.Set(""))
	assert.False(t, *target)
	assert.Equal(t, "false", unpauseFlag.String())

	require.NoError(t, pauseFlag.Set("true"))
	assert.True(t, *target)
}
