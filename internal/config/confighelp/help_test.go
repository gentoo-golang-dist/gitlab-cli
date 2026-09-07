//go:build !integration

package confighelp

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/config"
)

func TestSettings_CoversEveryDocumentedKeyExactlyOnce(t *testing.T) {
	t.Parallel()

	got := Settings()
	lines := strings.Split(got, "\n")

	var want int
	for _, kd := range config.KeySchema {
		if !kd.UserSettable || kd.HelpHidden {
			continue
		}
		want++
		assert.Equal(t, 1, strings.Count(got, "- `"+kd.Name+"`: "),
			"key %q should appear exactly once", kd.Name)
	}
	assert.Len(t, lines, want)
}

func TestSettings_OmitsHiddenAndInternalKeys(t *testing.T) {
	t.Parallel()

	got := Settings()
	for _, kd := range config.KeySchema {
		if kd.UserSettable && !kd.HelpHidden {
			continue
		}
		assert.NotContains(t, got, "- `"+kd.Name+"`: ", "key %q should not be documented", kd.Name)
	}
}

func TestSettings_DocumentsAliases(t *testing.T) {
	t.Parallel()

	got := Settings()
	for _, kd := range config.KeySchema {
		if !kd.UserSettable || kd.HelpHidden {
			continue
		}
		for _, alias := range kd.Aliases {
			assert.Contains(t, got, "`"+alias+"`", "alias %q of %q should be documented", alias, kd.Name)
		}
	}
}

func TestSettings_RendersOneBulletPerLine(t *testing.T) {
	t.Parallel()

	for line := range strings.SplitSeq(Settings(), "\n") {
		assert.True(t, strings.HasPrefix(line, "- `"), "unexpected line: %q", line)
		assert.True(t, strings.HasSuffix(line, "."), "line should end in a period: %q", line)
	}
}

func TestSettings_DocumentsEnvVars(t *testing.T) {
	t.Parallel()

	got := Settings()
	for _, kd := range config.KeySchema {
		if !kd.UserSettable || kd.HelpHidden {
			continue
		}
		for _, name := range config.EnvVarsForKey(kd) {
			assert.Contains(t, got, "`"+name+"`", "env var %q of %q should be documented", name, kd.Name)
		}
	}
}

func TestEnvVarsByGroup_PutsAlternativesUnderTheirPreferredName(t *testing.T) {
	t.Parallel()

	var names []string
	for _, section := range EnvVarsByGroup() {
		assert.NotEmpty(t, section.Vars, "group %q should not be rendered empty", section.Group)
		for _, ev := range section.Vars {
			names = append(names, ev.Name)
		}
	}

	assert.Subset(t, names, []string{"GLAB_GLAMOUR_STYLE", "GLAMOUR_STYLE"})
	assert.Less(t,
		indexOf(t, names, "GLAB_GLAMOUR_STYLE"),
		indexOf(t, names, "GLAMOUR_STYLE"),
		"the preferred name should come first")
}

func TestEnvVarsByGroup_DescriptionsAreOneMarkdownSentence(t *testing.T) {
	t.Parallel()

	for _, section := range EnvVarsByGroup() {
		for _, ev := range section.Vars {
			assert.NotContains(t, ev.Description, "\n", "%s should be flattened", ev.Name)
			assert.True(t, strings.HasSuffix(ev.Description, "."), "%s should end in a period", ev.Name)
			assert.NotRegexp(t, quotedLiteral, ev.Description,
				"%s should have quoted literals marked up as code, leaving possessive apostrophes alone", ev.Name)
		}
	}
}

// TypeList keys never reach Config.Get, so advertising a variable for them
// would document one that does nothing.
func TestEnvVarsForKey_SkipsListKeys(t *testing.T) {
	t.Parallel()

	for _, kd := range config.KeySchema {
		if kd.Type == config.TypeList {
			assert.Empty(t, config.EnvVarsForKey(kd), "list key %q should have no env var", kd.Name)
		}
	}
}

func TestEnvironmentHelp_RendersOneBlockPerVariable(t *testing.T) {
	t.Parallel()

	got := EnvironmentHelp()
	blocks := strings.Split(got, "\n\n")
	assert.Len(t, blocks, len(config.EnvVars()))

	for _, block := range blocks {
		assert.NotContains(t, block, "\n", "each block should be a single line")
		name, _, found := strings.Cut(block, ": ")
		assert.True(t, found, "block should be %q: %q", "NAME: description", block)
		assert.Equal(t, strings.ToUpper(name), name, "block should start with the variable name: %q", block)
	}
}

func indexOf(t *testing.T, haystack []string, needle string) int {
	t.Helper()
	for i, s := range haystack {
		if s == needle {
			return i
		}
	}
	t.Fatalf("%q not found", needle)
	return -1
}
