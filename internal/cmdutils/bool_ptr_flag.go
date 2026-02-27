package cmdutils

import "fmt"

// BoolPtrFlag is a pflag Value that sets a *bool to a fixed value when the flag is present.
// Use it for flags that map to the same field with three states: unset (nil), true, or false.
// For example, --pause sets the target to &true, --unpause sets it to &false.
type BoolPtrFlag struct {
	target   **bool
	setValue bool
}

// NewBoolPtrFlag returns a flag value that, when set, assigns &setValue to *target.
// target is a pointer to the *bool field (e.g. &opts.pause).
func NewBoolPtrFlag(target **bool, setValue bool) *BoolPtrFlag {
	return &BoolPtrFlag{target: target, setValue: setValue}
}

func (f *BoolPtrFlag) String() string {
	if f.target == nil || *f.target == nil {
		return ""
	}
	return fmt.Sprintf("%v", **f.target)
}

func (f *BoolPtrFlag) Set(val string) error {
	if val == "true" || val == "" {
		*f.target = &f.setValue
		return nil
	}
	if val == "false" {
		v := false
		*f.target = &v
		return nil
	}
	return fmt.Errorf("invalid value for flag: %s", val)
}

func (f *BoolPtrFlag) Type() string { return "bool" }

// IsBoolFlag returns true so pflag treats the flag as a boolean (no argument required when using NoOptDefVal).
func (f *BoolPtrFlag) IsBoolFlag() bool { return true }
