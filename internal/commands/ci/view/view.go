package view

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/ci/ciutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/git"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/utils"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/gdamore/tcell/v2"
	"github.com/lunixbochs/vtclean"
	"github.com/pkg/errors"
	"github.com/rivo/tview"
	"github.com/spf13/cobra"
	gitlab "gitlab.com/gitlab-org/api/client-go"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type options struct {
	io           *iostreams.IOStreams
	gitlabClient func() (*gitlab.Client, error)
	baseRepo     func() (glrepo.Interface, error)
	config       func() config.Config

	refName       string
	openInBrowser bool
}

type ViewJobKind int64

const (
	Job ViewJobKind = iota
	Bridge
)

type ViewJob struct {
	ID           int        `json:"id"`
	Name         string     `json:"name"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	ErasedAt     *time.Time `json:"erased_at"`
	Duration     float64    `json:"duration"`
	Stage        string     `json:"stage"`
	Status       string     `json:"status"`
	AllowFailure bool       `json:"allow_failure"`

	Kind ViewJobKind

	OriginalJob    *gitlab.Job
	OriginalBridge *gitlab.Bridge
}

type SearchState struct {
	Active          bool
	Query           string
	Matches         []SearchMatch
	CurrentMatch    int
	LastScrollPos   int
	InputMode       bool
	OriginalContent string // Store the original log content with formatting before search highlighting
}

// LogState tracks the loading state of job logs
type LogState struct {
	Loading   bool // Whether logs are currently being fetched/streamed
	Completed bool // Whether log streaming has completed
}

type SearchMatch struct {
	Line  int
	Start int
	End   int
}

func ViewJobFromBridge(bridge *gitlab.Bridge) *ViewJob {
	vj := &ViewJob{}
	vj.ID = bridge.ID
	vj.Name = bridge.Name
	vj.Status = bridge.Status
	vj.Stage = bridge.Stage
	vj.StartedAt = bridge.StartedAt
	vj.FinishedAt = bridge.FinishedAt
	vj.ErasedAt = bridge.ErasedAt
	vj.Duration = bridge.Duration
	vj.AllowFailure = bridge.AllowFailure
	vj.OriginalBridge = bridge
	vj.Kind = Bridge
	return vj
}

func ViewJobFromJob(job *gitlab.Job) *ViewJob {
	vj := &ViewJob{}
	vj.ID = job.ID
	vj.Name = job.Name
	vj.Status = job.Status
	vj.Stage = job.Stage
	vj.StartedAt = job.StartedAt
	vj.FinishedAt = job.FinishedAt
	vj.ErasedAt = job.ErasedAt
	vj.Duration = job.Duration
	vj.AllowFailure = job.AllowFailure
	vj.OriginalJob = job
	vj.Kind = Job
	return vj
}

// getSearchState returns the search state for a given job name, creating a new one if needed
func getSearchState(jobName string) *SearchState {
	if searchStates == nil {
		searchStates = make(map[string]*SearchState)
	}

	if state, exists := searchStates[jobName]; exists {
		return state
	}

	// Create new search state with default values
	state := &SearchState{
		Active:        false,
		Query:         "",
		Matches:       []SearchMatch{},
		CurrentMatch:  -1,
		LastScrollPos: 0,
		InputMode:     false,
	}

	searchStates[jobName] = state
	return state
}

// clearSearchState removes the search state for a given job name
func clearSearchState(jobName string) {
	if searchStates != nil {
		delete(searchStates, jobName)
	}
}

// getLogState returns the log state for a given job name, creating a new one if needed
func getLogState(jobName string) *LogState {
	if logStates == nil {
		logStates = make(map[string]*LogState)
	}

	if state, exists := logStates[jobName]; exists {
		return state
	}

	// Create new log state with default values
	state := &LogState{
		Loading:   false,
		Completed: false,
	}

	logStates[jobName] = state
	return state
}

// setLogLoading marks a job's logs as currently loading
func setLogLoading(jobName string, loading bool) {
	state := getLogState(jobName)
	state.Loading = loading
}

// setLogCompleted marks a job's logs as completed loading
func setLogCompleted(jobName string, completed bool) {
	state := getLogState(jobName)
	state.Completed = completed
	if completed {
		state.Loading = false // Log is completed, so it's no longer loading
	}
}

// isLogCompleted returns true if the job's logs have finished loading
func isLogCompleted(jobName string) bool {
	state := getLogState(jobName)
	return state.Completed
}

// canActivateSearch checks if search can be activated (needs loaded content)
func (s *SearchState) canActivateSearch(content string) bool {
	return content != ""
}

// activateSearch enters search mode
func (s *SearchState) activateSearch() {
	s.Active = true
	s.InputMode = true
	s.Query = "/"
}

// deactivateSearch exits search mode
func (s *SearchState) deactivateSearch() {
	s.Active = false
	s.InputMode = false
	s.Query = ""
	s.Matches = []SearchMatch{}
	s.CurrentMatch = -1
}

// updateQuery updates the search query
func (s *SearchState) updateQuery(query string) {
	s.Query = query
}

// handleBackspace handles backspace key presses in search mode
func (s *SearchState) handleBackspace(key tcell.Key) bool {
	if !s.InputMode || len(s.Query) == 0 {
		return false
	}

	if key == tcell.KeyBackspace || key == tcell.KeyBackspace2 {
		if len(s.Query) > 1 {
			s.Query = s.Query[:len(s.Query)-1]
		} else {
			// Exit search mode when deleting the last character (/)
			s.deactivateSearch()
		}
		return true
	}
	return false
}

// performSearch searches for matches in the given content
func (s *SearchState) performSearch(content, query string) []SearchMatch {
	if query == "" {
		s.Matches = []SearchMatch{}
		return s.Matches
	}

	var matches []SearchMatch
	lines := strings.Split(content, "\n")
	lowerQuery := strings.ToLower(query)

	for lineNum, line := range lines {
		lowerLine := strings.ToLower(line)
		searchStart := 0

		for {
			// Find next occurrence of query in the line (case-insensitive)
			idx := strings.Index(lowerLine[searchStart:], lowerQuery)
			if idx == -1 {
				break
			}

			// Calculate actual position in original line
			actualStart := searchStart + idx
			actualEnd := actualStart + len(query)

			matches = append(matches, SearchMatch{
				Line:  lineNum,
				Start: actualStart,
				End:   actualEnd,
			})

			// Move search position past this match
			searchStart = actualEnd
		}
	}

	// Update state with the matches
	s.Matches = matches
	return matches
}

// getMatchCount returns the number of search matches
func (s *SearchState) getMatchCount() int {
	return len(s.Matches)
}

// shouldActivateSearch determines if search can be activated based on current state
func shouldActivateSearch(logsVisible, modalVisible bool, logContent string, jobName string) bool {
	if !logsVisible || modalVisible || logContent == "" {
		return false
	}

	// Only allow search activation if logs have finished loading
	return isLogCompleted(jobName)
}

// handleSearchKeyInput processes key input when search is active
func handleSearchKeyInput(state *SearchState, key tcell.Key, char rune) bool {
	if !state.Active || !state.InputMode {
		return false
	}

	// Handle backspace keys (both KeyBackspace and KeyBackspace2 for cross-platform support)
	if key == tcell.KeyBackspace || key == tcell.KeyBackspace2 {
		return state.handleBackspace(key)
	}

	// Handle regular character input
	if char != 0 && char != '\n' && char != '\r' {
		state.Query += string(char)
		return true
	}

	return false
}

// handleSearchEscape processes escape key when search might be active
func handleSearchEscape(state *SearchState, jobName string) bool {
	if !state.Active {
		return false // Let normal escape handling take over
	}

	// Clear highlighting before deactivating search
	clearSearchHighlighting(jobName)
	state.deactivateSearch()
	return true // Consumed the escape key
}

// handleSearchEnter processes enter key when search might be active
func handleSearchEnter(state *SearchState, logContent string, jobName string) bool {
	if !state.Active {
		return false // Let normal enter handling take over
	}

	if state.InputMode {
		// Submit search query - switch from input mode to navigation mode
		query := state.Query[1:] // Remove the leading "/"

		// If query is empty (just pressed "/" then Enter), don't perform search
		// but still exit search mode and stay in log view
		if strings.TrimSpace(query) == "" {
			state.deactivateSearch()
			return true // Consume the key so it doesn't close the log
		}

		state.performSearch(logContent, query)
		state.InputMode = false
		if len(state.Matches) > 0 {
			state.CurrentMatch = 0 // Start at first match
		}

		// Apply highlighting to the log content
		applySearchHighlighting(jobName, query)
		return true // Consumed the enter key
	} else {
		// Navigate to next match
		if len(state.Matches) > 0 {
			state.CurrentMatch = (state.CurrentMatch + 1) % len(state.Matches)
		}
		return true // Consumed the enter key
	}
}

// handleSearchSlash processes "/" key for search activation
func handleSearchSlash(state *SearchState, logsVisible, modalVisible bool, logContent string, jobName string) bool {
	if !shouldActivateSearch(logsVisible, modalVisible, logContent, jobName) {
		return false // Don't consume the key
	}

	state.activateSearch()
	return true // Consumed the "/" key
}

// updateSearchDisplay updates the search bar display for a specific job
func updateSearchDisplay(jobName string, app *tview.Application) {
	if logFrames == nil {
		return
	}

	logsKey := "logs-" + jobName
	frame, exists := logFrames[logsKey]
	if !exists {
		return
	}

	searchState := getSearchState(jobName)

	// Clear previous footer text
	frame.Clear()

	if !searchState.Active {
		// Keep footer space allocated but empty
		frame.AddText(" ", false, tview.AlignLeft, tcell.ColorDefault)
		return
	}

	if searchState.InputMode {
		// Show search input with cursor indicator
		frame.AddText(searchState.Query+"█", false, tview.AlignLeft, tcell.ColorYellow)
	} else {
		// Show search results navigation
		if len(searchState.Matches) > 0 {
			text := fmt.Sprintf("%s [%d/%d matches]",
				searchState.Query[1:], // Remove leading /
				searchState.CurrentMatch+1,
				len(searchState.Matches))
			frame.AddText(text, false, tview.AlignLeft, tcell.ColorGreen)
		} else {
			noMatchText := searchState.Query + " [no matches]"
			frame.AddText(noMatchText, false, tview.AlignLeft, tcell.ColorRed)
		}
	}

	if app != nil {
		app.ForceDraw()
	}
}

// highlightMatches adds tview markup to highlight search matches in log text
func highlightMatches(logContent, searchQuery string) string {
	if searchQuery == "" || logContent == "" {
		return logContent
	}

	// Convert to lowercase for case-insensitive matching
	lowerQuery := strings.ToLower(searchQuery)
	lowerContent := strings.ToLower(logContent)

	// Find all matches and build list of ranges to highlight
	var highlights []struct {
		start, end int
	}

	startPos := 0
	for {
		pos := strings.Index(lowerContent[startPos:], lowerQuery)
		if pos == -1 {
			break
		}

		actualPos := startPos + pos
		highlights = append(highlights, struct{ start, end int }{
			start: actualPos,
			end:   actualPos + len(searchQuery),
		})
		startPos = actualPos + len(searchQuery)
	}

	// If no matches found, return original content
	if len(highlights) == 0 {
		return logContent
	}

	// Build result string with highlighting markup
	var result strings.Builder
	lastEnd := 0

	for _, highlight := range highlights {
		// Add text before highlight
		if highlight.start > lastEnd {
			result.WriteString(logContent[lastEnd:highlight.start])
		}

		// Add highlighted text with tview markup
		matchedText := logContent[highlight.start:highlight.end]
		result.WriteString("[red::]")
		result.WriteString(matchedText)
		result.WriteString("[white::-]")

		lastEnd = highlight.end
	}

	// Add remaining text after last highlight
	if lastEnd < len(logContent) {
		result.WriteString(logContent[lastEnd:])
	}

	return result.String()
}

// applySearchHighlighting applies search highlighting to the log TextView for a specific job
func applySearchHighlighting(jobName, searchQuery string) {
	if logViews == nil {
		return
	}

	logsKey := "logs-" + jobName
	tv, exists := logViews[logsKey]
	if !exists {
		return
	}

	searchState := getSearchState(jobName)

	// Use the original content that was captured when logs completed
	// This prevents any accumulation of newlines or highlighting artifacts
	if searchState.OriginalContent == "" {
		// Fallback: if for some reason original content wasn't captured, use current content
		searchState.OriginalContent = tv.GetText(false)
	}

	// Always apply highlighting to the stored original content
	highlightedContent := highlightMatches(searchState.OriginalContent, searchQuery)

	// Strip any trailing newline to prevent accumulation when TextView adds its own
	highlightedContent = strings.TrimSuffix(highlightedContent, "\n")

	// Update the TextView with highlighted content
	tv.SetText(highlightedContent)
}

// clearSearchHighlighting removes search highlighting from the log TextView
func clearSearchHighlighting(jobName string) {
	if logViews == nil {
		return
	}

	logsKey := "logs-" + jobName
	tv, exists := logViews[logsKey]
	if !exists {
		return
	}

	// Restore the original content with its original formatting
	searchState := getSearchState(jobName)
	if searchState.OriginalContent != "" {
		// Strip any trailing newline to prevent accumulation when TextView adds its own
		originalContent := strings.TrimSuffix(searchState.OriginalContent, "\n")
		tv.SetText(originalContent)
		// Clear the stored original content since we're exiting search mode
		searchState.OriginalContent = ""
	}
}

func NewCmdView(f cmdutils.Factory) *cobra.Command {
	opts := options{
		io:           f.IO(),
		gitlabClient: f.GitLabClient,
		baseRepo:     f.BaseRepo,
		config:       f.Config,
	}
	pipelineCIView := &cobra.Command{
		Use:   "view [branch/tag]",
		Short: "View, run, trace, log, and cancel CI/CD job's current pipeline.",
		Long: heredoc.Doc(`Supports viewing, running, tracing, and canceling jobs.

		Use arrow keys to navigate jobs and logs.

		- 'Enter' to toggle through a job's logs / traces, or display a child pipeline. Trigger jobs are marked with a '»'.
		- 'Esc' or 'q' to close the logs or trace, or return to the parent pipeline.
		- 'Ctrl+R', 'Ctrl+P' to run, retry, or play a job. Use 'Tab' or arrow keys to navigate the modal, and 'Enter' to confirm.
		- 'Ctrl+D' to cancel a job. If the selected job isn't running or pending, quits the CI/CD view.
		- 'Ctrl+Q' to quit the CI/CD view.
		- 'Ctrl+Space' to suspend application and view the logs. Similar to 'glab pipeline ci trace'.
		Supports vi style bindings and arrow keys for navigating jobs and logs.
	`),
		Example: heredoc.Doc(`
			# Uses current branch
			$ glab pipeline ci view

			# Get latest pipeline on main branch
			$ glab pipeline ci view main

			# just like the second example
			$ glab pipeline ci view -b main

			# Get latest pipeline on main branch of profclems/glab repo
			$ glab pipeline ci view -b main -R profclems/glab
		`),
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.complete(args); err != nil {
				return err
			}

			return opts.run()
		},
	}

	pipelineCIView.Flags().
		StringVarP(&opts.refName, "branch", "b", "", "Check pipeline status for a branch or tag. Defaults to the current branch.")
	pipelineCIView.Flags().BoolVarP(&opts.openInBrowser, "web", "w", false, "Open pipeline in a browser. Uses default browser, or browser specified in BROWSER variable.")

	return pipelineCIView
}

func (o *options) complete(args []string) error {
	if o.refName == "" {
		if len(args) == 1 {
			o.refName = args[0]
		} else {
			refName, err := git.CurrentBranch()
			if err != nil {
				return err
			}
			o.refName = refName
		}
	}

	return nil
}

func (o *options) run() error {
	client, err := o.gitlabClient()
	if err != nil {
		return err
	}

	repo, err := o.baseRepo()
	if err != nil {
		return err
	}

	projectID := repo.FullName()

	commit, _, err := client.Commits.GetCommit(projectID, o.refName, nil)
	if err != nil {
		return err
	}

	commitSHA := commit.ID
	if commit.LastPipeline == nil {
		return fmt.Errorf("Can't find pipeline for commit: %s", commitSHA)
	}

	cfg := o.config()

	if o.openInBrowser { // open in browser if --web flag is specified
		webURL := commit.LastPipeline.WebURL

		if o.io.IsOutputTTY() {
			fmt.Fprintf(o.io.StdErr, "Opening %s in your browser.\n", utils.DisplayURL(webURL))
		}

		browser, _ := cfg.Get(repo.RepoHost(), "browser")
		return utils.OpenInBrowser(webURL, browser)
	}

	p, _, err := client.Pipelines.GetPipeline(projectID, commit.LastPipeline.ID)
	if err != nil {
		return fmt.Errorf("Can't get pipeline #%d info: %s", commit.LastPipeline.ID, err)
	}
	pipelineUser := p.User

	pipelines = make([]gitlab.PipelineInfo, 0, 10)

	root := tview.NewPages()
	root.
		SetBackgroundColor(tcell.ColorDefault).
		SetBorderPadding(1, 1, 2, 2).
		SetBorder(true).
		SetTitle(fmt.Sprintf(" Pipeline #%d triggered %s by %s ", commit.LastPipeline.ID, utils.TimeToPrettyTimeAgo(*commit.LastPipeline.CreatedAt), pipelineUser.Name))

	boxes = make(map[string]*tview.TextView)
	jobsCh := make(chan []*ViewJob)
	forceUpdateCh := make(chan bool)
	inputCh := make(chan struct{})

	screen, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	app := tview.NewApplication()
	defer recoverPanic(app)

	var navi navigator
	app.SetInputCapture(inputCapture(app, root, navi, inputCh, forceUpdateCh, o, client, projectID, commitSHA))
	go updateJobs(app, jobsCh, forceUpdateCh, client, commit)
	go func() {
		defer recoverPanic(app)
		for {
			app.SetFocus(root)
			jobsView(app, jobsCh, inputCh, root, client, projectID, commitSHA)
			app.Draw()
		}
	}()
	if err := app.SetScreen(screen).SetRoot(root, true).SetAfterDrawFunc(linkJobsView(app)).Run(); err != nil {
		return err
	}
	return nil
}

func inputCapture(
	app *tview.Application,
	root *tview.Pages,
	navi navigator,
	inputCh chan struct{},
	forceUpdateCh chan bool,
	opts *options,
	apiClient *gitlab.Client,
	projectID string,
	commitSHA string,
) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		// Never consume critical system keys - always let them pass through
		if event.Key() == tcell.KeyCtrlC {
			return event // Always pass through Ctrl+C for force quit
		}

		// Handle search functionality when logs are visible
		if logsVisible && curJob != nil {
			searchState := getSearchState(curJob.Name)

			// Get log content for search operations
			var logContent string
			logsKey := "logs-" + curJob.Name
			if logViews != nil {
				if tv, exists := logViews[logsKey]; exists {
					logContent = tv.GetText(false) // false = don't strip formatting
				}
			}

			// Handle slash key for search activation
			if event.Rune() == '/' {
				if handleSearchSlash(searchState, logsVisible, modalVisible, logContent, curJob.Name) {
					updateSearchDisplay(curJob.Name, app)
					return nil // Consumed the key
				}
			}

			// Handle escape key for search exit
			if event.Key() == tcell.KeyEscape {
				if handleSearchEscape(searchState, curJob.Name) {
					updateSearchDisplay(curJob.Name, app)
					return nil // Consumed the key
				}
			}

			// Handle enter key for search submission/navigation
			if event.Key() == tcell.KeyEnter {
				if handleSearchEnter(searchState, logContent, curJob.Name) {
					updateSearchDisplay(curJob.Name, app)
					return nil // Consumed the key
				}
			}

			// Handle character and backspace input in search mode
			if searchState.Active && searchState.InputMode {
				if handleSearchKeyInput(searchState, event.Key(), event.Rune()) {
					updateSearchDisplay(curJob.Name, app)
					return nil // Consumed the key
				}
			}
		}

		if event.Rune() == 'q' || event.Key() == tcell.KeyEscape {
			switch {
			case modalVisible:
				modalVisible = !modalVisible
				root.HidePage("yesno")
				if inputCh == nil {
					inputCh <- struct{}{}
				}
			case logsVisible:
				logsVisible = !logsVisible
				root.HidePage("logs-" + curJob.Name)
				if inputCh == nil {
					inputCh <- struct{}{}
				}
				app.ForceDraw()
			case len(pipelines) > 0:
				pipelines = pipelines[:len(pipelines)-1]
				curJob = nil
				forceUpdateCh <- true
				app.ForceDraw()
			default:
				app.Stop()
				return nil
			}
		}
		if !modalVisible && !logsVisible && len(jobs) > 0 {
			curJob = navi.Navigate(jobs, event)
			root.SendToFront("jobs-" + curJob.Name)
			if inputCh == nil {
				inputCh <- struct{}{}
			}
		}
		switch event.Key() {
		case tcell.KeyCtrlQ:
			app.Stop()
			return nil
		case tcell.KeyCtrlD:
			if curJob.Kind == Job && (curJob.Status == "pending" || curJob.Status == "running") {
				modalVisible = true
				modal := tview.NewModal().
					SetBackgroundColor(tcell.ColorDefault).
					SetText(fmt.Sprintf("Are you sure you want to cancel %s?", curJob.Name)).
					AddButtons([]string{"✘ No", "✔ Yes"}).
					SetDoneFunc(func(buttonIndex int, buttonLabel string) {
						modalVisible = false
						root.RemovePage("yesno")
						if buttonLabel == "✘ No" {
							app.ForceDraw()
							return
						}
						root.RemovePage("logs-" + curJob.Name)
						app.ForceDraw()
						job, _, err := apiClient.Jobs.CancelJob(projectID, curJob.ID)
						if err != nil {
							app.Stop()
							log.Fatal(err)
						}
						if job != nil {
							curJob = ViewJobFromJob(job)
							app.ForceDraw()
						}
					})
				root.AddAndSwitchToPage("yesno", modal, false)
				inputCh <- struct{}{}
				app.ForceDraw()
				return nil
			}
		case tcell.KeyCtrlP, tcell.KeyCtrlR:
			if modalVisible || curJob.Kind != Job {
				break
			}
			modalVisible = true
			modal := tview.NewModal().
				SetBackgroundColor(tcell.ColorDefault).
				SetText(fmt.Sprintf("Are you sure you want to run %s?", curJob.Name)).
				AddButtons([]string{"✘ No", "✔ Yes"}).
				SetDoneFunc(func(buttonIndex int, buttonLabel string) {
					modalVisible = false
					root.RemovePage("yesno")
					if buttonLabel != "✔ Yes" {
						app.ForceDraw()
						return
					}
					root.RemovePage("logs-" + curJob.Name)
					app.ForceDraw()

					job, err := api.PlayOrRetryJobs(
						apiClient,
						projectID,
						curJob.ID,
						curJob.Status,
					)
					if err != nil {
						app.Stop()
						log.Fatal(err)
					}
					if job != nil {
						curJob = ViewJobFromJob(job)
						app.ForceDraw()
					}
				})
			root.AddAndSwitchToPage("yesno", modal, false)
			inputCh <- struct{}{}
			app.ForceDraw()
			return nil
		case tcell.KeyEnter:
			if !modalVisible {
				if curJob.Kind == Job {
					logsVisible = !logsVisible
					if !logsVisible {
						root.HidePage("logs-" + curJob.Name)
					}
					inputCh <- struct{}{}
					app.ForceDraw()
				} else {
					pipelines = append(pipelines, *curJob.OriginalBridge.DownstreamPipeline)
					curJob = nil
					forceUpdateCh <- true
					app.ForceDraw()
				}
				return nil
			}
		case tcell.KeyCtrlSpace:
			app.Suspend(func() {
				ctx, cancel := context.WithCancel(context.Background())
				go func() {
					err := ciutils.RunTraceSha(
						ctx,
						apiClient,
						opts.io.StdOut,
						projectID,
						commitSHA,
						curJob.Name,
					)
					if err != nil {
						app.Stop()
						log.Fatal(err)
					}
					if ctx.Err() == nil {
						fmt.Println("\nPress <Enter> to resume the ci GUI view.")
					}
				}()
				reader := bufio.NewReader(os.Stdin)
				for {
					r, _, err := reader.ReadRune()
					if err != io.EOF && err != nil {
						app.Stop()
						log.Fatal(err)
					}
					if r == '\n' {
						cancel()
						break
					}
				}
			})
			if inputCh == nil {
				inputCh <- struct{}{}
			}
			return nil
		}
		if inputCh == nil {
			inputCh <- struct{}{}
		}
		return event
	}
}

var (
	logsVisible, modalVisible bool
	curJob                    *ViewJob
	jobs                      []*ViewJob
	pipelines                 []gitlab.PipelineInfo
	boxes                     map[string]*tview.TextView
	logViews                  map[string]*tview.TextView
	logFrames                 map[string]*tview.Frame
	searchStates              map[string]*SearchState
	logStates                 map[string]*LogState // Track log loading state per job
)

func curPipeline(commit *gitlab.Commit) gitlab.PipelineInfo {
	if len(pipelines) == 0 {
		return *commit.LastPipeline
	}
	return pipelines[len(pipelines)-1]
}

// navigator manages the internal state for processing tcell.EventKeys
type navigator struct {
	depth, idx int
}

// Navigate uses the ci stages as boundaries and returns the currently focused
// job index after processing a *tcell.EventKey
func (n *navigator) Navigate(jobs []*ViewJob, event *tcell.EventKey) *ViewJob {
	stage := jobs[n.idx].Stage
	prev, next := adjacentStages(jobs, stage)
	switch event.Key() {
	case tcell.KeyLeft:
		stage = prev
	case tcell.KeyRight:
		stage = next
	}
	switch event.Rune() {
	case 'h':
		stage = prev
	case 'l':
		stage = next
	}
	l, u := stageBounds(jobs, stage)

	switch event.Key() {
	case tcell.KeyDown:
		n.depth++
		if n.depth > u-l {
			n.depth = u - l
		}
	case tcell.KeyUp:
		n.depth--
	}
	switch event.Rune() {
	case 'j':
		n.depth++
		if n.depth > u-l {
			n.depth = u - l
		}
	case 'k':
		n.depth--
	case 'g':
		n.depth = 0
	case 'G':
		n.depth = u - l
	}

	if n.depth < 0 {
		n.depth = 0
	}
	n.idx = min(l+n.depth, u)
	return jobs[n.idx]
}

func stageBounds(jobs []*ViewJob, s string) (int, int) {
	if len(jobs) <= 1 {
		return 0, 0
	}
	var l, u int
	p := jobs[0].Stage
	for i, v := range jobs {
		if v.Stage != s && u != 0 {
			return l, u
		}
		if v.Stage != p {
			l = i
			p = v.Stage
		}
		if v.Stage == s {
			u = i
		}
	}
	return l, u
}

func adjacentStages(jobs []*ViewJob, s string) (string, string) {
	if len(jobs) == 0 {
		return "", ""
	}
	p := jobs[0].Stage

	var n string
	for _, v := range jobs {
		if v.Stage != s && n != "" {
			n = v.Stage
			return p, n
		}
		if v.Stage == s {
			n = "cur"
		}
		if n == "" {
			p = v.Stage
		}
	}
	n = jobs[len(jobs)-1].Stage
	return p, n
}

func jobsView(
	app *tview.Application,
	jobsCh chan []*ViewJob,
	inputCh chan struct{},
	root *tview.Pages,
	apiClient *gitlab.Client,
	projectID string,
	commitSHA string,
) {
	select {
	case jobs = <-jobsCh:
	case <-inputCh:
	case <-time.NewTicker(time.Second * 1).C:
	}
	if jobs == nil {
		jobs = <-jobsCh
	}
	if curJob == nil && len(jobs) > 0 {
		curJob = jobs[0]
	}
	if modalVisible {
		return
	}
	if logsVisible {
		logsKey := "logs-" + curJob.Name
		if !root.SwitchToPage(logsKey).HasPage(logsKey) {
			tv := tview.NewTextView()
			tv.
				SetDynamicColors(true).
				SetBackgroundColor(tcell.ColorDefault).
				SetBorderPadding(0, 0, 1, 1).
				SetBorder(true)

			// Wrap TextView in Frame for search bar support
			frame := tview.NewFrame(tv)
			frame.SetBackgroundColor(tcell.ColorDefault)
			// Remove Frame's internal borders/spacing - SetBorders(top, bottom, header, footer, left, right)
			frame.SetBorders(0, 0, 0, 1, 0, 0)
			// Pre-allocate footer space to prevent layout shift
			frame.AddText(" ", false, tview.AlignLeft, tcell.ColorDefault)

			// Store both TextView and Frame for search functionality
			if logViews == nil {
				logViews = make(map[string]*tview.TextView)
			}
			if logFrames == nil {
				logFrames = make(map[string]*tview.Frame)
			}
			logViews[logsKey] = tv
			logFrames[logsKey] = frame

			// Mark logs as loading when we start fetching
			setLogLoading(curJob.Name, true)
			setLogCompleted(curJob.Name, false)

			go func() {
				defer func() {
					// Mark logs as completed when done (whether successful or error)
					setLogCompleted(curJob.Name, true)

					// Capture the final log content immediately when streaming completes
					// This ensures we get the clean, final content before any highlighting
					searchState := getSearchState(curJob.Name)
					if searchState.OriginalContent == "" {
						originalContent := tv.GetText(false)
						// Strip any trailing newline to prevent accumulation issues
						searchState.OriginalContent = strings.TrimSuffix(originalContent, "\n")
					}
				}()

				err := ciutils.RunTraceSha(
					context.Background(),
					apiClient,
					vtclean.NewWriter(tview.ANSIWriter(tv), true),
					projectID,
					commitSHA,
					curJob.Name,
				)
				if err != nil {
					app.Stop()
					log.Fatal(err)
				}
			}()
			root.AddAndSwitchToPage("logs-"+curJob.Name, frame, true)
		}
		return
	}
	px, _, maxX, maxY := root.GetInnerRect()
	var (
		stages    = 0
		lastStage = ""
	)
	// get the number of stages
	for _, j := range jobs {
		if j.Stage != lastStage {
			lastStage = j.Stage
			stages++
		}
	}
	lastStage = ""
	var (
		rowIdx   int
		stageIdx int
		maxTitle = 20
	)
	boxKeys := make(map[string]bool)
	for _, j := range jobs {
		boxX := px + (maxX / stages * stageIdx)
		if j.Stage != lastStage {
			stageIdx++
			lastStage = j.Stage
			key := "stage-" + j.Stage
			boxKeys[key] = true

			x, y, w, h := boxX, maxY/6-4, maxTitle+2, 3
			b := box(root, key, x, y, w, h)

			caser := cases.Title(language.English)
			b.SetText(caser.String(j.Stage))
			b.SetTextAlign(tview.AlignCenter)
		}
	}
	lastStage = jobs[0].Stage
	rowIdx = 0
	stageIdx = 0
	for _, j := range jobs {
		if j.Stage != lastStage {
			rowIdx = 0
			lastStage = j.Stage
			stageIdx++
		}
		boxX := px + (maxX / stages * stageIdx)

		key := "jobs-" + j.Name
		boxKeys[key] = true
		x, y, w, h := boxX, maxY/6+(rowIdx*5), maxTitle+2, 4
		b := box(root, key, x, y, w, h)
		b.SetTitle(j.Name)
		// The scope of jobs to show, one or array of: created, pending, running,
		// failed, success, canceled, skipped; showing all jobs if none provided
		var statChar rune
		switch j.Status {
		case "success":
			b.SetBorderColor(tcell.ColorGreen)
			statChar = '✔'
		case "failed":
			if j.AllowFailure {
				b.SetBorderColor(tcell.ColorOrange)
				statChar = '!'
			} else {
				b.SetBorderColor(tcell.ColorRed)
				statChar = '✘'
			}
		case "running":
			b.SetBorderColor(tcell.ColorBlue)
			statChar = '●'
		case "pending":
			b.SetBorderColor(tcell.ColorYellow)
			statChar = '●'
		case "manual":
			b.SetBorderColor(tcell.ColorGrey)
			statChar = '■'
		case "canceled":
			statChar = 'Ø'
		case "skipped":
			statChar = '»'
		}
		// retryChar := '⟳'
		title := fmt.Sprintf("%c %s", statChar, j.Name)
		// trim the suffix if it matches the stage, I've seen
		// the pattern in 2 different places to handle
		// different stages for the same service and it tends
		// to make the title spill over the max
		title = strings.TrimSuffix(title, ":"+j.Stage)
		b.SetTitle(title)
		// tview default aligns center, which is nice, but if
		// the title is too long we want to bias towards seeing
		// the beginning of it
		if tview.TaggedStringWidth(title) > maxTitle {
			b.SetTitleAlign(tview.AlignLeft)
		}
		triggerText := ""
		if j.Kind == Bridge {
			triggerText = "»"
		}
		if j.StartedAt != nil {
			end := time.Now()
			if j.FinishedAt != nil {
				end = *j.FinishedAt
			}
			b.SetText(triggerText + "\n" + utils.FmtDuration(end.Sub(*j.StartedAt)))
			b.SetTextAlign(tview.AlignRight)
		} else {
			b.SetText(triggerText)
		}
		b.SetTextAlign(tview.AlignRight)
		rowIdx++

	}
	for k := range boxes {
		if !boxKeys[k] {
			root.RemovePage(k)
		}
	}
	root.SendToFront("jobs-" + curJob.Name)
}

func box(root *tview.Pages, key string, x, y, w, h int) *tview.TextView {
	b, ok := boxes[key]
	if !ok {
		b = tview.NewTextView()
		b.
			SetBackgroundColor(tcell.ColorDefault).
			SetBorder(true)
		boxes[key] = b
	}
	b.SetRect(x, y, w, h)

	root.AddPage(key, b, false, true)
	return b
}

func recoverPanic(app *tview.Application) {
	if r := recover(); r != nil {
		app.Stop()
		log.Fatalf("%s\n%s\n", r, string(debug.Stack()))
	}
}

func updateJobs(
	app *tview.Application,
	jobsCh chan []*ViewJob,
	forceUpdateCh chan bool,
	apiClient *gitlab.Client,
	commit *gitlab.Commit,
) {
	defer recoverPanic(app)
	for {
		if modalVisible {
			time.Sleep(time.Second * 1)
			continue
		}
		var jobs []*gitlab.Job
		var bridges []*gitlab.Bridge
		var err error
		pipeline := curPipeline(commit)
		jobs, bridges, err = api.PipelineJobsWithID(
			apiClient,
			pipeline.ProjectID,
			pipeline.ID,
		)
		if err != nil {
			app.Stop()
			log.Fatal(errors.Wrap(err, "failed to find CI jobs."))
		}
		if len(jobs) == 0 && len(bridges) == 0 {
			app.Stop()
			log.Fatal("No jobs found in the pipeline. Your '.gitlab-ci.yml' file might be invalid, or the pipeline triggered no jobs.")
		}
		viewJobs := make([]*ViewJob, 0, len(jobs)+len(bridges))
		for _, j := range jobs {
			viewJobs = append(viewJobs, ViewJobFromJob(j))
		}
		for _, b := range bridges {
			viewJobs = append(viewJobs, ViewJobFromBridge(b))
		}
		jobsCh <- latestJobs(viewJobs)
		select {
		case <-forceUpdateCh:
		case <-time.After(time.Second * 5):
		}

	}
}

func linkJobsView(app *tview.Application) func(screen tcell.Screen) {
	return func(screen tcell.Screen) {
		defer recoverPanic(app)
		err := linkJobs(screen, jobs, boxes)
		if err != nil {
			app.Stop()
			log.Fatal(err)
		}
	}
}

func linkJobs(screen tcell.Screen, jobs []*ViewJob, boxes map[string]*tview.TextView) error {
	if logsVisible || modalVisible {
		return nil
	}
	for i, j := range jobs {
		if _, ok := boxes["jobs-"+j.Name]; !ok {
			return errors.Errorf("jobs-%s not found at index: %d", jobs[i].Name, i)
		}
	}
	var padding int
	// find the amount of space between two jobs is adjacent stages
	for i, k := 0, 1; k < len(jobs); i, k = i+1, k+1 {
		if jobs[i].Stage == jobs[k].Stage {
			continue
		}
		x1, _, w, _ := boxes["jobs-"+jobs[i].Name].GetRect()
		x2, _, _, _ := boxes["jobs-"+jobs[k].Name].GetRect()
		stageWidth := x2 - x1 - w
		switch {
		case stageWidth <= 3:
			padding = 1
		case stageWidth <= 6:
			padding = 2
		case stageWidth > 6:
			padding = 3
		}
	}
	for i, k := 0, 1; k < len(jobs); i, k = i+1, k+1 {
		v1 := boxes["jobs-"+jobs[i].Name]
		v2 := boxes["jobs-"+jobs[k].Name]
		link(screen, v1.Box, v2.Box, padding,
			jobs[i].Stage == jobs[0].Stage,           // is first stage?
			jobs[i].Stage == jobs[len(jobs)-1].Stage) // is last stage?
	}
	return nil
}

func link(
	screen tcell.Screen,
	v1 *tview.Box,
	v2 *tview.Box,
	padding int,
	firstStage, lastStage bool,
) {
	x1, y1, w, h := v1.GetRect()
	x2, y2, _, _ := v2.GetRect()

	dx, dy := x2-x1, y2-y1

	p := padding

	// drawing stages
	if dx != 0 {
		hline(screen, x1+w, y2+h/2, dx-w)
		if dy != 0 {
			// dy != 0 means the last stage had multple jobs
			screen.SetContent(x1+w+p-1, y2+h/2, '╦', nil, tcell.StyleDefault)
		}
		return
	}

	// Drawing a job in the same stage
	// left of view
	if !firstStage {
		if r, _, _, _ := screen.GetContent(x2-p, y1+h/2); r == '╚' {
			screen.SetContent(x2-p, y1+h/2, '╠', nil, tcell.StyleDefault)
		} else {
			screen.SetContent(x2-p, y1+h/2, '╦', nil, tcell.StyleDefault)
		}

		for i := 1; i < p; i++ {
			screen.SetContent(x2-i, y2+h/2, '═', nil, tcell.StyleDefault)
		}
		screen.SetContent(x2-p, y2+h/2, '╚', nil, tcell.StyleDefault)

		vline(screen, x2-p, y1+h-1, dy-1)
	}
	// right of view
	if !lastStage {
		if r, _, _, _ := screen.GetContent(x2+w+p-1, y1+h/2); r == '┛' {
			screen.SetContent(x2+w+p-1, y1+h/2, '╣', nil, tcell.StyleDefault)
		}
		for i := range p - 1 {
			screen.SetContent(x2+w+i, y2+h/2, '═', nil, tcell.StyleDefault)
		}
		screen.SetContent(x2+w+p-1, y2+h/2, '╝', nil, tcell.StyleDefault)

		vline(screen, x2+w+p-1, y1+h-1, dy-1)
	}
}

func hline(screen tcell.Screen, x, y, l int) {
	for i := range l {
		screen.SetContent(x+i, y, '═', nil, tcell.StyleDefault)
	}
}

func vline(screen tcell.Screen, x, y, l int) {
	for i := range l {
		screen.SetContent(x, y+i, '║', nil, tcell.StyleDefault)
	}
}

// latestJobs returns a list of unique jobs favoring the last stage+name
// version of a job in the provided list
func latestJobs(jobs []*ViewJob) []*ViewJob {
	var (
		lastJob = make(map[string]*ViewJob, len(jobs))
		dupIdx  = -1
	)
	for i, j := range jobs {
		_, ok := lastJob[j.Stage+j.Name]
		if dupIdx == -1 && ok {
			dupIdx = i
		}
		// always want the latest job
		lastJob[j.Stage+j.Name] = j
	}
	if dupIdx == -1 {
		dupIdx = len(jobs)
	}
	// first duplicate marks where retries begin
	outJobs := make([]*ViewJob, dupIdx)
	for i := range outJobs {
		j := jobs[i]
		outJobs[i] = lastJob[j.Stage+j.Name]
	}

	return outJobs
}
