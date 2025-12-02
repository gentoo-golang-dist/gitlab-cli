package attestation

import (
	"github.com/MakeNowJust/heredoc/v2"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	attestationVerifyCmd "gitlab.com/gitlab-org/cli/internal/commands/attestation/verify"

	"github.com/spf13/cobra"
)

func NewCmdAttestation(f cmdutils.Factory) *cobra.Command {
	attestationCmd := &cobra.Command{
		Use:   "attestation <command> [flags]",
		Short: `Functionality related to software attestations, e.g. SLSA. (EXPERIMENTAL)`,
		Long:  ``,
		Example: heredoc.Doc(`
			$ glab attestation verify filename.txt --project gilab-org/gitlab
			$ glab attestation verify filename.txt --project 123
		`),
		Annotations: map[string]string{
			"help:arguments": heredoc.Doc(`
			A project can be supplied as argument in any of the following formats:
			- by number, e.g. "123"; or
			- by path, e.g. "gitlab-org/cli"
			`),
		},
	}

	attestationCmd.AddCommand(attestationVerifyCmd.NewCmdVerify(f))

	return attestationCmd
}
