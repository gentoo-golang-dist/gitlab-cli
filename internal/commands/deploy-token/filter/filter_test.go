//go:build !integration

package filter

import (
	"reflect"
	"testing"
)

func TestFilter(t *testing.T) {
	type args[T any] struct {
		s    []T
		test func(t T) bool
	}
	type testCase[T any] struct {
		name string
		args args[T]
		want []T
	}

	type deployToken struct {
		Active bool
		Name   string
	}

	tokens := []deployToken{
		{Active: false, Name: "Token1"},
		{Active: true, Name: "Token1"},
		{Active: false, Name: "Token2"},
		{Active: true, Name: "Token2"},
		{Active: false, Name: "Token3"},
	}

	tests := []testCase[deployToken]{
		{
			name: "find all active tokens",
			args: args[deployToken]{
				s:    tokens,
				test: func(t deployToken) bool { return t.Active },
			},
			want: []deployToken{
				{Active: true, Name: "Token1"},
				{Active: true, Name: "Token2"},
			},
		},
		{
			name: "find active token by name",
			args: args[deployToken]{
				s:    tokens,
				test: func(t deployToken) bool { return t.Active && t.Name == "Token2" },
			},
			want: []deployToken{
				{Active: true, Name: "Token2"},
			},
		},
		{
			name: "find no tokens",
			args: args[deployToken]{
				s:    tokens,
				test: func(t deployToken) bool { return t.Active && t.Name == "Token123" },
			},
			want: []deployToken{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Filter(tt.args.s, tt.args.test); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Filter() = %v, want %v", got, tt.want)
			}
		})
	}
}
