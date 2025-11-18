package testing

import (
	"os"
	"testing"

	"go.opencensus.io/stats/view"
)

// SetupOpenCensusForTest disables OpenCensus telemetry in test environments
// and ensures proper cleanup of any started workers.
func SetupOpenCensusForTest(t *testing.T) {
	t.Helper()
	
	// Set environment variables to disable OpenCensus telemetry
	t.Setenv("OPENCENSUS_DISABLE", "true")
	t.Setenv("TESTING", "true")
	t.Setenv("GO_TEST", "1")
	
	// Ensure any existing view workers are stopped
	t.Cleanup(func() {
		// Stop any running view workers
		view.Stop()
	})
}

// DisableOpenCensusGlobally disables OpenCensus telemetry globally.
// This should be called in TestMain or init functions.
func DisableOpenCensusGlobally() {
	os.Setenv("OPENCENSUS_DISABLE", "true")
	os.Setenv("TESTING", "true")
	os.Setenv("GO_TEST", "1")
}