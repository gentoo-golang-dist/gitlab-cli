package iostreams

import (
	"context"
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"gitlab.com/gitlab-org/cli/internal/theme"
)

// SelectOption provides a way to map display text to values for type-safe select prompts
type SelectOption[T any] struct {
	Label string // What the user sees
	Value T      // What gets returned
}

// NewSelectOption creates a new SelectOption with the given label and value
func NewSelectOption[T any](label string, value T) SelectOption[T] {
	return SelectOption[T]{Label: label, Value: value}
}

// PromptInput prompts the user for text input with a title, description, and default value.
// The result is stored in the provided result pointer.
// Returns ErrUserCancelled if the user cancels the prompt.
func (s *IOStreams) PromptInput(ctx context.Context, result *string, title, description, defaultValue string) error {
	if defaultValue != "" {
		*result = defaultValue
	}

	input := huh.NewInput().
		Title(title).
		Value(result)

	if description != "" {
		input = input.Description(description)
	}

	return s.Run(ctx, input)
}

// PromptSelect prompts the user to select one option from a list of string options.
// The result is stored in the provided result pointer.
// Returns ErrUserCancelled if the user cancels the prompt.
func (s *IOStreams) PromptSelect(ctx context.Context, result *string, title, description string, options []string) error {
	selector := huh.NewSelect[string]().
		Title(title).
		Options(huh.NewOptions(options...)...).
		Value(result)

	if description != "" {
		selector = selector.Description(description)
	}

	return s.Run(ctx, selector)
}

// PromptSelectTyped prompts the user to select one option from a list of typed options.
// This allows mapping display labels to arbitrary values.
// The result is stored in the provided result pointer.
// Returns ErrUserCancelled if the user cancels the prompt.
func PromptSelectTyped[T comparable](s *IOStreams, ctx context.Context, result *T, title, description string, options []SelectOption[T]) error {
	huhOptions := make([]huh.Option[T], 0, len(options))
	for _, opt := range options {
		huhOptions = append(huhOptions, huh.NewOption(opt.Label, opt.Value))
	}

	selector := huh.NewSelect[T]().
		Title(title).
		Options(huhOptions...).
		Value(result)

	if description != "" {
		selector = selector.Description(description)
	}

	return runTypedField(s, ctx, selector)
}

// PromptConfirm prompts the user for a yes/no confirmation.
// The result is stored in the provided result pointer.
// Returns ErrUserCancelled if the user cancels the prompt.
func (s *IOStreams) PromptConfirm(ctx context.Context, result *bool, title, description string, defaultValue bool) error {
	*result = defaultValue

	confirm := huh.NewConfirm().
		Title(title).
		Affirmative("Yes!").
		Negative("No.").
		Value(result)

	if description != "" {
		confirm = confirm.Description(description)
	}

	return s.Run(ctx, confirm)
}

// PromptMultiSelect prompts the user to select multiple options from a list.
// The result is stored in the provided result pointer.
// Returns ErrUserCancelled if the user cancels the prompt.
func (s *IOStreams) PromptMultiSelect(ctx context.Context, result *[]string, title, description string, options []string) error {
	limit := min(len(options), 10)

	multiSelect := huh.NewMultiSelect[string]().
		Title(title).
		Options(huh.NewOptions(options...)...).
		Filterable(false).
		Limit(limit).
		Value(result)

	if description != "" {
		multiSelect = multiSelect.Description(description)
	}

	return s.Run(ctx, multiSelect)
}

// runTypedField is a helper function that runs a single typed huh field and handles error conversion.
// This is used by generic functions that cannot use IOStreams.Run directly.
func runTypedField[T comparable](s *IOStreams, ctx context.Context, field *huh.Select[T]) error {
	group := huh.NewGroup(field)

	form := huh.NewForm(group).
		WithInput(s.In).
		WithOutput(s.StdOut).
		WithShowHelp(false).
		WithTheme(theme.HuhTheme())

	err := form.RunWithContext(ctx)

	if errors.Is(err, huh.ErrUserAborted) {
		fmt.Fprintln(s.StdErr, "Cancelled.")
		return ErrUserCancelled
	}

	return err
}

// FormBuilder provides a fluent API for building multi-field forms without
// requiring direct use of the huh library in command code.
type FormBuilder struct {
	io     *IOStreams
	fields []huh.Field
}

// NewForm creates a new FormBuilder for constructing multi-field forms.
func (s *IOStreams) NewForm() *FormBuilder {
	return &FormBuilder{
		io:     s,
		fields: make([]huh.Field, 0),
	}
}

// AddInput adds a text input field to the form.
func (fb *FormBuilder) AddInput(result *string, title, description, defaultValue string) *FormBuilder {
	if defaultValue != "" {
		*result = defaultValue
	}

	input := huh.NewInput().
		Title(title).
		Value(result)

	if description != "" {
		input = input.Description(description)
	}

	fb.fields = append(fb.fields, input)
	return fb
}

// AddInputWithValidation adds a text input field with validation to the form.
func (fb *FormBuilder) AddInputWithValidation(result *string, title, description, defaultValue string, validator func(string) error) *FormBuilder {
	if defaultValue != "" {
		*result = defaultValue
	}

	input := huh.NewInput().
		Title(title).
		Value(result)

	if description != "" {
		input = input.Description(description)
	}
	if validator != nil {
		input = input.Validate(validator)
	}

	fb.fields = append(fb.fields, input)
	return fb
}

// AddPassword adds a password input field to the form.
func (fb *FormBuilder) AddPassword(result *string, title, description string, validator func(string) error) *FormBuilder {
	input := huh.NewInput().
		Title(title).
		EchoMode(huh.EchoModePassword).
		Value(result)

	if description != "" {
		input = input.Description(description)
	}
	if validator != nil {
		input = input.Validate(validator)
	}

	fb.fields = append(fb.fields, input)
	return fb
}

// AddSelect adds a string selection field to the form.
func (fb *FormBuilder) AddSelect(result *string, title, description string, options []string) *FormBuilder {
	selector := huh.NewSelect[string]().
		Title(title).
		Options(huh.NewOptions(options...)...).
		Value(result)

	if description != "" {
		selector = selector.Description(description)
	}

	fb.fields = append(fb.fields, selector)
	return fb
}

// AddConfirm adds a yes/no confirmation field to the form.
func (fb *FormBuilder) AddConfirm(result *bool, title, description string, defaultValue bool) *FormBuilder {
	*result = defaultValue

	confirm := huh.NewConfirm().
		Title(title).
		Affirmative("Yes!").
		Negative("No.").
		Value(result)

	if description != "" {
		confirm = confirm.Description(description)
	}

	fb.fields = append(fb.fields, confirm)
	return fb
}

// AddMultiSelect adds a multi-select field to the form.
func (fb *FormBuilder) AddMultiSelect(result *[]string, title, description string, options []string) *FormBuilder {
	limit := min(len(options), 10)

	multiSelect := huh.NewMultiSelect[string]().
		Title(title).
		Options(huh.NewOptions(options...)...).
		Filterable(false).
		Limit(limit).
		Value(result)

	if description != "" {
		multiSelect = multiSelect.Description(description)
	}

	fb.fields = append(fb.fields, multiSelect)
	return fb
}

// AddText adds a multiline text input field to the form.
func (fb *FormBuilder) AddText(result *string, title, description, placeholder string) *FormBuilder {
	text := huh.NewText().
		Title(title).
		Value(result)

	if description != "" {
		text = text.Description(description)
	}
	if placeholder != "" {
		text = text.Placeholder(placeholder)
	}

	fb.fields = append(fb.fields, text)
	return fb
}

// Run executes the form and returns any error.
// Returns ErrUserCancelled if the user cancels the form.
func (fb *FormBuilder) Run(ctx context.Context) error {
	if len(fb.fields) == 0 {
		return nil
	}

	return fb.io.RunForm(ctx, fb.fields...)
}

// AddSelectTyped adds a type-safe selection field to the form.
// Since Go doesn't support generic methods, this is a standalone function.
func AddSelectTyped[T comparable](fb *FormBuilder, result *T, title, description string, options []SelectOption[T]) *FormBuilder {
	huhOptions := make([]huh.Option[T], 0, len(options))
	for _, opt := range options {
		huhOptions = append(huhOptions, huh.NewOption(opt.Label, opt.Value))
	}

	selector := huh.NewSelect[T]().
		Title(title).
		Options(huhOptions...).
		Value(result)

	if description != "" {
		selector = selector.Description(description)
	}

	fb.fields = append(fb.fields, selector)
	return fb
}

// AddMultiSelectTyped adds a type-safe multi-select field to the form.
// Since Go doesn't support generic methods, this is a standalone function.
func AddMultiSelectTyped[T comparable](fb *FormBuilder, result *[]T, title, description string, options []SelectOption[T]) *FormBuilder {
	huhOptions := make([]huh.Option[T], 0, len(options))
	for _, opt := range options {
		huhOptions = append(huhOptions, huh.NewOption(opt.Label, opt.Value))
	}

	limit := min(len(options), 10)

	multiSelect := huh.NewMultiSelect[T]().
		Title(title).
		Options(huhOptions...).
		Filterable(false).
		Limit(limit).
		Value(result)

	if description != "" {
		multiSelect = multiSelect.Description(description)
	}

	fb.fields = append(fb.fields, multiSelect)
	return fb
}
