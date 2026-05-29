package get

import (
	"fmt"
	"strconv"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

func NewCmdGet(f cmdutils.Factory) *cobra.Command {
	securefileGetCmd := &cobra.Command{
		Use:   "get <id>",
		Short: `Get details of a secure file by ID.`,
		Long: heredoc.Docf(`
		Get details of a single secure file in a project, identified by its
		numeric ID. The response includes the file's name, checksum, and
		associated metadata.

		This command requires GitLab 18.0 or later.

		By default, the file is looked up in the current project. Use
		%[1]s--repo%[1]s to target another project.
		`, "`"),
		Aliases: []string{"show"},
		Args:    cobra.ExactArgs(1),
		Example: heredoc.Doc(`
			# Get details of a secure file by ID
			glab securefile get 1

			# Get details using the 'show' alias
			glab securefile show 1

			# Get details from another project
			glab securefile get 1 -R owner/repo
		`),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.GitLabClient()
			if err != nil {
				return err
			}

			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}

			fileID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("Secure file ID must be an integer: %s", args[0])
			}

			file, _, err := client.SecureFiles.ShowSecureFileDetails(repo.FullName(), int64(fileID))
			if err != nil {
				return fmt.Errorf("Error getting secure file: %v", err)
			}

			return f.IO().PrintJSON(file)
		},
	}

	cmdutils.AddJQFlag(securefileGetCmd, f.IO())
	return securefileGetCmd
}
