package comment

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"gitlab.com/gitlab-org/cli/internal/mcpannotations"

	"gitlab.com/gitlab-org/cli/internal/commands/issuable"
	"gitlab.com/gitlab-org/cli/internal/commands/issue/issueutils"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/utils"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/spf13/cobra"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

var listIssueNotes = func(client *gitlab.Client, projectID any, issueID int, opts *gitlab.ListIssueNotesOptions) ([]*gitlab.Note, error) {
	if opts.PerPage == 0 {
		opts.PerPage = api.DefaultListLimit
	}
	notes, _, err := client.Notes.ListIssueNotes(projectID, issueID, opts)
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
	issueType      issuable.IssueType
	message        string
	commentID      int
	issueID        string

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
		Short:   "Manage comments on issues",
		Long: "List, add, and reply to comments on issues.\n\n" +
			"Note: 'comment' is the preferred terminology. The 'note' alias is provided for backward compatibility.\n",
		Example: heredoc.Doc(`
			$ glab issue comment list 123
			$ glab issue comment add 123 -m "Great work!"
			$ glab issue comment reply 123 456 -m "Thanks for the feedback"
			$ glab issue note 123 -m "Quick comment"
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

	listCmd := NewCmdList(f)
	commentCmd.AddCommand(listCmd)
	addCmd := NewCmdAdd(f)
	commentCmd.AddCommand(addCmd)
	replyCmd := NewCmdReply(f)
	commentCmd.AddCommand(replyCmd)

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
		Short: "List comments on an issue",
		Long:  "List all comments and notes on the specified issue\n",
		Args:  cobra.ExactArgs(1),
		Annotations: map[string]string{
			mcpannotations.Safe: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(cmd, args); err != nil {
				return err
			}

			if err := opts.validate(); err != nil {
				return err
			}

			return opts.run()
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
		Use:   "add <id>",
		Short: "Add a comment to an issue",
		Long:  "Add a new comment to the specified issue\n",
		Args:  cobra.ExactArgs(1),
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

			return opts.run()
		},
	}

	cmd.Flags().StringVarP(&opts.message, "message", "m", "", "Comment message.")

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
		Short: "Reply to a comment on an issue",
		Long:  "Reply to a specific comment on the specified issue\n",
		Args:  cobra.ExactArgs(2),
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

			return opts.run()
		},
	}

	cmd.Flags().StringVarP(&opts.message, "message", "m", "", "Reply message.")

	return cmd
}

// determineIssueType determines the issue type from the command's parent context
func determineIssueType(cmd *cobra.Command) issuable.IssueType {
	// Walk up the command tree to find the parent command
	parent := cmd.Parent()
	for parent != nil {
		// Check if parent has issueType annotation
		if issueTypeStr, ok := parent.Annotations["issueType"]; ok {
			return issuable.IssueType(issueTypeStr)
		}
		// Check command path as fallback
		cmdPath := parent.CommandPath()
		if strings.Contains(cmdPath, "incident") {
			return issuable.TypeIncident
		}
		if strings.Contains(cmdPath, "issue") {
			return issuable.TypeIssue
		}
		parent = parent.Parent()
	}
	// Default to issue if we can't determine
	return issuable.TypeIssue
}

func (o *options) complete(cmd *cobra.Command, args []string) error {
	o.issueType = determineIssueType(cmd)
	o.issueID = args[0]

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

func (o *options) run() error {
	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	issue, baseRepo, err := issueutils.IssueFromArg(o.apiClient, client, o.baseRepo, o.defaultHostname, o.issueID)
	if err != nil {
		return err
	}

	switch o.commandType {
	case commandTypeList:
		return o.runList(client, issue, baseRepo)
	case commandTypeAdd:
		return o.runAdd(client, issue, baseRepo)
	case commandTypeReply:
		return o.runReply(client, issue, baseRepo)
	default:
		return fmt.Errorf("unknown command type: %s", o.commandType)
	}
}

func (o *options) runList(client *gitlab.Client, issue *gitlab.Issue, baseRepo glrepo.Interface) error {
	valid, msg := issuable.ValidateIncidentCmd(o.issueType, "list comments", issue)
	if !valid {
		fmt.Fprintln(o.io.StdErr, msg)
		return nil
	}

	var notes []*gitlab.Note
	// Both issues and incidents use the same issue notes API
	l := &gitlab.ListIssueNotesOptions{
		Sort: gitlab.Ptr("asc"),
		ListOptions: gitlab.ListOptions{
			Page:    o.pageNumber,
			PerPage: o.perPage,
		},
	}
	notes, err := listIssueNotes(client, baseRepo.FullName(), issue.IID, l)
	if err != nil {
		return err
	}

	if o.outputFormat == "json" {
		return o.printJSONComments(notes)
	}

	return o.printComments(notes, o.issueType, o.showSystemLogs)
}

func (o *options) runAdd(client *gitlab.Client, issue *gitlab.Issue, baseRepo glrepo.Interface) error {
	valid, msg := issuable.ValidateIncidentCmd(o.issueType, "add comment", issue)
	if !valid {
		fmt.Fprintln(o.io.StdErr, msg)
		return nil
	}

	message := o.message
	if strings.TrimSpace(message) == "" {
		editor, err := cmdutils.GetEditor(o.config)
		if err != nil {
			return err
		}

		message = utils.Editor(utils.EditorOptions{
			Label:         "Comment message:",
			Help:          fmt.Sprintf("Enter the comment message for the %s.", o.issueType),
			FileName:      fmt.Sprintf("*_%s_COMMENT_EDITMSG.md", strings.ToUpper(string(o.issueType))),
			EditorCommand: editor,
		})
	}

	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("aborted... Comment has an empty message")
	}

	var note *gitlab.Note
	// Both issues and incidents use the same issue notes API
	note, _, err := client.Notes.CreateIssueNote(baseRepo.FullName(), issue.IID, &gitlab.CreateIssueNoteOptions{Body: &message})
	if err != nil {
		return err
	}

	fmt.Fprintf(o.io.StdOut, "%s#note_%d\n", issue.WebURL, note.ID)
	return nil
}

func (o *options) runReply(client *gitlab.Client, issue *gitlab.Issue, baseRepo glrepo.Interface) error {
	valid, msg := issuable.ValidateIncidentCmd(o.issueType, "reply to comment", issue)
	if !valid {
		fmt.Fprintln(o.io.StdErr, msg)
		return nil
	}

	message := o.message
	if strings.TrimSpace(message) == "" {
		editor, err := cmdutils.GetEditor(o.config)
		if err != nil {
			return err
		}

		message = utils.Editor(utils.EditorOptions{
			Label:         "Reply message:",
			Help:          fmt.Sprintf("Enter the reply message for comment #%d on the %s.", o.commentID, o.issueType),
			FileName:      fmt.Sprintf("*_%s_REPLY_EDITMSG.md", strings.ToUpper(string(o.issueType))),
			EditorCommand: editor,
		})
	}

	if strings.TrimSpace(message) == "" {
		return fmt.Errorf("aborted... Reply has an empty message")
	}

	// For replies, we just add a new comment with a reference to the original
	// GitLab doesn't have a direct "reply" API, so we mention the original comment
	replyMessage := fmt.Sprintf("@%s (replying to comment #%d)\n\n%s", issue.Author.Username, o.commentID, message)

	var note *gitlab.Note
	// Both issues and incidents use the same issue notes API
	note, _, err := client.Notes.CreateIssueNote(baseRepo.FullName(), issue.IID, &gitlab.CreateIssueNoteOptions{Body: &replyMessage})
	if err != nil {
		return err
	}

	fmt.Fprintf(o.io.StdOut, "%s#note_%d\n", issue.WebURL, note.ID)
	return nil
}

func (o *options) printComments(notes []*gitlab.Note, issueType issuable.IssueType, showSystemLogs bool) error {
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
