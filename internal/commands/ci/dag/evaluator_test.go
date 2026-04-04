//go:build !integration

package dag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvalExpression_Truthiness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr string
		vars map[string]string
		want bool
	}{
		{"set variable is truthy", "$VAR", map[string]string{"VAR": "hello"}, true},
		{"empty variable is falsy", "$VAR", map[string]string{"VAR": ""}, false},
		{"unset variable is falsy", "$VAR", map[string]string{}, false},
		{"negated set variable", "!$VAR", map[string]string{"VAR": "hello"}, false},
		{"negated unset variable", "!$VAR", map[string]string{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := EvalExpression(tt.expr, tt.vars)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestEvalExpression_Equality(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr string
		vars map[string]string
		want bool
	}{
		{"equal strings", `$VAR == "hello"`, map[string]string{"VAR": "hello"}, true},
		{"unequal strings", `$VAR == "hello"`, map[string]string{"VAR": "world"}, false},
		{"not equal match", `$VAR != "hello"`, map[string]string{"VAR": "world"}, true},
		{"not equal no match", `$VAR != "hello"`, map[string]string{"VAR": "hello"}, false},
		{"single quoted string", `$VAR == 'hello'`, map[string]string{"VAR": "hello"}, true},
		{"variable to variable", `$A == $B`, map[string]string{"A": "x", "B": "x"}, true},
		{"variable to variable unequal", `$A == $B`, map[string]string{"A": "x", "B": "y"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := EvalExpression(tt.expr, tt.vars)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestEvalExpression_NullCheck(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr string
		vars map[string]string
		want bool
	}{
		{"unset var equals null", "$VAR == null", map[string]string{}, true},
		{"empty var equals null", "$VAR == null", map[string]string{"VAR": ""}, true},
		{"set var not equals null", "$VAR == null", map[string]string{"VAR": "x"}, false},
		{"set var != null", "$VAR != null", map[string]string{"VAR": "x"}, true},
		{"unset var != null", "$VAR != null", map[string]string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := EvalExpression(tt.expr, tt.vars)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestEvalExpression_Regex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr string
		vars map[string]string
		want bool
	}{
		{"regex match", `$VAR =~ /^hello/`, map[string]string{"VAR": "hello world"}, true},
		{"regex no match", `$VAR =~ /^hello/`, map[string]string{"VAR": "world"}, false},
		{"regex not match operator", `$VAR !~ /^hello/`, map[string]string{"VAR": "world"}, true},
		{"regex case insensitive", `$VAR =~ /hello/i`, map[string]string{"VAR": "HELLO"}, true},
		{"regex alternation", `$VAR =~ /^(push|merge_request)$/`, map[string]string{"VAR": "push"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := EvalExpression(tt.expr, tt.vars)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestEvalExpression_BooleanOps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr string
		vars map[string]string
		want bool
	}{
		{"and both true", "$A && $B", map[string]string{"A": "1", "B": "1"}, true},
		{"and one false", "$A && $B", map[string]string{"A": "1"}, false},
		{"or one true", "$A || $B", map[string]string{"A": "1"}, true},
		{"or both false", "$A || $B", map[string]string{}, false},
		{"and takes precedence over or", "$A || $B && $C", map[string]string{"A": "1"}, true},
		{"parentheses override precedence", "($A || $B) && $C", map[string]string{"A": "1"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := EvalExpression(tt.expr, tt.vars)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestEvalExpression_RealWorldExpressions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr string
		vars map[string]string
		want bool
	}{
		{
			"MR pipeline source check",
			`$CI_PIPELINE_SOURCE == "merge_request_event"`,
			map[string]string{"CI_PIPELINE_SOURCE": "merge_request_event"},
			true,
		},
		{
			"tag check truthy",
			"$CI_COMMIT_TAG",
			map[string]string{"CI_COMMIT_TAG": "v1.0.0"},
			true,
		},
		{
			"tag check falsy",
			"$CI_COMMIT_TAG",
			map[string]string{},
			false,
		},
		{
			"branch equals default branch",
			"$CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH",
			map[string]string{"CI_COMMIT_BRANCH": "main", "CI_DEFAULT_BRANCH": "main"},
			true,
		},
		{
			"complex AND with OR and parentheses",
			`$CI_COMMIT_TAG && $CI_SERVER_HOST == "gitlab.com" && ($CI_PROJECT_PATH == "gitlab-org/cli" || $CI_PROJECT_PATH == "gitlab-org/security/cli")`,
			map[string]string{
				"CI_COMMIT_TAG":   "v1.0",
				"CI_SERVER_HOST":  "gitlab.com",
				"CI_PROJECT_PATH": "gitlab-org/cli",
			},
			true,
		},
		{
			"complex AND with OR - wrong project",
			`$CI_COMMIT_TAG && $CI_SERVER_HOST == "gitlab.com" && ($CI_PROJECT_PATH == "gitlab-org/cli" || $CI_PROJECT_PATH == "gitlab-org/security/cli")`,
			map[string]string{
				"CI_COMMIT_TAG":   "v1.0",
				"CI_SERVER_HOST":  "gitlab.com",
				"CI_PROJECT_PATH": "other/project",
			},
			false,
		},
		{
			"AND with negation",
			`$CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH && $CI_PIPELINE_SOURCE != "merge_request_event"`,
			map[string]string{
				"CI_COMMIT_BRANCH":   "main",
				"CI_DEFAULT_BRANCH":  "main",
				"CI_PIPELINE_SOURCE": "push",
			},
			true,
		},
		{
			"MR IID truthy with visibility check",
			`$CI_MERGE_REQUEST_IID && $CI_PROJECT_VISIBILITY == "public"`,
			map[string]string{
				"CI_MERGE_REQUEST_IID":  "42",
				"CI_PROJECT_VISIBILITY": "public",
			},
			true,
		},
		{
			"merge train check",
			`$CI_MERGE_REQUEST_EVENT_TYPE == "merge_train"`,
			map[string]string{"CI_MERGE_REQUEST_EVENT_TYPE": "merge_train"},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := EvalExpression(tt.expr, tt.vars)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}
