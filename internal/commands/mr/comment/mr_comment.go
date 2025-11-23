package comment

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gitlab.com/gitlab-org/cli/internal/mcpannotations"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/mr/mrutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/utils"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

var listMRNotes = func(client *gitlab.Client, projectID any, mrID int, opts *gitlab.ListMergeRequestNotesOptions) ([]*gitlab.Note, error) {
	if opts.PerPage == 0 {
		opts.PerPage = api.DefaultListLimit
	}
	notes, _, err := client.Notes.ListMergeRequestNotes(projectID, mrID, opts)
	if err != nil {
		return nil, err
	}
	return notes, nil
}

type commandType string

const (
	commandTypeList  commandType = "list"
	commandTypeAdd   commandType = "add"
	commandTypeReply commandType = "reply"
)

type options struct {
	commandType    commandType
	showSystemLogs bool
	outputFormat   string
	pageNumber     int
	perPage        int
	message        string
	commentID      int
	mrID           string
	unique         bool

	io              *iostreams.IOStreams
	gitlabClient    func() (*gitlab.Client, error)
	apiClient       func(repoHost string) (*api.Client, error)
	baseRepo        func() (glrepo.Interface, error)
	config          func() config.Config
	defaultHostname string
}

func NewCmdComment(f cmdutils.Factory) *cobra.Command {
	commentCmd := &cobra.Command{
		Use:     "comment <command>",
		Aliases: []string{"note"},
		Short:   "Manage comments on merge requests",
		Long: "List, add, and reply to comments on merge requests.\n\n" +
			"Note: 'comment' is the preferred terminology. The 'note' alias is provided for backward compatibility.\n",
		Example: heredoc.Doc(`
			$ glab mr comment list 123
			$ glab mr comment add 123 -m "Great work!"
			$ glab mr comment reply 123 456 -m "Thanks for the feedback"
			$ glab mr note 123 -m "Quick comment"
		`) + "\n",
		RunE: func(cmd *cobra.Command, args []string) error {
			// If called directly (without subcommand) via "note" alias, route to "add"
			if cmd.CalledAs() == "note" {
				addCmd := NewCmdAdd(f)
				return addCmd.RunE(cmd, args)
			}
			return cmd.Help()
		},
	}

	commentCmd.AddCommand(NewCmdList(f))
	commentCmd.AddCommand(NewCmdAdd(f))
	commentCmd.AddCommand(NewCmdReply(f))

	return commentCmd
}

func NewCmdList(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		commandType:     commandTypeList,
		io:              f.IO(),
		gitlabClient:    f.GitLabClient,
		apiClient:       f.ApiClient,
		baseRepo:        f.BaseRepo,
		config:          f.Config,
		defaultHostname: f.DefaultHostname(),
	}

	cmd := &cobra.Command{
		Use:   "list <id>",
		Short: "List comments on a merge request",
		Long:  "List all comments and notes on the specified merge request\n",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(args); err != nil {
				return err
			}

			if err := opts.validate(); err != nil {
				return err
			}

			return opts.run(f)
		},
	}

	fl := cmd.Flags()
	fl.BoolVarP(&opts.showSystemLogs, "system-logs", "s", false, "Show system activities and logs.")
	fl.StringVarP(&opts.outputFormat, "output", "F", "text", "Format output as: text, json.")
	fl.IntVarP(&opts.pageNumber, "page", "p", 1, "Page number.")
	fl.IntVarP(&opts.perPage, "per-page", "P", 20, "Number of items to list per page.")

	return cmd
}

func NewCmdAdd(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		commandType:     commandTypeAdd,
		io:              f.IO(),
		gitlabClient:    f.GitLabClient,
		apiClient:       f.ApiClient,
		baseRepo:        f.BaseRepo,
		config:          f.Config,
		defaultHostname: f.DefaultHostname(),
	}

	cmd := &cobra.Command{
		Use:   "add [<id> | <branch>]",
		Short: "Add a comment to a merge request",
		Long:  "Add a new comment to the specified merge request\n",
		Args:  cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(args); err != nil {
				return err
			}

			if err := opts.validate(); err != nil {
				return err
			}

			return opts.run(f)
		},
	}

	cmd.Flags().StringVarP(&opts.message, "message", "m", "", "Comment message.")
	cmd.Flags().BoolVar(&opts.unique, "unique", false, "Don't create a comment or note if it already exists.")

	return cmd
}

func NewCmdReply(f cmdutils.Factory) *cobra.Command {
	opts := &options{
		commandType:     commandTypeReply,
		io:              f.IO(),
		gitlabClient:    f.GitLabClient,
		apiClient:       f.ApiClient,
		baseRepo:        f.BaseRepo,
		config:          f.Config,
		defaultHostname: f.DefaultHostname(),
	}

	cmd := &cobra.Command{
		Use:   "reply <id> <comment-id>",
		Short: "Reply to a comment on a merge request",
		Long:  "Reply to a specific comment on the specified merge request\n",
		Args:  cobra.ExactArgs(2),
		Annotations: map[string]string{
			mcpannotations.Destructive: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(args); err != nil {
				return err
			}

			if err := opts.validate(); err != nil {
				return err
			}

			return opts.run(f)
		},
	}

	cmd.Flags().StringVarP(&opts.message, "message", "m", "", "Reply message.")

	return cmd
}

func (o *options) complete(args []string) error {
	if len(args) > 0 {
		o.mrID = args[0]
	}

	if o.commandType == commandTypeReply && len(args) > 1 {
		commentID, err := strconv.Atoi(args[1])
		if err != nil {
			return fmt.Errorf("invalid comment ID: %s", args[1])
		}
		o.commentID = commentID
	}

	return nil
}

func (o *options) validate() error {
	if o.commandType == commandTypeReply {
		if o.commentID <= 0 {
			return fmt.Errorf("invalid comment ID: %d", o.commentID)
		}
	}

	if o.commandType == commandTypeList {
		if o.outputFormat != "text" && o.outputFormat != "json" {
			return fmt.Errorf("invalid output format: %s. Must be 'text' or 'json'", o.outputFormat)
		}
	}

	return nil
}

func (o *options) run(f cmdutils.Factory) error {
	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	mrArgs := []string{}
	if o.mrID != "" {
		mrArgs = []string{o.mrID}
	}
	mr, baseRepo, err := mrutils.MRFromArgsWithOpts(f, mrArgs, &gitlab.GetMergeRequestsOptions{}, "any")
	if err != nil {
		return err
	}

	switch o.commandType {
	case commandTypeList:
		return o.runList(client, mr, baseRepo)
	case commandTypeAdd:
		return o.runAdd(client, mr, baseRepo)
	case commandTypeReply:
		return o.runReply(client, mr, baseRepo)
	default:
		return fmt.Errorf("unknown command type: %s", o.commandType)
	}
}

func (o *options) runList(client *gitlab.Client, mr *gitlab.MergeRequest, baseRepo glrepo.Interface) error {
	l := &gitlab.ListMergeRequestNotesOptions{
		Sort: gitlab.Ptr("asc"),
		ListOptions: gitlab.ListOptions{
			Page:    o.pageNumber,
			PerPage: o.perPage,
		},
	}
	notes, err := listMRNotes(client, baseRepo.FullName(), mr.IID, l)
	if err != nil {
		return err
	}

	if o.outputFormat == "json" {
		return o.printJSONComments(notes)
	}

	return o.printComments(notes, "merge request", o.showSystemLogs)
}

func (o *options) runAdd(client *gitlab.Client, mr *gitlab.MergeRequest, baseRepo glrepo.Interface) error {
	message := o.message
	if strings.TrimSpace(message) == "" {
		editor, err := cmdutils.GetEditor(o.config)
		if err != nil {
			return err
		}

		message = utils.Editor(utils.EditorOptions{
			Label:         "Comment message:",
			Help:          "Enter the comment message for the merge request.",
			FileName:      "*_MR_COMMENT_EDITMSG.md",
			EditorCommand: editor,
		})
	}

	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("aborted... Comment has an empty message")
	}

	if o.unique {
		opts := &gitlab.ListMergeRequestNotesOptions{ListOptions: gitlab.ListOptions{PerPage: api.DefaultListLimit}}
		notes, _, err := client.Notes.ListMergeRequestNotes(baseRepo.FullName(), mr.IID, opts)
		if err != nil {
			return fmt.Errorf("running merge request note deduplication: %v", err)
		}
		for _, noteInfo := range notes {
			if noteInfo.Body == message {
				fmt.Fprintf(o.io.StdOut, "%s#note_%d\n", mr.WebURL, noteInfo.ID)
				return nil
			}
		}
	}

	note, _, err := client.Notes.CreateMergeRequestNote(baseRepo.FullName(), mr.IID, &gitlab.CreateMergeRequestNoteOptions{Body: &message})
	if err != nil {
		return err
	}

	fmt.Fprintf(o.io.StdOut, "%s#note_%d\n", mr.WebURL, note.ID)
	return nil
}

func (o *options) runReply(client *gitlab.Client, mr *gitlab.MergeRequest, baseRepo glrepo.Interface) error {
	message := o.message
	if strings.TrimSpace(message) == "" {
		editor, err := cmdutils.GetEditor(o.config)
		if err != nil {
			return err
		}

		message = utils.Editor(utils.EditorOptions{
			Label:         "Reply message:",
			Help:          fmt.Sprintf("Enter the reply message for comment #%d on the merge request.", o.commentID),
			FileName:      "*_MR_REPLY_EDITMSG.md",
			EditorCommand: editor,
		})
	}

	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("aborted... Reply has an empty message")
	}

	// For replies, we just add a new comment with a reference to the original
	// GitLab doesn't have a direct "reply" API, so we mention the original comment
	replyMessage := fmt.Sprintf("@%s (replying to comment #%d)\n\n%s", mr.Author.Username, o.commentID, message)

	note, _, err := client.Notes.CreateMergeRequestNote(baseRepo.FullName(), mr.IID, &gitlab.CreateMergeRequestNoteOptions{Body: &replyMessage})
	if err != nil {
		return err
	}

	fmt.Fprintf(o.io.StdOut, "%s#note_%d\n", mr.WebURL, note.ID)
	return nil
}

func (o *options) printComments(notes []*gitlab.Note, issueType string, showSystemLogs bool) error {
	c := o.io.Color()

	fmt.Fprintln(o.io.StdOut, heredoc.Doc(`
		--------------------------------------------
		Comments / Notes
		--------------------------------------------
		`))

	if len(notes) > 0 {
		for i, note := range notes {
			if note.System && !showSystemLogs {
				continue
			}
			createdAt := utils.TimeToPrettyTimeAgo(*note.CreatedAt)
			fmt.Fprintf(o.io.StdOut, "[%d] ", i+1)
			fmt.Fprint(o.io.StdOut, note.Author.Username)
			if note.System {
				fmt.Fprintf(o.io.StdOut, " %s ", note.Body)
				fmt.Fprintln(o.io.StdOut, c.Gray(createdAt))
			} else {
				body, _ := utils.RenderMarkdown(note.Body, o.io.BackgroundColor())
				fmt.Fprint(o.io.StdOut, " commented ")
				fmt.Fprintf(o.io.StdOut, c.Gray("%s\n"), createdAt)

				// Display file and line context if available
				if note.Position != nil {
					printCommentFileContext(o.io.StdOut, c, note.Position)
				}

				fmt.Fprintln(o.io.StdOut, utils.Indent(body, " "))
			}
			fmt.Fprintln(o.io.StdOut)
		}
	} else {
		fmt.Fprintf(o.io.StdOut, "This %s has no comments.\n", issueType)
	}

	return nil
}

func (o *options) printJSONComments(notes []*gitlab.Note) error {
	notesJSON, err := json.Marshal(notes)
	if err != nil {
		return err
	}
	fmt.Fprintln(o.io.StdOut, string(notesJSON))
	return nil
}

func printCommentFileContext(out io.Writer, c *iostreams.ColorPalette, position *gitlab.NotePosition) {
	if position == nil {
		return
	}

	fileInfo := fmt.Sprintf("File: %s", position.NewPath)
	if position.OldPath != "" && position.OldPath != position.NewPath {
		fileInfo = fmt.Sprintf("File: %s → %s", position.OldPath, position.NewPath)
	}

	if position.NewLine != 0 {
		fileInfo += fmt.Sprintf(" (line %d)", position.NewLine)
	}

	fmt.Fprintf(out, c.Gray("  %s\n"), fileInfo)
}
