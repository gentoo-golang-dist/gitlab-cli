package add

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
)

type ContentGenerator func(root *cobra.Command) string

func NewCmdAdd(f cmdutils.Factory, generateContent ContentGenerator) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Write the skill file to the specified path",
		Long: heredoc.Doc(`
			Write the glab skill file to the specified path.

			The path argument is required. If a file already exists at the
			specified path, you will be prompted to confirm replacement.
			Use --force to replace without prompting.
		`),
		Example: heredoc.Doc(`
			$ glab skill add ./SKILL.md
			$ glab skill add ~/.config/claude/skills/glab/SKILL.md
			$ glab skill add ./SKILL.md --force
		`),
		Args: cobra.ExactArgs(1),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]

			if _, err := os.Stat(path); err == nil {
				if !force {
					var shouldReplace bool
					if err := f.IO().Confirm(cmd.Context(), &shouldReplace, fmt.Sprintf("File already exists at %s. Replace?", path)); err != nil {
						return err
					}
					if !shouldReplace {
						return cmdutils.SilentError
					}
				}
			}

			content := generateContent(cmd.Root())

			if dir := filepath.Dir(path); dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Errorf("failed to create directory: %w", err)
				}
			}

			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}

			fmt.Fprintf(f.IO().StdOut, "Wrote skill to %s\n", path)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Replace existing file without prompting")

	return cmd
}
