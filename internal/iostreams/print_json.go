package iostreams

import (
	"encoding/json"
	"reflect"
)

// PrintJSON marshals v to JSON and writes it to stdout. If v is a nil slice,
// it converts it to an empty slice so that JSON marshaling produces [] instead
// of null. This addresses the issue where gitlab.ScanAndCollect returns nil for
// empty results, which would otherwise marshal as null instead of [].
//
// Nested slices within the data structure are left as-is to preserve the
// semantic difference between absent fields (null) and empty arrays ([]) in
// the original API response.
//
// When IOStreams.JQ is active (a --jq expression was supplied), the value is
// passed through the filter before being written.
func (s *IOStreams) PrintJSON(v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice && rv.IsNil() {
		v = reflect.MakeSlice(rv.Type(), 0, 0).Interface()
	}

	if s.JQ != nil && s.JQ.IsActive() {
		return s.JQ.Apply(s.StdOut, v)
	}

	encoder := json.NewEncoder(s.StdOut) //nolint:forbidigo // this is the PrintJSON helper itself
	return encoder.Encode(v)
}
