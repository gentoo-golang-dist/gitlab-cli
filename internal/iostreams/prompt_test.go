//go:build !integration

package iostreams

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

// createTestIOStreams creates an IOStreams instance for testing prompts.
// Note: These tests primarily verify the structure of the methods and types.
// Full integration tests would require terminal emulation which is complex.
func createTestIOStreams() *IOStreams {
	return &IOStreams{
		In:     io.NopCloser(bytes.NewReader(nil)),
		StdOut: &bytes.Buffer{},
		StdErr: &bytes.Buffer{},

		IsaTTY:   true,
		IsErrTTY: true,
		IsInTTY:  true,
	}
}

func TestNewSelectOption(t *testing.T) {
	t.Run("string option", func(t *testing.T) {
		opt := NewSelectOption("Display Label", "value")
		assert.Equal(t, "Display Label", opt.Label)
		assert.Equal(t, "value", opt.Value)
	})

	t.Run("int64 option", func(t *testing.T) {
		opt := NewSelectOption("ID 123", int64(123))
		assert.Equal(t, "ID 123", opt.Label)
		assert.Equal(t, int64(123), opt.Value)
	})

	t.Run("struct option", func(t *testing.T) {
		type customType struct {
			ID   int
			Name string
		}
		val := customType{ID: 1, Name: "test"}
		opt := NewSelectOption("Custom Item", val)
		assert.Equal(t, "Custom Item", opt.Label)
		assert.Equal(t, val, opt.Value)
	})
}

func TestSelectOption(t *testing.T) {
	t.Run("SelectOption with string type", func(t *testing.T) {
		options := []SelectOption[string]{
			{Label: "Option A", Value: "a"},
			{Label: "Option B", Value: "b"},
			{Label: "Option C", Value: "c"},
		}

		assert.Len(t, options, 3)
		assert.Equal(t, "Option A", options[0].Label)
		assert.Equal(t, "a", options[0].Value)
	})

	t.Run("SelectOption with int64 type", func(t *testing.T) {
		options := []SelectOption[int64]{
			{Label: "Key 1", Value: 100},
			{Label: "Key 2", Value: 200},
		}

		assert.Len(t, options, 2)
		assert.Equal(t, "Key 1", options[0].Label)
		assert.Equal(t, int64(100), options[0].Value)
	})
}

func TestFormBuilder(t *testing.T) {
	t.Run("NewForm creates empty builder", func(t *testing.T) {
		ios := createTestIOStreams()
		fb := ios.NewForm()

		assert.NotNil(t, fb)
		assert.NotNil(t, fb.fields)
		assert.Empty(t, fb.fields)
	})

	t.Run("AddInput adds field", func(t *testing.T) {
		ios := createTestIOStreams()
		fb := ios.NewForm()

		var result string
		fb.AddInput(&result, "Title", "Description", "default")

		assert.Len(t, fb.fields, 1)
		assert.Equal(t, "default", result)
	})

	t.Run("AddInputWithValidation adds field", func(t *testing.T) {
		ios := createTestIOStreams()
		fb := ios.NewForm()

		var result string
		validator := func(s string) error { return nil }
		fb.AddInputWithValidation(&result, "Title", "Description", "default", validator)

		assert.Len(t, fb.fields, 1)
		assert.Equal(t, "default", result)
	})

	t.Run("AddPassword adds field", func(t *testing.T) {
		ios := createTestIOStreams()
		fb := ios.NewForm()

		var result string
		fb.AddPassword(&result, "Password", "Enter password", nil)

		assert.Len(t, fb.fields, 1)
	})

	t.Run("AddSelect adds field", func(t *testing.T) {
		ios := createTestIOStreams()
		fb := ios.NewForm()

		var result string
		fb.AddSelect(&result, "Choose", "Pick one", []string{"a", "b", "c"})

		assert.Len(t, fb.fields, 1)
	})

	t.Run("AddConfirm adds field", func(t *testing.T) {
		ios := createTestIOStreams()
		fb := ios.NewForm()

		var result bool
		fb.AddConfirm(&result, "Confirm?", "Are you sure?", true)

		assert.Len(t, fb.fields, 1)
		assert.True(t, result) // default value applied
	})

	t.Run("AddMultiSelect adds field", func(t *testing.T) {
		ios := createTestIOStreams()
		fb := ios.NewForm()

		var result []string
		fb.AddMultiSelect(&result, "Select multiple", "", []string{"x", "y", "z"})

		assert.Len(t, fb.fields, 1)
	})

	t.Run("AddText adds field", func(t *testing.T) {
		ios := createTestIOStreams()
		fb := ios.NewForm()

		var result string
		fb.AddText(&result, "Description", "Enter text", "placeholder...")

		assert.Len(t, fb.fields, 1)
	})

	t.Run("fluent API chains correctly", func(t *testing.T) {
		ios := createTestIOStreams()
		fb := ios.NewForm()

		var inputResult string
		var selectResult string
		var confirmResult bool

		result := fb.
			AddInput(&inputResult, "Name", "", "").
			AddSelect(&selectResult, "Type", "", []string{"a", "b"}).
			AddConfirm(&confirmResult, "Continue?", "", false)

		assert.Same(t, fb, result) // same instance returned
		assert.Len(t, fb.fields, 3)
	})

	t.Run("Run with empty fields returns nil", func(t *testing.T) {
		ios := createTestIOStreams()
		fb := ios.NewForm()

		err := fb.Run(context.Background())
		assert.NoError(t, err)
	})
}

func TestAddSelectTyped(t *testing.T) {
	t.Run("adds typed select field", func(t *testing.T) {
		ios := createTestIOStreams()
		fb := ios.NewForm()

		var result int64
		options := []SelectOption[int64]{
			{Label: "One", Value: 1},
			{Label: "Two", Value: 2},
		}

		AddSelectTyped(fb, &result, "Select number", "", options)

		assert.Len(t, fb.fields, 1)
	})

	t.Run("returns same builder for chaining", func(t *testing.T) {
		ios := createTestIOStreams()
		fb := ios.NewForm()

		var result string
		options := []SelectOption[string]{
			{Label: "A", Value: "a"},
		}

		returned := AddSelectTyped(fb, &result, "Title", "Desc", options)

		assert.Same(t, fb, returned)
	})
}

func TestAddMultiSelectTyped(t *testing.T) {
	t.Run("adds typed multi-select field", func(t *testing.T) {
		ios := createTestIOStreams()
		fb := ios.NewForm()

		var result []int
		options := []SelectOption[int]{
			{Label: "One", Value: 1},
			{Label: "Two", Value: 2},
			{Label: "Three", Value: 3},
		}

		AddMultiSelectTyped(fb, &result, "Select numbers", "", options)

		assert.Len(t, fb.fields, 1)
	})
}

func TestPromptMethodsExist(t *testing.T) {
	// These tests verify that the prompt methods exist and have the correct signatures.
	// They don't actually run the prompts since that requires terminal emulation.

	ios := createTestIOStreams()

	t.Run("PromptInput exists with correct signature", func(t *testing.T) {
		var fn func(context.Context, *string, string, string, string) error = ios.PromptInput
		assert.NotNil(t, fn)
	})

	t.Run("PromptSelect exists with correct signature", func(t *testing.T) {
		var fn func(context.Context, *string, string, string, []string) error = ios.PromptSelect
		assert.NotNil(t, fn)
	})

	t.Run("PromptConfirm exists with correct signature", func(t *testing.T) {
		var fn func(context.Context, *bool, string, string, bool) error = ios.PromptConfirm
		assert.NotNil(t, fn)
	})

	t.Run("PromptMultiSelect exists with correct signature", func(t *testing.T) {
		var fn func(context.Context, *[]string, string, string, []string) error = ios.PromptMultiSelect
		assert.NotNil(t, fn)
	})
}

func TestPromptSelectTyped(t *testing.T) {
	t.Run("function exists with correct signature", func(t *testing.T) {
		// Verify that PromptSelectTyped can be called with the expected types
		ios := createTestIOStreams()
		var result int64
		options := []SelectOption[int64]{
			{Label: "Option 1", Value: 1},
		}

		// This verifies the function signature is correct
		var fn func(*IOStreams, context.Context, *int64, string, string, []SelectOption[int64]) error
		fn = PromptSelectTyped[int64]
		assert.NotNil(t, fn)

		// The function exists and accepts the correct parameters
		_ = ios
		_ = result
		_ = options
	})
}
