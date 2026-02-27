package cmdutils

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewBoolFlagPair sets up two mutually exclusive boolean flags that can be mapped to the same underlying value.
// For example a `--pause` and `--unpause`, where `--pause` sets the underlying value to `true` and `--unpause`
// to false. Not setting either of the flags results the underlying to be `nil`.
func NewBoolFlagPair(cmd *cobra.Command, v **bool, onName, onDesc, offName, offDesc string) {
	fl := cmd.Flags()
	fl.Var(&boolPtrFlag{target: v, setValue: true}, onName, onDesc)
	fl.Lookup(onName).NoOptDefVal = "true"
	fl.Var(&boolPtrFlag{target: v, setValue: false}, offName, offDesc)
	fl.Lookup(offName).NoOptDefVal = "true"

	cmd.MarkFlagsMutuallyExclusive(onName, offName)
}

// boolPtrFlag is a pflag Value that sets a *bool to a fixed value when the flag is present.
type boolPtrFlag struct {
	target   **bool
	setValue bool
}

func (f *boolPtrFlag) String() string {
	if f.target == nil || *f.target == nil {
		return ""
	}

	return fmt.Sprintf("%v", **f.target)
}

func (f *boolPtrFlag) Set(val string) error {
	switch val {
	case "true", "":
		*f.target = &f.setValue
		return nil
	case "false":
		v := false
		*f.target = &v
		return nil
	default:
		return fmt.Errorf("invalid value for flag: %s", val)
	}
}

func (f *boolPtrFlag) Type() string { return "bool" }

func (f *boolPtrFlag) IsBoolFlag() bool { return true }
