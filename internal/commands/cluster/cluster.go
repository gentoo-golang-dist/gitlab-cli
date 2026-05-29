package cluster

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	agentCmd "gitlab.com/gitlab-org/cli/internal/commands/cluster/agent"
	graphCmd "gitlab.com/gitlab-org/cli/internal/commands/cluster/graph"
)

func NewCmdCluster(f cmdutils.Factory) *cobra.Command {
	clusterCmd := &cobra.Command{
		Use:   "cluster <command> [flags]",
		Short: `Manage GitLab Agents for Kubernetes and their clusters.`,
		Long: heredoc.Doc(`
			Agents connect your cluster to GitLab, enabling pull-based deployments
			and secure access to the Kubernetes API.
		`),
	}

	cmdutils.EnableRepoOverride(clusterCmd, f)

	clusterCmd.AddCommand(agentCmd.NewCmdAgent(f))
	clusterCmd.AddCommand(graphCmd.NewCmdGraph(f))

	return clusterCmd
}
