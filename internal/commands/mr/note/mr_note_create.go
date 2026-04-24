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
	message string
	unique  bool
	reply   string

	// Populated in complete.
	client *gitlab.Client
	mr     *gitlab.MergeRequest
	repo   glrepo.Interface
	body   string
}

func NewCmdCreate(f cmdutils.Factory) *cobra.Command {
	opts := &createOptions{
		io:           f.IO(),
		factory:      f,
		gitlabClient: f.GitLabClient,
	}

	cmd := &cobra.Command{
		Use:   "create [<id> | <branch>]",
		Short: "Create a comment or discussion on a merge request. (EXPERIMENTAL)",
		Long: heredoc.Docf(`
			Add a comment to a merge request. The command creates the comment as a new
			discussion thread.

			Use %[1]s--reply%[1]s to add a note to an existing discussion thread instead of
			starting a new one. The value can be a full discussion ID or a unique
			prefix of at least 8 characters.
		`, "`") + text.ExperimentalString,
		Example: heredoc.Doc(`
			# Add a comment to merge request 123
			glab mr note create 123 -m "Looks good to me!"

			# Add a comment to the current branch's merge request
			glab mr note create -m "LGTM"

			# Open editor to compose the message
			glab mr note create 123

			# Pipe from stdin
			echo "LGTM" | glab mr note create 123

			# Skip if already posted
			glab mr note create 123 -m "LGTM" --unique

			# Reply to an existing discussion thread
			glab mr note create 123 --reply abc12345 -m "I agree!"
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
	fl.BoolVar(&opts.unique, "unique", false, "Don't create a note if a note with the same body already exists. Reads all MR comments first.")
	fl.StringVar(&opts.reply, "reply", "", "Reply to an existing discussion. Accepts a full discussion ID or a prefix of 8 or more characters.")

	cmd.MarkFlagsMutuallyExclusive("reply", "unique")

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

	return nil
}

func (o *createOptions) validate() error {
	if strings.TrimSpace(o.body) == "" {
		return fmt.Errorf("aborted... Note has an empty message.")
	}
	if o.reply != "" && len(o.reply) < 8 {
		return fmt.Errorf("discussion ID prefix must be at least 8 characters, got %d", len(o.reply))
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

	return o.runCreate(ctx)
}

func (o *createOptions) runCreate(ctx context.Context) error {
	disc, _, err := o.client.Discussions.CreateMergeRequestDiscussion(
		o.repo.FullName(),
		o.mr.IID,
		&gitlab.CreateMergeRequestDiscussionOptions{Body: &o.body},
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
