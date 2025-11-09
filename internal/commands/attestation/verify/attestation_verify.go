package approve

import (
	"fmt"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
)

func NewCmdVerify(f cmdutils.Factory) *cobra.Command {
	attestationVerifyCmd := &cobra.Command{
		Use:   "verify",
		Short: `Verify the provenance of a specific artifact or file`,
		Long:  ``,
		Example: heredoc.Doc(`
			$ glab attestation verify filename.txt --project gilab-org/gitlab
			$ glab attestation verify filename.txt --project 123
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("WFAWFAWFA")
			return nil
		},
	}

	attestationVerifyCmd.Flags().StringP("sha", "s", "", "SHA, which must match the SHA of the HEAD commit of the merge request.")

	return attestationVerifyCmd
}
