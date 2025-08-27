package downloadall

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"

	"github.com/MakeNowJust/heredoc/v2"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/securefile/download"
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
			client, err := f.GitLabClient()
			if err != nil {
				return err
			}

			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}

			l := &gitlab.ListProjectSecureFilesOptions{
				Page:    1,
				PerPage: api.MaxPerPage,
			}

			files, _, err := client.SecureFiles.ListProjectSecureFiles(repo.FullName(), l)
			if err != nil {
				return fmt.Errorf("Error fetching list of secure files: %v", err)
			}

			if len(files) == 0 {
				return nil
			}

			path, err := cmd.Flags().GetString("path")
			if err != nil {
				return fmt.Errorf("Unable to get path flag: %v", err)
			}

			err = download.CreateDirectory(path)
			if err != nil {
				return err
			}

			for _, file := range files {
				filePath, err := securejoin.SecureJoin(path, file.Name)
				if err != nil {
					return err
				}

				if err := download.SaveFile(client, repo, file.ID, filePath); err != nil {
					return err
				}

				if err := verifyChecksum(*file, filePath); err != nil {
					return err
				}
			}

			return nil
		},
	}

	securefileDownloadAllCmd.Flags().StringVarP(&opts.dir, "path", "p", ".", "Path to download the secure files to")

	return securefileDownloadAllCmd
}

func verifyChecksum(file gitlab.SecureFile, filePath string) error {
	body, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	sum := sha256.Sum256(body)

	if hex.EncodeToString(sum[:]) == file.Checksum {
		return nil
	}

	return fmt.Errorf("Failure validating checksum for %s", file.Name)
}
