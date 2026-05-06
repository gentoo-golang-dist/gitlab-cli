//go:build !integration

package stack

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/cli/internal/testing/cmdtest"
	"gitlab.com/gitlab-org/cli/internal/text"
)

func TestStackCmd(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewCmdStack(cmdtest.NewTestFactory(nil))
	cmd.SetOut(&buf)

	assert.Nil(t, cmd.Execute())

	assert.Contains(t, buf.String(), "Stacked diffs are a way of creating small changes that build upon each other to ultimately deliver")
	assert.Contains(t, buf.String(), text.ExperimentalString)
}
