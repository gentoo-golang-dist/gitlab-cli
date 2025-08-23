package downloadall

import (
	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
)

type options struct {
	dir string
}

func NewCmdDownloadAll(f cmdutils.Factory) *cobra.Command {
	opts := &options{}

	securefileDownloadAllCmd := &cobra.Command{
		Use:   "download-all [flags]",
		Short: `Download all secure files for a project.`,
		Example: heredoc.Doc(`
		    Download all (liimit 100) secure files for a project to the current directory and verify checksums.
		    - glab securefile download-all

		    Download all (limit 100) secure files for a project to the current directory and verify checksums to a given path.
		    - glab securefile download-all --path="securefiles/"
		`),
		Long: heredoc.Doc(`
		    Download all secure files for a project, and verify their checksums.
			A maximum of 100 secure files can be downloaded.
			Files are saved to the specified directory with their original names.
			If no directory is specified, files are downloaded to the current directory.
		`),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// client, err := f.GitLabClient()
			// if err != nil {
			// 	return err
			// }
			// return nil
		},
	}

	securefileDownloadAllCmd.Flags().StringVarP(&opts.dir, "path", "p", ".", "Path to download the secure files to")

	return securefileDownloadAllCmd
}
