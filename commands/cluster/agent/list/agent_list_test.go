package list

import (
	"testing"
	"time"

	"github.com/MakeNowJust/heredoc/v2"
	"go.uber.org/mock/gomock"

	"gitlab.com/gitlab-org/cli/commands/cmdtest"

	"github.com/stretchr/testify/assert"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

func TestAgentList(t *testing.T) {
	// GIVEN
	tc, exec := cmdtest.Setup(t, NewCmdAgentList, true, "")

	tc.MockClusterAgents.EXPECT().
		ListAgents(gomock.Any(), &gitlab.ListAgentsOptions{Page: 1, PerPage: 30}).
		Return([]*gitlab.Agent{
			{
				ID:        1,
				Name:      "local",
				CreatedAt: gitlab.Ptr(time.Now().Add(-24 * time.Hour)),
			},
			{
				ID:        2,
				Name:      "prd",
				CreatedAt: gitlab.Ptr(time.Now().Add(-24 * time.Hour)),
			},
		}, &gitlab.Response{}, nil).
		Times(1)

	// WHEN
	output, err := exec("")
	if err != nil {
		t.Errorf("error running command `cluster agent list`: %v", err)
	}

	// THEN
	assert.Equal(t, heredoc.Doc(`
		Showing 2 agents on OWNER/REPO. (Page 1)

		ID	Name	Created At
		1	local	about 1 day ago
		2	prd	about 1 day ago

	`), output.String())
	assert.Equal(t, ``, output.Stderr())
}

func TestAgentList_Pagination(t *testing.T) {
	// GIVEN
	tc, exec := cmdtest.Setup(t, NewCmdAgentList, true, "")

	tc.MockClusterAgents.EXPECT().
		ListAgents(gomock.Any(), &gitlab.ListAgentsOptions{Page: 2, PerPage: 1}).
		Return([]*gitlab.Agent{
			{
				ID:        2,
				Name:      "prd",
				CreatedAt: gitlab.Ptr(time.Now().Add(-24 * time.Hour)),
			},
		}, &gitlab.Response{NextPage: 0}, nil)

	// WHEN
	output, err := exec("--page 2 --per-page 1")
	if err != nil {
		t.Errorf("error running command `cluster agent list`: %v", err)
	}

	assert.Equal(t, heredoc.Doc(`
		Showing 1 agent on OWNER/REPO. (Page 2)

		ID	Name	Created At
		2	prd	about 1 day ago

	`), output.String())
	assert.Equal(t, ``, output.Stderr())
}
