package cmdutils

import (
	"fmt"
	"maps"
	"slices"
)

type enumValue[T ~string] struct {
	allowed  map[T]struct{}
	valueRef *T
}

func (e *enumValue[T]) Type() string {
	return "string"
}

func (e *enumValue[T]) String() string {
	return string(*e.valueRef)
}

func (e *enumValue[T]) Set(v string) error {
	tv := T(v)
	_, ok := e.allowed[tv]
	if !ok {
		return fmt.Errorf("must be one of %v", slices.Collect(maps.Keys(e.allowed)))
	}
	*e.valueRef = tv
	return nil
}

// AllowedValuer can be implemented by flag.Value types that have a constrained
// set of valid values, so callers (e.g. MCP schema generation) can surface them.
type AllowedValuer interface {
	AllowedValues() []string
}

// AllowedValues returns the sorted list of valid values for this flag.
func (e *enumValue[T]) AllowedValues() []string {
	keys := slices.Collect(maps.Keys(e.allowed))
	result := make([]string, len(keys))
	for i, k := range keys {
		result[i] = string(k)
	}
	slices.Sort(result)
	return result
}

func NewEnumValue[T ~string](allowed []T, d T, v *T) *enumValue[T] {
	if v == nil {
		panic("the given enum flag value cannot be nil")
	}

	m := make(map[T]struct{}, len(allowed))
	for _, a := range allowed {
		m[a] = struct{}{}
	}
	*v = d
	return &enumValue[T]{
		allowed:  m,
		valueRef: v,
	}
}
