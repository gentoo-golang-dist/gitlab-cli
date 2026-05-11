package note

import (
	"context"
	"fmt"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"

	gitlab "gitlab.com/gitlab-org/api/client-go/v2"

	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/text"
)

type createOptions struct {
	io           *iostreams.IOStreams
	factory      cmdutils.Factory
	gitlabClient func() (*gitlab.Client, error)

	// Flags.
	message  string
	unique   bool
	reply    string
	filePath string
	line     string
	oldLine  int
	thread   bool

	// Populated in complete.
	client   *gitlab.Client
	mr       *gitlab.MergeRequest
	repo     glrepo.Interface
	body     string
	position *gitlab.PositionOptions
}

func NewCmdCreate(f cmdutils.Factory) *cobra.Command {
	opts := &createOptions{
		io:           f.IO(),
		factory:      f,
		gitlabClient: f.GitLabClient,
	}

	cmd := &cobra.Command{
		Use:   "create [<id> | <branch>]",
		Short: "Create a note or thread on a merge request. (EXPERIMENTAL)",
		Long: heredoc.Docf(`
			Add a note to a merge request. By default, the command posts a
			non-resolvable note that does not block merging when the project requires
			"all threads resolved before merging".

			Use %[1]s--thread%[1]s to create a note with a resolvable thread instead.
			Thread mode is also implied automatically by %[1]s--reply%[1]s,
			%[1]s--file%[1]s, %[1]s--line%[1]s, and %[1]s--old-line%[1]s, since those
			operate on threads.

			Note: earlier versions of this command (introduced in v1.93.0) always
			created a resolvable thread. The default switched to a non-resolvable
			note to restore the long-standing behavior of the deprecated
			%[1]sglab mr note -m%[1]s. Pass %[1]s--thread%[1]s to opt into the
			previous default.

			Use %[1]s--reply%[1]s to add a note to an existing thread instead of
			starting a new one. The value can be a full discussion ID or a unique
			prefix of at least 8 characters.

			Use %[1]s--file%[1]s to place a diff comment on a specific file in the latest
			merge request diff version. Combine with %[1]s--line%[1]s (new side) or
			%[1]s--old-line%[1]s (old/removed side) to target a specific line. Omit
			both flags for a file-level comment.

			The flag rules are:

			- %[1]s--line%[1]s and %[1]s--old-line%[1]s require %[1]s--file%[1]s, and
			cannot be used together.
			- %[1]s--file%[1]s, %[1]s--reply%[1]s, and %[1]s--unique%[1]s are mutually
			exclusive.
		`, "`") + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Add a note to merge request 123 (does not create a thread)
			glab mr note create 123 -m "Looks good to me!"

			# Add a note to the current branch's merge request
			glab mr note create -m "LGTM"

			# Create a note with a resolvable thread
			glab mr note create 123 --thread -m "Please rename this function"

			# Open editor to compose the message
			glab mr note create 123

			# Pipe from stdin
			echo "LGTM" | glab mr note create 123

			# Skip if already posted
			glab mr note create 123 -m "LGTM" --unique

			# Reply to an existing thread (--thread implied)
			glab mr note create 123 --reply abc12345 -m "I agree!"

			# Add a diff comment on line 42 of main.go
			glab mr note create 123 --file main.go --line 42 -m "Needs refactoring"

			# Add a diff comment on lines 10-15 (multiline range)
			glab mr note create 123 --file main.go --line 10:15 -m "Extract this block"

			# Add a diff comment on a removed line (old side)
			glab mr note create 123 --file main.go --old-line 7 -m "Why was this removed?"

			# Add a file-level diff comment (no line specified)
			glab mr note create 123 --file main.go -m "General comment on this file"
		`),
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(cmd, args); err != nil {
				return err
			}
			if err := opts.validate(); err != nil {
				return err
			}
			return opts.run(cmd.Context())
		},
	}

	fl := cmd.Flags()
	fl.StringVarP(&opts.message, "message", "m", "", "Comment or note message.")
	fl.BoolVar(&opts.unique, "unique", false, "Don't create a note if a note with the same body already exists. Reads all merge request comments first.")
	fl.StringVar(&opts.reply, "reply", "", "Reply to an existing discussion. Accepts a full discussion ID or a prefix of 8 or more characters.")
	fl.StringVar(&opts.filePath, "file", "", "File path for a diff comment, like <path/to/file>. Targets the latest merge request diff version.")
	fl.StringVar(&opts.line, "line", "", "Line in the new version. A single line number, like 42, or a range, like 10:15.")
	fl.IntVar(&opts.oldLine, "old-line", 0, "Line in the old version, for commenting on a removed line.")
	fl.BoolVarP(&opts.thread, "thread", "t", false, "Create a resolvable thread instead of a non-resolvable note. Implied by --reply, --file, --line, and --old-line. (default false)")

	cmd.MarkFlagsMutuallyExclusive("reply", "unique")
	cmd.MarkFlagsMutuallyExclusive("reply", "file")
	cmd.MarkFlagsMutuallyExclusive("unique", "file")
	cmd.MarkFlagsMutuallyExclusive("line", "old-line")

	return cmd
}

func (o *createOptions) complete(cmd *cobra.Command, args []string) error {
	client, err := o.gitlabClient()
	if err != nil {
		return err
	}
	o.client = client

	mr, repo, err := mrutils.MRFromArgs(cmd.Context(), o.factory, args, "any")
	if err != nil {
		return err
	}
	o.mr = mr
	o.repo = repo

	body := o.message
	if strings.TrimSpace(body) == "" {
		body, err = getBodyFromStdinOrEditor(o.factory, cmd)
		if err != nil {
			return err
		}
	}
	o.body = body

	if o.filePath != "" {
		lineStart, lineEnd, err := mrutils.ParseLine(o.line)
		if err != nil {
			return err
		}

		version, err := mrutils.GetLatestDiffVersion(o.client, o.repo.FullName(), o.mr.IID)
		if err != nil {
			return err
		}

		fileDiff, err := mrutils.FindFileDiff(version, o.filePath)
		if err != nil {
			return err
		}

		position, err := mrutils.BuildDiffPosition(version, fileDiff, lineStart, lineEnd, o.oldLine)
		if err != nil {
			return err
		}

		o.position = position
	}

	return nil
}

func (o *createOptions) validate() error {
	if strings.TrimSpace(o.body) == "" {
		return fmt.Errorf("aborted... Note has an empty message.")
	}
	if o.reply != "" && len(o.reply) < 8 {
		return fmt.Errorf("discussion ID prefix must be at least 8 characters, got %d", len(o.reply))
	}
	if (o.line != "" || o.oldLine != 0) && o.filePath == "" {
		return fmt.Errorf("--line and --old-line require --file")
	}
	return nil
}

func (o *createOptions) run(ctx context.Context) error {
	switch {
	case o.reply != "":
		return o.runReply(ctx)
	case o.unique:
		found, err := deduplicateNote(o.client, o.repo.FullName(), o.mr.IID, o.body, o.mr.WebURL, o.io.StdOut)
		if err != nil {
			return err
		}
		if found {
			return nil
		}
	}

	// Diff comments and explicit --thread route through the Discussions API;
	// otherwise post a non-resolvable note via the Notes API.
	if o.thread || o.position != nil {
		return o.runCreateThread(ctx)
	}
	return o.runCreateNote(ctx)
}

func (o *createOptions) runCreateThread(ctx context.Context) error {
	createOpts := &gitlab.CreateMergeRequestDiscussionOptions{Body: &o.body}
	if o.position != nil {
		createOpts.Position = o.position
	}

	disc, _, err := o.client.Discussions.CreateMergeRequestDiscussion(
		o.repo.FullName(),
		o.mr.IID,
		createOpts,
		gitlab.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to create discussion: %w", err)
	}

	if len(disc.Notes) == 0 {
		return fmt.Errorf("discussion created but returned no notes")
	}

	fmt.Fprintf(o.io.StdOut, "%s#note_%d\n", o.mr.WebURL, disc.Notes[0].ID)
	return nil
}

func (o *createOptions) runCreateNote(ctx context.Context) error {
	note, _, err := o.client.Notes.CreateMergeRequestNote(
		o.repo.FullName(),
		o.mr.IID,
		&gitlab.CreateMergeRequestNoteOptions{Body: &o.body},
		gitlab.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to create note: %w", err)
	}

	fmt.Fprintf(o.io.StdOut, "%s#note_%d\n", o.mr.WebURL, note.ID)
	return nil
}

func (o *createOptions) runReply(ctx context.Context) error {
	discussionID, err := mrutils.ResolveDiscussionID(ctx, o.client, o.repo.FullName(), o.mr.IID, o.reply)
	if err != nil {
		return err
	}

	note, _, err := o.client.Discussions.AddMergeRequestDiscussionNote(
		o.repo.FullName(),
		o.mr.IID,
		discussionID,
		&gitlab.AddMergeRequestDiscussionNoteOptions{Body: &o.body},
		gitlab.WithContext(ctx),
	)
	if err != nil {
		return fmt.Errorf("failed to add reply: %w", err)
	}

	fmt.Fprintf(o.io.StdOut, "%s#note_%d\n", o.mr.WebURL, note.ID)
	return nil
}
