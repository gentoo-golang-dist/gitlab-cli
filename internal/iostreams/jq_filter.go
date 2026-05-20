package iostreams

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/itchyny/gojq"
)

// JQFilter is the value backing the --jq flag. It implements pflag.Value
// so cobra parses and validates the jq expression as part of flag handling
// (Set runs once, at parse time). IOStreams holds a pointer to one of these
// and PrintJSON delegates filtering to Apply when the filter is active.
type JQFilter struct {
	expr  string
	query *gojq.Query
}

// Set parses and stores the jq expression. It satisfies pflag.Value.
func (j *JQFilter) Set(s string) error {
	if s == "" {
		j.expr = ""
		j.query = nil
		return nil
	}
	q, err := gojq.Parse(s)
	if err != nil {
		return fmt.Errorf("invalid --jq expression: %w", err)
	}
	j.expr = s
	j.query = q
	return nil
}

// String returns the stored expression. It satisfies pflag.Value.
func (j *JQFilter) String() string { return j.expr }

// Type is the cobra-displayed value type. It satisfies pflag.Value.
func (j *JQFilter) Type() string { return "string" }

// IsActive reports whether an expression has been supplied.
func (j *JQFilter) IsActive() bool { return j.query != nil }

// Apply runs the compiled jq query over v and writes results to w. Strings
// produced by the filter are written raw (matching jq -r); other values are
// emitted as compact JSON, one result per line.
func (j *JQFilter) Apply(w io.Writer, v any) error {
	if j.query == nil {
		return fmt.Errorf("Apply called on inactive JQFilter")
	}

	// Round-trip through Marshal/Unmarshal so gojq operates on plain
	// map/slice types instead of arbitrary structs.
	data, err := json.Marshal(v) //nolint:forbidigo // internal to JQFilter; preparing input for gojq
	if err != nil {
		return err
	}
	var input any
	if err := json.Unmarshal(data, &input); err != nil {
		return err
	}

	iter := j.query.Run(input)
	for {
		result, ok := iter.Next()
		if !ok {
			break
		}
		if errResult, isErr := result.(error); isErr {
			return fmt.Errorf("--jq error: %w", errResult)
		}
		// jq -r style: bare strings without surrounding quotes.
		if str, isStr := result.(string); isStr {
			if _, err := fmt.Fprintln(w, str); err != nil {
				return err
			}
			continue
		}
		out, err := json.Marshal(result) //nolint:forbidigo // internal to JQFilter; encoding gojq result
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, string(out)); err != nil {
			return err
		}
	}
	return nil
}
