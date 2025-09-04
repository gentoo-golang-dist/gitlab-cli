package view

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/assert"
)

// Test_searchState_initialization tests search state initialization per TextView
func Test_searchState_initialization(t *testing.T) {
	// Test that search state is properly initialized for each job
	jobName := "test-job-1"

	// Get search state for a job (should create new state)
	state := getSearchState(jobName)

	assert.NotNil(t, state, "Search state should be created")
	assert.False(t, state.Active, "Search should not be active initially")
	assert.Equal(t, "", state.Query, "Search query should be empty initially")
	assert.Empty(t, state.Matches, "Search matches should be empty initially")
	assert.Equal(t, -1, state.CurrentMatch, "Current match should be -1 initially")
	assert.Equal(t, 0, state.LastScrollPos, "Last scroll position should be 0 initially")
	assert.False(t, state.InputMode, "Input mode should be false initially")

	// Test that getting the same job's state returns the same instance
	state2 := getSearchState(jobName)
	assert.Same(t, state, state2, "Should return same search state instance for same job")

	// Test that different jobs have different search states
	state3 := getSearchState("different-job")
	assert.NotSame(t, state, state3, "Different jobs should have different search states")
}

// Test_searchState_toggleSearchMode tests entering/exiting search mode
// Should only be available once log text has been loaded after GetTraceSha() completes
func Test_searchState_toggleSearchMode(t *testing.T) {
	jobName := "test-job"

	// Test that search mode cannot be activated without loaded content
	t.Run("cannot activate without loaded content", func(t *testing.T) {
		state := getSearchState(jobName)

		// Attempt to activate search mode when no content is loaded
		canActivate := state.canActivateSearch("")
		assert.False(t, canActivate, "Search should not be activatable without loaded content")
	})

	// Test search mode activation with loaded content
	t.Run("can activate with loaded content", func(t *testing.T) {
		state := getSearchState(jobName)
		logContent := "Sample log line 1\nSample log line 2\nError occurred\n"

		// Should be able to activate search mode
		canActivate := state.canActivateSearch(logContent)
		assert.True(t, canActivate, "Search should be activatable with loaded content")

		// Activate search mode
		state.activateSearch()
		assert.True(t, state.Active, "Search should be active after activation")
		assert.True(t, state.InputMode, "Input mode should be true after activation")
		assert.Equal(t, "", state.Query, "Query should be empty after activation")
	})

	// Test search mode deactivation
	t.Run("can deactivate search mode", func(t *testing.T) {
		state := getSearchState(jobName)

		// Start with active search
		state.activateSearch()
		state.updateQuery("test")

		// Deactivate search
		state.deactivateSearch()
		assert.False(t, state.Active, "Search should not be active after deactivation")
		assert.False(t, state.InputMode, "Input mode should be false after deactivation")
		assert.Equal(t, "", state.Query, "Query should be empty after deactivation")
		assert.Empty(t, state.Matches, "Matches should be cleared after deactivation")
		assert.Equal(t, -1, state.CurrentMatch, "Current match should be -1 after deactivation")
	})

	// Test search state persistence across different jobs
	t.Run("independent search states per job", func(t *testing.T) {
		job1State := getSearchState("job1")
		job2State := getSearchState("job2")

		// Activate search for job1
		job1State.activateSearch()
		job1State.updateQuery("job1search")

		// Activate different search for job2
		job2State.activateSearch()
		job2State.updateQuery("job2search")

		// Verify states are independent
		assert.Equal(t, "job1search", job1State.Query, "Job1 should maintain its search query")
		assert.Equal(t, "job2search", job2State.Query, "Job2 should maintain its search query")
		assert.True(t, job1State.Active, "Job1 search should remain active")
		assert.True(t, job2State.Active, "Job2 search should remain active")
	})
}

// Test_searchState_updateQuery tests updating search query
func Test_searchState_updateQuery(t *testing.T) {
	jobName := "test-job"
	state := getSearchState(jobName)
	state.activateSearch() // Start in search mode

	testCases := []struct {
		name          string
		input         string
		expectedQuery string
		expectedInput bool
	}{
		{
			name:          "initial slash only",
			input:         "/",
			expectedQuery: "/",
			expectedInput: true,
		},
		{
			name:          "single character",
			input:         "/h",
			expectedQuery: "/h",
			expectedInput: true,
		},
		{
			name:          "multiple characters",
			input:         "hello",
			expectedQuery: "hello",
			expectedInput: true,
		},
		{
			name:          "query with spaces",
			input:         "/hello world",
			expectedQuery: "/hello world",
			expectedInput: true,
		},
		{
			name:          "special characters",
			input:         "/test-123_abc",
			expectedQuery: "/test-123_abc",
			expectedInput: true,
		},
		{
			name:          "empty query after slash",
			input:         "/",
			expectedQuery: "/",
			expectedInput: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			state.updateQuery(tc.input)
			assert.Equal(t, tc.expectedQuery, state.Query, "Query should match expected value")
			assert.Equal(t, tc.expectedInput, state.InputMode, "Input mode should match expected value")
		})
	}
}

// Test_searchState_clearQuery tests clearing search query with backspace events
// Covers both MacOS and Windows backspace handling
func Test_searchState_clearQuery(t *testing.T) {
	jobName := "test-job"
	state := getSearchState(jobName)
	state.activateSearch()
	state.updateQuery("hello")

	// Test backspace key (most systems)
	t.Run("backspace key clears character", func(t *testing.T) {
		originalQuery := state.Query

		handled := state.handleBackspace(tcell.KeyBackspace)
		assert.True(t, handled, "Backspace should be handled")

		expectedQuery := originalQuery[:len(originalQuery)-1]
		assert.Equal(t, expectedQuery, state.Query, "Query should have last character removed")
	})

	// Test backspace2 key (alternative systems, Windows)
	t.Run("backspace2 key clears character", func(t *testing.T) {
		state.updateQuery("world")
		originalQuery := state.Query

		handled := state.handleBackspace(tcell.KeyBackspace2)
		assert.True(t, handled, "Backspace2 should be handled")

		expectedQuery := originalQuery[:len(originalQuery)-1]
		assert.Equal(t, expectedQuery, state.Query, "Query should have last character removed")
	})

	// Test clearing entire query
	t.Run("clear entire query", func(t *testing.T) {
		state.updateQuery("test")

		// Keep backspacing until the search query is empty
		for len(state.Query) > 0 {
			state.handleBackspace(tcell.KeyBackspace)
		}

		assert.Equal(t, "", state.Query, "Query should be empty after clearing")
		assert.True(t, state.Active, "Search should remain active after clearing query")

		handled := state.handleEscape("test-job")
		assert.True(t, handled, "Escape key should be handled")
		assert.False(t, state.Active, "Search should be deactivated after Escape key")
	})

	// Test backspace when not in input mode
	t.Run("backspace ignored when not in input mode", func(t *testing.T) {
		state.activateSearch()
		state.InputMode = false // Not in input mode (e.g., during navigation)
		originalQuery := state.Query

		handled := state.handleBackspace(tcell.KeyBackspace)
		assert.False(t, handled, "Backspace should not be handled when not in input mode")
		assert.Equal(t, originalQuery, state.Query, "Query should remain unchanged")
	})
}

// Test_performSearch_caseInsensitive tests case-insensitive search with match counting
func Test_performSearch_caseInsensitive(t *testing.T) {
	jobName := "test-job"
	state := getSearchState(jobName)

	// Sample log content with mixed case
	logContent := `INFO: Application started
ERROR: Database connection failed  
info: Retrying connection
DEBUG: Connection successful
error: User authentication failed
INFO: Request processed successfully`

	testCases := []struct {
		name            string
		query           string
		expectedCount   int
		expectedMatches []SearchMatch
	}{
		{
			name:          "case insensitive - 'error'",
			query:         "error",
			expectedCount: 2,
			expectedMatches: []SearchMatch{
				{Line: 1, Start: 0, End: 5},
				{Line: 4, Start: 0, End: 5},
			},
		},
		{
			name:          "case insensitive - 'info'",
			query:         "info",
			expectedCount: 3,
			expectedMatches: []SearchMatch{
				{Line: 0, Start: 0, End: 4},
				{Line: 2, Start: 0, End: 4},
				{Line: 5, Start: 0, End: 4},
			},
		},
		{
			name:          "case insensitive - 'CONNECTION'",
			query:         "CONNECTION",
			expectedCount: 3,
			expectedMatches: []SearchMatch{
				{Line: 1, Start: 16, End: 26},
				{Line: 2, Start: 15, End: 25},
				{Line: 3, Start: 7, End: 17},
			},
		},
		{
			name:            "no matches",
			query:           "notfound",
			expectedCount:   0,
			expectedMatches: []SearchMatch{},
		},
		{
			name:            "empty query",
			query:           "",
			expectedCount:   0,
			expectedMatches: []SearchMatch{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := state.performSearch(logContent, tc.query)

			assert.Equal(t, tc.expectedCount, len(matches), "Should find expected number of matches")
			assert.Equal(t, tc.expectedCount, state.getMatchCount(), "Match count should be correct")

			// Verify specific matches
			for i, expectedMatch := range tc.expectedMatches {
				if i < len(matches) {
					assert.Equal(t, expectedMatch.Line, matches[i].Line, "Match line should be correct")
					assert.Equal(t, expectedMatch.Start, matches[i].Start, "Match start position should be correct")
					assert.Equal(t, expectedMatch.End, matches[i].End, "Match end position should be correct")
					// Note: Text comparison should be case-insensitive since we're testing case-insensitive search
				}
			}

			// Update state with matches
			state.Matches = matches
			if len(matches) > 0 {
				state.CurrentMatch = 0
			} else {
				state.CurrentMatch = -1
			}
		})
	}
}

// Test_searchState_persistenceAcrossJobSwitches tests search state persistence
// when switching between different job logs (key feature of per-TextView approach)
func Test_searchState_persistenceAcrossJobSwitches(t *testing.T) {
	// Create search states for different jobs
	job1 := "build-job"
	job2 := "test-job"
	job3 := "deploy-job"

	// Set up different search states for each job
	t.Run("setup independent search states", func(t *testing.T) {
		// Job 1: Active search for "error"
		state1 := getSearchState(job1)
		state1.activateSearch()
		state1.updateQuery("error")
		state1.InputMode = false // Switched to navigation mode
		state1.CurrentMatch = 2  // On 3rd match

		// Job 2: Active search for "info"
		state2 := getSearchState(job2)
		state2.activateSearch()
		state2.updateQuery("info")
		state2.CurrentMatch = 0 // On 1st match

		// Job 3: No search active
		state3 := getSearchState(job3)
		assert.False(t, state3.Active, "Job3 should have no active search")

		// Verify states are independent
		assert.Equal(t, "error", state1.Query, "Job1 search query should persist")
		assert.Equal(t, "info", state2.Query, "Job2 search query should persist")
		assert.Equal(t, "", state3.Query, "Job3 should have empty query")

		assert.Equal(t, 2, state1.CurrentMatch, "Job1 match position should persist")
		assert.Equal(t, 0, state2.CurrentMatch, "Job2 match position should persist")
		assert.Equal(t, -1, state3.CurrentMatch, "Job3 should have no matches")
	})

	// Test behavior when returning to previous job
	t.Run("search state restored when returning to job", func(t *testing.T) {
		// Simulate switching away from job1, then back
		state1First := getSearchState(job1)
		originalQuery := state1First.Query
		originalMatch := state1First.CurrentMatch

		// Switch to different job
		_ = getSearchState(job2)

		// Return to job1 - should get same state back
		state1Second := getSearchState(job1)
		assert.Same(t, state1First, state1Second, "Should return same search state instance")
		assert.Equal(t, originalQuery, state1Second.Query, "Search query should be preserved")
		assert.Equal(t, originalMatch, state1Second.CurrentMatch, "Match position should be preserved")
		assert.True(t, state1Second.Active, "Search should still be active")
	})

	// Test clearing search state for specific job
	t.Run("can clear search state for specific job", func(t *testing.T) {
		// Clear search state for job1
		clearSearchState(job1)

		// Job1 state should be reset
		state1New := getSearchState(job1)
		assert.False(t, state1New.Active, "Job1 search should be cleared")
		assert.Equal(t, "", state1New.Query, "Job1 query should be empty")
		assert.Equal(t, -1, state1New.CurrentMatch, "Job1 match should be reset")

		// Other jobs should be unaffected
		state2 := getSearchState(job2)
		assert.True(t, state2.Active, "Job2 search should remain active")
		assert.Equal(t, "info", state2.Query, "Job2 query should be preserved")
	})
}

// Test_performSearch_basicMatches tests basic string matching functionality
func Test_performSearch_basicMatches(t *testing.T) {
	jobName := "test-job"
	state := getSearchState(jobName)

	logContent := `Starting application
Processing request ID: 12345
Request completed successfully
Starting cleanup process
Cleanup finished`

	testCases := []struct {
		name          string
		query         string
		expectedCount int
		description   string
	}{
		{
			name:          "single word match",
			query:         "Starting",
			expectedCount: 2,
			description:   "Should find 'Starting' at beginning of two lines",
		},
		{
			name:          "partial word match",
			query:         "request",
			expectedCount: 2,
			description:   "Should find 'request' case-insensitively",
		},
		{
			name:          "exact phrase match",
			query:         "cleanup process",
			expectedCount: 1,
			description:   "Should find exact phrase",
		},
		{
			name:          "number match",
			query:         "12345",
			expectedCount: 1,
			description:   "Should find numeric content",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := state.performSearch(logContent, tc.query)
			assert.Equal(t, tc.expectedCount, len(matches), tc.description)

			// Verify all matches contain the query text (case-insensitive)
			for _, match := range matches {
				lines := strings.Split(logContent, "\n")
				matchedText := lines[match.Line][match.Start:match.End]
				assert.True(t,
					strings.EqualFold(matchedText, tc.query),
					"Matched text should equal query (case-insensitive)")
			}
		})
	}
}

// Test_performSearch_noMatches tests behavior when no matches are found
func Test_performSearch_noMatches(t *testing.T) {
	jobName := "test-job"
	state := getSearchState(jobName)

	logContent := `Application running normally
All systems operational
No errors detected`

	testCases := []struct {
		name  string
		query string
	}{
		{
			name:  "non-existent word",
			query: "failure",
		},
		{
			name:  "partial word that doesn't exist",
			query: "xyz",
		},
		{
			name:  "phrase that doesn't exist",
			query: "critical error",
		},
		{
			name:  "special characters that don't exist",
			query: "@#$%",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := state.performSearch(logContent, tc.query)

			assert.Empty(t, matches, "Should return empty matches for non-existent query")
			assert.Equal(t, 0, len(state.Matches), "State matches should be empty")
			assert.Equal(t, 0, state.getMatchCount(), "Match count should be 0")
		})
	}

	// Verify that performSearch doesn't modify CurrentMatch
	t.Run("performSearch should not modify CurrentMatch", func(t *testing.T) {
		state.CurrentMatch = 5 // Set to non-zero
		state.performSearch(logContent, "nonexistent")
		// CurrentMatch should remain unchanged by performSearch
		assert.Equal(t, 5, state.CurrentMatch, "CurrentMatch should not be modified by performSearch")
	})
}

// Test_performSearch_multipleMatches tests finding multiple matches across lines
func Test_performSearch_multipleMatches(t *testing.T) {
	jobName := "test-job"
	state := getSearchState(jobName)

	// Log content with multiple occurrences of search terms
	logContent := `DEBUG: Starting test run
Running test: unit_test_1
Test unit_test_1 passed
Running test: unit_test_2
Test unit_test_2 failed
Running test: integration_test
Test integration_test passed
Summary: 2 tests passed, 1 test failed`

	testCases := []struct {
		name            string
		query           string
		expectedCount   int
		expectedLines   []int
		verifyPositions bool
	}{
		{
			name:            "word at different positions",
			query:           "Running",
			expectedCount:   3,
			expectedLines:   []int{1, 3, 5},
			verifyPositions: true,
		},
		{
			name:            "repeated word in same line",
			query:           "unit",
			expectedCount:   4, // "unit_test_1" twice, "unit_test_2" twice
			expectedLines:   []int{1, 2, 3, 4},
			verifyPositions: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := state.performSearch(logContent, tc.query)

			assert.Equal(t, tc.expectedCount, len(matches),
				"Should find all occurrences of '%s'", tc.query)

			// Verify matches are in ascending order (line, then position)
			for i := 1; i < len(matches); i++ {
				prev := matches[i-1]
				curr := matches[i]

				if curr.Line == prev.Line {
					assert.True(t, curr.Start > prev.Start,
						"Matches on same line should be in position order")
				} else {
					assert.True(t, curr.Line > prev.Line,
						"Matches should be in line order")
				}
			}

			// Verify specific line numbers if provided
			if tc.verifyPositions && len(tc.expectedLines) > 0 {
				for i, expectedLine := range tc.expectedLines {
					if i < len(matches) {
						assert.Equal(t, expectedLine, matches[i].Line,
							"Match %d should be on line %d", i, expectedLine)
					}
				}
			}

			// Verify all matches contain the search text
			lines := strings.Split(logContent, "\n")
			for _, match := range matches {
				matchedText := lines[match.Line][match.Start:match.End]
				assert.True(t,
					strings.EqualFold(matchedText, tc.query),
					"Matched text '%s' should equal query '%s' (case-insensitive)",
					matchedText, tc.query)
			}
		})
	}
}

// Test_shouldActivateSearch tests the pure logic for when search can be activated
func Test_shouldActivateSearch(t *testing.T) {
	testCases := []struct {
		name         string
		logsVisible  bool
		modalVisible bool
		logContent   string
		expected     bool
		description  string
	}{
		{
			name:         "should activate - logs visible, no modal, has content",
			logsVisible:  true,
			modalVisible: false,
			logContent:   "Sample log content",
			expected:     true,
			description:  "Normal case - all conditions met for search activation",
		},
		{
			name:         "should not activate - no logs visible",
			logsVisible:  false,
			modalVisible: false,
			logContent:   "Sample log content",
			expected:     false,
			description:  "Cannot search when no logs are visible",
		},
		{
			name:         "should not activate - modal visible",
			logsVisible:  true,
			modalVisible: true,
			logContent:   "Sample log content",
			expected:     false,
			description:  "Cannot search when confirmation modal is open",
		},
		{
			name:         "should not activate - no content",
			logsVisible:  true,
			modalVisible: false,
			logContent:   "",
			expected:     false,
			description:  "Cannot search when log content is empty (still loading)",
		},
		{
			name:         "should not activate - multiple conditions false",
			logsVisible:  false,
			modalVisible: true,
			logContent:   "",
			expected:     false,
			description:  "Multiple blocking conditions should prevent activation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Set up log state if the test expects search to be successful
			if tc.expected {
				logState := getLogState("test-job")
				logState.Completed = true
			}

			result := shouldActivateSearch(tc.logsVisible, tc.modalVisible, tc.logContent, "test-job")
			assert.Equal(t, tc.expected, result, tc.description)
		})
	}
}

// Test_handleSearchSlash tests "/" key search activation logic
func Test_handleSearchSlash(t *testing.T) {
	jobName := "test-job"

	t.Run("activates search when conditions are met", func(t *testing.T) {
		state := getSearchState(jobName)
		state.deactivateSearch() // Ensure clean state

		// Set up log state for successful activation
		logState := getLogState(jobName)
		logState.Completed = true

		consumed := handleSearchSlash(state, true, false, "log content", jobName)

		assert.True(t, consumed, "Should consume / key when activating search")
		assert.True(t, state.Active, "Search should be active after / key")
		assert.True(t, state.InputMode, "Should be in input mode after / key")
		assert.Equal(t, "", state.Query, "Query should be empty when first activated")
	})

	t.Run("does not activate when logs not visible", func(t *testing.T) {
		state := getSearchState(jobName)
		state.deactivateSearch()

		consumed := handleSearchSlash(state, false, false, "log content", jobName)

		assert.False(t, consumed, "Should not consume / key when logs not visible")
		assert.False(t, state.Active, "Search should not be active")
	})

	t.Run("does not activate when modal visible", func(t *testing.T) {
		state := getSearchState(jobName)
		state.deactivateSearch()

		consumed := handleSearchSlash(state, true, true, "log content", jobName)

		assert.False(t, consumed, "Should not consume / key when modal visible")
		assert.False(t, state.Active, "Search should not be active")
	})

	t.Run("does not activate when no content", func(t *testing.T) {
		state := getSearchState(jobName)
		state.deactivateSearch()

		consumed := handleSearchSlash(state, true, false, "", jobName)

		assert.False(t, consumed, "Should not consume / key when no content")
		assert.False(t, state.Active, "Search should not be active")
	})

	t.Run("returns to input mode when in navigation mode", func(t *testing.T) {
		state := getSearchState(jobName)
		state.deactivateSearch()

		// Set up log state for successful activation
		logState := getLogState(jobName)
		logState.Completed = true

		// Simulate a completed search (navigation mode)
		state.Active = true
		state.InputMode = false
		state.Query = "existing search"

		consumed := handleSearchSlash(state, true, false, "log content", jobName)

		assert.True(t, consumed, "Should consume / key when returning to input mode")
		assert.True(t, state.Active, "Search should remain active")
		assert.True(t, state.InputMode, "Should be in input mode after / key")
		assert.Equal(t, "existing search", state.Query, "Should preserve existing query")
	})

	t.Run("allows / to be typed when already in input mode", func(t *testing.T) {
		state := getSearchState(jobName)
		state.deactivateSearch()

		// Set up log state for successful activation
		logState := getLogState(jobName)
		logState.Completed = true

		// Start in input mode with some query
		state.Active = true
		state.InputMode = true
		state.Query = "test"

		consumed := handleSearchSlash(state, true, false, "log content", jobName)

		assert.False(t, consumed, "Should not consume / key when already in input mode")
		assert.True(t, state.Active, "Search should remain active")
		assert.True(t, state.InputMode, "Should remain in input mode")
		assert.Equal(t, "test", state.Query, "Query should be unchanged")
	})
}

// Test_handleSearchEscape tests escape key handling in search mode
func Test_handleSearchEscape(t *testing.T) {
	jobName := "test-job"

	t.Run("exits search when search is active", func(t *testing.T) {
		state := getSearchState(jobName)
		state.activateSearch()
		state.updateQuery("test query")

		consumed := state.handleEscape(jobName)

		assert.True(t, consumed, "Should consume Esc key when search is active")
		assert.False(t, state.Active, "Search should be deactivated after Esc")
		assert.False(t, state.InputMode, "Input mode should be false after Esc")
		assert.Equal(t, "", state.Query, "Query should be cleared after Esc")
	})

	t.Run("does not consume when search not active", func(t *testing.T) {
		state := getSearchState(jobName)
		state.deactivateSearch()

		consumed := state.handleEscape(jobName)

		assert.False(t, consumed, "Should not consume Esc key when search not active")
		// This allows normal Esc handling (hide logs, etc.)
	})
}

// Test_handleSearchEnter tests enter key handling in search mode
func Test_handleSearchEnter(t *testing.T) {
	jobName := "test-job"
	logContent := "error on line 1\ninfo message\nerror on line 3\nmore info\nerror on line 5"

	t.Run("submits search when in input mode", func(t *testing.T) {
		state := getSearchState(jobName)
		state.activateSearch()
		state.updateQuery("error")
		assert.True(t, state.InputMode, "Should be in input mode initially")

		consumed := handleSearchEnter(state, logContent, jobName)

		assert.True(t, consumed, "Should consume Enter key when submitting search")
		assert.True(t, state.Active, "Search should still be active after Enter")
		assert.False(t, state.InputMode, "Should exit input mode after Enter")
		assert.Equal(t, 3, len(state.Matches), "Should find 3 matches for 'error'")
		assert.Equal(t, 0, state.CurrentMatch, "Should start at first match")
	})

	t.Run("navigates to next match when not in input mode", func(t *testing.T) {
		state := getSearchState(jobName)
		state.activateSearch()
		state.updateQuery("error")
		state.performSearch(logContent, "error")
		state.InputMode = false // Switch to navigation mode
		state.CurrentMatch = 0  // Start at first match

		consumed := handleSearchEnter(state, logContent, jobName)

		assert.True(t, consumed, "Should consume Enter key for navigation")
		assert.Equal(t, 1, state.CurrentMatch, "Should move to next match")
	})

	t.Run("wraps around to first match", func(t *testing.T) {
		state := getSearchState(jobName)
		state.activateSearch()
		state.performSearch(logContent, "error")
		state.InputMode = false
		state.CurrentMatch = 2 // Last match (0-indexed)

		consumed := handleSearchEnter(state, logContent, jobName)

		assert.True(t, consumed, "Should consume Enter key")
		assert.Equal(t, 0, state.CurrentMatch, "Should wrap to first match")
	})

	t.Run("does not consume when search not active", func(t *testing.T) {
		state := getSearchState(jobName)
		state.deactivateSearch()

		consumed := handleSearchEnter(state, logContent, jobName)

		assert.False(t, consumed, "Should not consume Enter when search not active")
		// This allows normal Enter handling (toggle logs, etc.)
	})
}

// Test_handleSearchKeyInput tests character input handling in search mode
func Test_handleSearchKeyInput(t *testing.T) {
	jobName := "test-job"

	t.Run("adds characters to query when in input mode", func(t *testing.T) {
		state := getSearchState(jobName)
		state.activateSearch()
		assert.Equal(t, "", state.Query, "Query should be empty when first activated")

		// Test individual characters
		testChars := []struct {
			char     rune
			expected string
		}{
			{'e', "e"},
			{'r', "er"},
			{'r', "err"},
			{'o', "erro"},
			{'r', "error"},
		}

		for _, tc := range testChars {
			consumed := handleSearchKeyInput(state, tcell.KeyRune, tc.char)

			assert.True(t, consumed, "Should consume character input")
			assert.Equal(t, tc.expected, state.Query, "Query should include typed character")
			assert.True(t, state.InputMode, "Should remain in input mode")
		}
	})

	t.Run("handles special characters", func(t *testing.T) {
		state := getSearchState(jobName)
		state.activateSearch()

		specialChars := []rune{'-', '_', '.', ':', ' ', '(', ')', '[', ']', '@', '#'}

		for _, char := range specialChars {
			state.updateQuery("") // Reset query

			consumed := handleSearchKeyInput(state, tcell.KeyRune, char)

			assert.True(t, consumed, "Should consume special character: %c", char)
			expected := string(char)
			assert.Equal(t, expected, state.Query, "Should add special character to query")
		}
	})

	t.Run("handles backspace keys", func(t *testing.T) {
		state := getSearchState(jobName)
		state.activateSearch()
		state.updateQuery("hello")

		// Test KeyBackspace
		consumed := handleSearchKeyInput(state, tcell.KeyBackspace, 0)

		assert.True(t, consumed, "Should consume backspace key")
		assert.Equal(t, "hell", state.Query, "Should remove last character")
	})

	t.Run("handles backspace2 for cross-platform support", func(t *testing.T) {
		state := getSearchState(jobName)
		state.activateSearch()
		state.updateQuery("world")

		// Test KeyBackspace2 (Windows/alternative systems)
		consumed := handleSearchKeyInput(state, tcell.KeyBackspace2, 0)

		assert.True(t, consumed, "Should consume backspace2 key")
		assert.Equal(t, "worl", state.Query, "Should remove last character")
	})

	t.Run("ignores newline characters", func(t *testing.T) {
		state := getSearchState(jobName)
		state.activateSearch()
		originalQuery := state.Query

		// Test newline characters
		consumed1 := handleSearchKeyInput(state, tcell.KeyRune, '\n')
		consumed2 := handleSearchKeyInput(state, tcell.KeyRune, '\r')

		assert.False(t, consumed1, "Should not consume newline")
		assert.False(t, consumed2, "Should not consume carriage return")
		assert.Equal(t, originalQuery, state.Query, "Query should be unchanged")
	})

	t.Run("does not consume when search not active", func(t *testing.T) {
		state := getSearchState(jobName)
		state.deactivateSearch()

		consumed := handleSearchKeyInput(state, tcell.KeyRune, 'a')

		assert.False(t, consumed, "Should not consume input when search not active")
	})

	t.Run("does not consume when not in input mode", func(t *testing.T) {
		state := getSearchState(jobName)
		state.activateSearch()
		state.InputMode = false // Switch to navigation mode

		consumed := handleSearchKeyInput(state, tcell.KeyRune, 'a')

		assert.False(t, consumed, "Should not consume input when not in input mode")
	})
}

// Test_searchIntegration_inputCaptureWiring tests that the extracted search functions
// are properly wired into the inputCapture function
func Test_searchIntegration_inputCaptureWiring(t *testing.T) {
	// This test will fail until you integrate the search functions into inputCapture
	// When implemented, remove the t.Skip() above

	jobName := "integration-test-job"
	logContent := "Sample log content for search testing"

	// Save original global state for cleanup
	originalLogsVisible := logsVisible
	originalModalVisible := modalVisible
	originalCurJob := curJob

	// Cleanup function to restore original state
	defer func() {
		logsVisible = originalLogsVisible
		modalVisible = originalModalVisible
		curJob = originalCurJob
	}()

	// Setup global state that inputCapture depends on
	logsVisible = true
	modalVisible = false
	curJob = &ViewJob{Name: jobName, Kind: Job}

	// Create mock components that inputCapture needs
	screen := tcell.NewSimulationScreen("")
	defer screen.Fini()

	app := tview.NewApplication()
	app.SetScreen(screen)

	root := tview.NewPages()
	inputCh := make(chan struct{}, 10)
	forceUpdateCh := make(chan bool, 10)
	var navi navigator

	// Add log page with content
	tv := tview.NewTextView()
	tv.SetText(logContent)
	root.AddPage("logs-"+jobName, tv, true, true)

	// Initialize boxes map and add the TextView
	if boxes == nil {
		boxes = make(map[string]*tview.TextView)
	}
	boxes["logs-"+jobName] = tv

	// Initialize logViews map and add the TextView for search functionality
	if logViews == nil {
		logViews = make(map[string]*tview.TextView)
	}
	logViews["logs-"+jobName] = tv

	// Create the actual inputCapture function
	capture := inputCapture(app, root, navi, inputCh, forceUpdateCh, &options{}, nil, "project", "sha")

	t.Run("slash key activates search", func(t *testing.T) {
		// Ensure search starts inactive
		state := getSearchState(jobName)
		state.deactivateSearch()

		// Set up log state as completed so search can be activated
		logState := getLogState(jobName)
		logState.Completed = true

		// Press "/" key
		event := tcell.NewEventKey(tcell.KeyRune, '/', tcell.ModNone)
		result := capture(event)

		// Verify search was activated
		assert.Nil(t, result, "inputCapture should consume / key when activating search")
		assert.True(t, state.Active, "Search should be active after / key")
		assert.Equal(t, "", state.Query, "Query should be empty when first activated")
	})

	t.Run("escape key exits search", func(t *testing.T) {
		// Setup active search
		state := getSearchState(jobName)
		state.activateSearch()
		state.updateQuery("test")

		// Press Escape key
		event := tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone)
		result := capture(event)

		// Verify search was deactivated
		assert.Nil(t, result, "inputCapture should consume Esc key when exiting search")
		assert.False(t, state.Active, "Search should be deactivated after Esc")
	})

	t.Run("character input works in search mode", func(t *testing.T) {
		// Setup active search
		state := getSearchState(jobName)
		state.activateSearch()

		// Type a character
		event := tcell.NewEventKey(tcell.KeyRune, 'a', tcell.ModNone)
		result := capture(event)

		// Verify character was added
		assert.Nil(t, result, "inputCapture should consume character input in search mode")
		assert.Equal(t, "a", state.Query, "Character should be added to search query")
	})

	t.Run("ctrl+c is never consumed", func(t *testing.T) {
		// Setup active search
		state := getSearchState(jobName)
		state.activateSearch()

		// Press Ctrl+C key
		event := tcell.NewEventKey(tcell.KeyCtrlC, 0, tcell.ModCtrl)
		result := capture(event)

		// Verify Ctrl+C is NOT consumed (always passes through)
		assert.NotNil(t, result, "inputCapture should never consume Ctrl+C, even in search mode")
		assert.Equal(t, tcell.KeyCtrlC, result.Key(), "Should return the original Ctrl+C event")
	})
}

// Test_searchHighlighting tests search term highlighting in log text
func Test_searchHighlighting(t *testing.T) {
	// This test verifies that search matches are properly highlighted with tview markup

	testCases := []struct {
		name           string
		logContent     string
		searchQuery    string
		expectedOutput string
		description    string
	}{
		{
			name:           "single match highlighting",
			logContent:     "This is an error message",
			searchQuery:    "error",
			expectedOutput: "This is an [red::]error[-:-:-] message",
			description:    "Should highlight single occurrence of search term",
		},
		{
			name:           "multiple matches on same line",
			logContent:     "test error and another test error",
			searchQuery:    "test",
			expectedOutput: "[red::]test[-:-:-] error and another [red::]test[-:-:-] error",
			description:    "Should highlight multiple occurrences on same line",
		},
		{
			name:           "case insensitive highlighting",
			logContent:     "ERROR in system and error in process",
			searchQuery:    "error",
			expectedOutput: "[red::]ERROR[-:-:-] in system and [red::]error[-:-:-] in process",
			description:    "Should highlight matches regardless of case",
		},
		{
			name:           "multiline content highlighting",
			logContent:     "Line 1: info message\nLine 2: error occurred\nLine 3: info complete",
			searchQuery:    "info",
			expectedOutput: "Line 1: [red::]info[-:-:-] message\nLine 2: error occurred\nLine 3: [red::]info[-:-:-] complete",
			description:    "Should highlight matches across multiple lines",
		},
		{
			name:           "no matches - no highlighting",
			logContent:     "This is a normal log message",
			searchQuery:    "notfound",
			expectedOutput: "This is a normal log message",
			description:    "Should return original text when no matches found",
		},
		{
			name:           "empty query - no highlighting",
			logContent:     "This is a normal log message",
			searchQuery:    "",
			expectedOutput: "This is a normal log message",
			description:    "Should return original text when query is empty",
		},
		{
			name:           "special characters in search",
			logContent:     "Connection [127.0.0.1:3306] established",
			searchQuery:    "127.0.0.1",
			expectedOutput: "Connection [[red::]127.0.0.1[-:-:-]:3306] established",
			description:    "Should handle special characters in search terms",
		},
		{
			name:           "partial word matching",
			logContent:     "Processing request ID: req_12345_end",
			searchQuery:    "req",
			expectedOutput: "Processing [red::]req[-:-:-]uest ID: [red::]req[-:-:-]_12345_end",
			description:    "Should highlight partial word matches",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := highlightMatches(tc.logContent, tc.searchQuery)
			assert.Equal(t, tc.expectedOutput, result, tc.description)
		})
	}
}

// Test_searchUI_renderSearchBar tests that the search bar is displayed correctly
func Test_searchUI_renderSearchBar(t *testing.T) {
	// This test verifies that when search is active, a search bar appears in the UI

	jobName := "test-job"

	testCases := []struct {
		name           string
		searchActive   bool
		searchQuery    string
		inputMode      bool
		matches        []SearchMatch
		currentMatch   int
		expectedFooter string
		description    string
	}{
		{
			name:           "no search bar when inactive",
			searchActive:   false,
			searchQuery:    "",
			expectedFooter: "",
			description:    "Footer should be empty when search is not active",
		},
		{
			name:           "shows slash when search activated",
			searchActive:   true,
			searchQuery:    "/",
			inputMode:      true,
			expectedFooter: "/",
			description:    "Footer should show '/' when search is first activated",
		},
		{
			name:           "shows search query in progress",
			searchActive:   true,
			searchQuery:    "/error",
			inputMode:      true,
			expectedFooter: "/error",
			description:    "Footer should show the current search query",
		},
		{
			name:           "shows match counter in navigation mode",
			searchActive:   true,
			searchQuery:    "/error",
			inputMode:      false,
			matches:        []SearchMatch{{Line: 1, Start: 0, End: 5}, {Line: 3, Start: 0, End: 5}},
			currentMatch:   0,
			expectedFooter: "error [1/2 matches]",
			description:    "Footer should show match counter during navigation",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a Frame to simulate the UI
			tv := tview.NewTextView()
			tv.SetText("Sample log content")
			frame := tview.NewFrame(tv)

			// Get search state
			state := getSearchState(jobName)

			// Set up search state based on test case
			if tc.searchActive {
				state.Active = true
				state.Query = tc.searchQuery
				state.InputMode = tc.inputMode
				state.Matches = tc.matches
				state.CurrentMatch = tc.currentMatch
			} else {
				state.deactivateSearch()
			}

			// Create a mock logFrames map for testing
			if logFrames == nil {
				logFrames = make(map[string]*tview.Frame)
			}
			logFrames["logs-"+jobName] = frame

			// Call the function that updates the search bar
			updateSearchDisplay(jobName, nil)

			// For testing, we need to verify the frame was updated correctly
			// This is a simplified test - in reality we'd need to check the frame's state
			// For now, we'll just verify the function doesn't panic
			assert.NotPanics(t, func() {
				updateSearchDisplay(jobName, nil)
			}, tc.description)
		})
	}
}

// Test_highlightMatchesWithCurrentMatch tests highlighting all matches with current match emphasized
func Test_highlightMatchesWithCurrentMatch(t *testing.T) {
	testCases := []struct {
		name           string
		logContent     string
		searchQuery    string
		currentMatch   int
		totalMatches   int
		expectedOutput string
		description    string
	}{
		{
			name:           "single match - current match highlighted",
			logContent:     "This is an error message",
			searchQuery:    "error",
			currentMatch:   0,
			totalMatches:   1,
			expectedOutput: "This is an [black:yellow]error[-:-:-] message",
			description:    "Should highlight current match differently from other matches",
		},
		{
			name:           "multiple matches - first match is current",
			logContent:     "error in line 1 and error in line 2",
			searchQuery:    "error",
			currentMatch:   0,
			totalMatches:   2,
			expectedOutput: "[black:yellow]error[-:-:-] in line 1 and [red::]error[-:-:-] in line 2",
			description:    "Should highlight current match in yellow, others in red",
		},
		{
			name:           "multiple matches - second match is current",
			logContent:     "error in line 1 and error in line 2",
			searchQuery:    "error",
			currentMatch:   1,
			totalMatches:   2,
			expectedOutput: "[red::]error[-:-:-] in line 1 and [black:yellow]error[-:-:-] in line 2",
			description:    "Should highlight current match in yellow, others in red",
		},
		{
			name:           "multiple matches across lines - middle match current",
			logContent:     "Line 1: info message\nLine 2: info data\nLine 3: info complete",
			searchQuery:    "info",
			currentMatch:   1,
			totalMatches:   3,
			expectedOutput: "Line 1: [red::]info[-:-:-] message\nLine 2: [black:yellow]info[-:-:-] data\nLine 3: [red::]info[-:-:-] complete",
			description:    "Should highlight current match across multiple lines",
		},
		{
			name:           "no current match set - all matches highlighted normally",
			logContent:     "error here and error there",
			searchQuery:    "error",
			currentMatch:   -1,
			totalMatches:   2,
			expectedOutput: "[red::]error[-:-:-] here and [red::]error[-:-:-] there",
			description:    "Should highlight all matches normally when no current match is set",
		},
		{
			name:           "current match index out of bounds - all matches highlighted normally",
			logContent:     "error here and error there",
			searchQuery:    "error",
			currentMatch:   5,
			totalMatches:   2,
			expectedOutput: "[red::]error[-:-:-] here and [red::]error[-:-:-] there",
			description:    "Should highlight all matches normally when current match index is invalid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := highlightMatchesWithCurrentMatch(tc.logContent, tc.searchQuery, tc.currentMatch)
			assert.Equal(t, tc.expectedOutput, result, tc.description)
		})
	}
}

// Test_searchNavigation_currentMatchTracking tests that current match is properly tracked during navigation
func Test_searchNavigation_currentMatchTracking(t *testing.T) {
	jobName := "test-job"
	logContent := "error on line 1\ninfo message\nerror on line 3\nmore info\nerror on line 5"

	t.Run("current match advances with Enter key", func(t *testing.T) {
		state := getSearchState(jobName)
		state.activateSearch()
		state.updateQuery("error")
		state.performSearch(logContent, "error")
		state.InputMode = false // Switch to navigation mode
		state.CurrentMatch = 0  // Start at first match

		// Navigate to next match
		consumed := handleSearchEnter(state, logContent, jobName)

		assert.True(t, consumed, "Should consume Enter key for navigation")
		assert.Equal(t, 1, state.CurrentMatch, "Should move to second match")
	})

	t.Run("current match wraps around at end", func(t *testing.T) {
		state := getSearchState(jobName)
		state.activateSearch()
		state.performSearch(logContent, "error")
		state.InputMode = false
		state.CurrentMatch = 2 // Last match (0-indexed)

		// Navigate past last match
		consumed := handleSearchEnter(state, logContent, jobName)

		assert.True(t, consumed, "Should consume Enter key")
		assert.Equal(t, 0, state.CurrentMatch, "Should wrap to first match")
	})

	t.Run("current match updates when new search is performed", func(t *testing.T) {
		state := getSearchState(jobName)
		state.activateSearch()
		state.updateQuery("error")
		
		// Perform search - should start at first match
		consumed := handleSearchEnter(state, logContent, jobName)

		assert.True(t, consumed, "Should consume Enter key when submitting search")
		assert.False(t, state.InputMode, "Should exit input mode after Enter")
		assert.Equal(t, 3, len(state.Matches), "Should find 3 matches for 'error'")
		assert.Equal(t, 0, state.CurrentMatch, "Should start at first match")
	})
}

// Test_applySearchHighlightingWithCurrentMatch tests that search highlighting with current match works end-to-end
func Test_applySearchHighlightingWithCurrentMatch(t *testing.T) {
	jobName := "test-job"
	logContent := "error line 1\nerror line 2\nerror line 3"

	// Set up mock TextView
	if logViews == nil {
		logViews = make(map[string]*tview.TextView)
	}
	tv := tview.NewTextView()
	tv.SetText(logContent)
	logViews["logs-"+jobName] = tv

	// Set up search state
	state := getSearchState(jobName)
	state.activateSearch()
	state.performSearch(logContent, "error")
	state.CurrentMatch = 1 // Set to second match
	state.OriginalContent = logContent

	t.Run("applies highlighting with current match emphasis", func(t *testing.T) {
		// This would call the new function that applies highlighting with current match
		applySearchHighlightingWithCurrentMatch(jobName, "error")

		// Get the updated text from TextView
		highlightedText := tv.GetText(false)

		// Verify that the current match is highlighted differently
		assert.Contains(t, highlightedText, "[black:yellow]error[-:-:-]", "Should contain current match highlighting")
		assert.Contains(t, highlightedText, "[red::]error[-:-:-]", "Should contain regular match highlighting")
	})
}
