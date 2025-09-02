package view

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
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
		assert.Equal(t, "/", state.Query, "Query should start with '/' after activation")
	})

	// Test search mode deactivation
	t.Run("can deactivate search mode", func(t *testing.T) {
		state := getSearchState(jobName)

		// Start with active search
		state.activateSearch()
		state.updateQuery("/test")

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
		job1State.updateQuery("/job1search")

		// Activate different search for job2
		job2State.activateSearch()
		job2State.updateQuery("/job2search")

		// Verify states are independent
		assert.Equal(t, "/job1search", job1State.Query, "Job1 should maintain its search query")
		assert.Equal(t, "/job2search", job2State.Query, "Job2 should maintain its search query")
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
			input:         "/hello",
			expectedQuery: "/hello",
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
	state.updateQuery("/hello")

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
		state.updateQuery("/world")
		originalQuery := state.Query

		handled := state.handleBackspace(tcell.KeyBackspace2)
		assert.True(t, handled, "Backspace2 should be handled")

		expectedQuery := originalQuery[:len(originalQuery)-1]
		assert.Equal(t, expectedQuery, state.Query, "Query should have last character removed")
	})

	// Test clearing entire query
	t.Run("clear entire query", func(t *testing.T) {
		state.updateQuery("/test")

		// Keep backspacing until only slash remains
		for len(state.Query) > 1 {
			state.handleBackspace(tcell.KeyBackspace)
		}

		assert.Equal(t, "/", state.Query, "Query should be only slash after clearing")

		// One more backspace should exit search mode
		handled := state.handleBackspace(tcell.KeyBackspace)
		assert.True(t, handled, "Final backspace should be handled")
		assert.False(t, state.Active, "Search should be deactivated after clearing slash")
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
				{Line: 1, Start: 0, End: 5, Text: "ERROR"},
				{Line: 4, Start: 0, End: 5, Text: "error"},
			},
		},
		{
			name:          "case insensitive - 'info'",
			query:         "info",
			expectedCount: 3,
			expectedMatches: []SearchMatch{
				{Line: 0, Start: 0, End: 4, Text: "INFO"},
				{Line: 2, Start: 0, End: 4, Text: "info"},
				{Line: 5, Start: 0, End: 4, Text: "INFO"},
			},
		},
		{
			name:          "case insensitive - 'CONNECTION'",
			query:         "CONNECTION",
			expectedCount: 3,
			expectedMatches: []SearchMatch{
				{Line: 1, Start: 16, End: 26, Text: "connection"},
				{Line: 2, Start: 15, End: 25, Text: "connection"},
				{Line: 3, Start: 7, End: 17, Text: "Connection"},
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
		state1.updateQuery("/error")
		state1.InputMode = false // Switched to navigation mode
		state1.CurrentMatch = 2  // On 3rd match

		// Job 2: Active search for "info"
		state2 := getSearchState(job2)
		state2.activateSearch()
		state2.updateQuery("/info")
		state2.CurrentMatch = 0 // On 1st match

		// Job 3: No search active
		state3 := getSearchState(job3)
		assert.False(t, state3.Active, "Job3 should have no active search")

		// Verify states are independent
		assert.Equal(t, "/error", state1.Query, "Job1 search query should persist")
		assert.Equal(t, "/info", state2.Query, "Job2 search query should persist")
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
		assert.Equal(t, "/info", state2.Query, "Job2 query should be preserved")
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
