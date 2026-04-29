//go:build !integration

package diff

import (
	"testing"
)

func TestParse(t *testing.T) {
	// Typical unified diff fragment from GitLab
	diffText := `@@ -1,5 +1,6 @@
 line1
 line2
+added line
 line3
-removed line
 line5
`

	lines := Parse(diffText)

	// Expected:
	// line1: unchanged old=1 new=1
	// line2: unchanged old=2 new=2
	// added line: added new=3
	// line3: unchanged old=3 new=4
	// removed line: removed old=4
	// line5: unchanged old=5 new=5

	expected := []Line{
		{Unchanged, 1, 1},
		{Unchanged, 2, 2},
		{Added, 0, 3},
		{Unchanged, 3, 4},
		{Removed, 4, 0},
		{Unchanged, 5, 5},
	}

	if len(lines) != len(expected) {
		t.Fatalf("got %d lines, want %d", len(lines), len(expected))
	}

	for i, got := range lines {
		want := expected[i]
		if got != want {
			t.Errorf("line %d: got %+v, want %+v", i, got, want)
		}
	}
}

func TestFindNewLine(t *testing.T) {
	lines := []Line{
		{Unchanged, 1, 1},
		{Added, 0, 2},
		{Unchanged, 2, 3},
	}

	tests := []struct {
		name      string
		target    int
		wantOld   int
		wantType  LineType
		wantError bool
	}{
		{name: "added line", target: 2, wantOld: 0, wantType: Added},
		{name: "unchanged line", target: 3, wantOld: 2, wantType: Unchanged},
		{name: "not found", target: 99, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldLine, lt, err := FindNewLine(lines, tt.target)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if lt != tt.wantType || oldLine != tt.wantOld {
				t.Errorf("got oldLine=%d type=%d, want oldLine=%d type=%d", oldLine, lt, tt.wantOld, tt.wantType)
			}
		})
	}
}

func TestParseBlankContextLines(t *testing.T) {
	// Blank context lines may appear as empty strings when trailing
	// whitespace is stripped from the diff (the " " becomes "").
	// The parser must count them as unchanged context lines.
	diffText := "@@ -10,5 +10,6 @@\n line10\n+added\n\n line12\n-removed\n line14\n"
	// The empty line between "+added" and " line12" is a blank context line.

	lines := Parse(diffText)

	expected := []Line{
		{Unchanged, 10, 10}, // line10
		{Added, 0, 11},      // added
		{Unchanged, 11, 12}, // blank context line (empty in diff)
		{Unchanged, 12, 13}, // line12
		{Removed, 13, 0},    // removed
		{Unchanged, 14, 14}, // line14
	}

	if len(lines) != len(expected) {
		t.Fatalf("got %d lines, want %d\nlines: %+v", len(lines), len(expected), lines)
	}

	for i, got := range lines {
		want := expected[i]
		if got != want {
			t.Errorf("line %d: got %+v, want %+v", i, got, want)
		}
	}
}

func TestFindOldLine(t *testing.T) {
	lines := []Line{
		{Unchanged, 1, 1},
		{Removed, 2, 0},
		{Unchanged, 3, 2},
	}

	tests := []struct {
		name      string
		target    int
		wantNew   int
		wantType  LineType
		wantError bool
	}{
		{name: "removed line", target: 2, wantNew: 0, wantType: Removed},
		{name: "unchanged line", target: 1, wantNew: 1, wantType: Unchanged},
		{name: "not found", target: 99, wantError: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newLine, lt, err := FindOldLine(lines, tt.target)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if lt != tt.wantType || newLine != tt.wantNew {
				t.Errorf("got newLine=%d type=%d, want newLine=%d type=%d", newLine, lt, tt.wantNew, tt.wantType)
			}
		})
	}
}
