package cmdutils

import (
	"fmt"
	"os"
	"strings"
)

// ParseRepoOverrideEarly extracts the repo override from command line arguments
// before full command parsing. This enables early repository resolution.
func ParseRepoOverrideEarly(args []string) (string, error) {
	// Environment variable takes precedence when no flag is provided
	repo, gotRepo := os.LookupEnv("GITLAB_REPO")

	// Parse command line arguments for -R or --repo
loop:
	for i, arg := range args {
		switch {
		case arg == "-R" || arg == "--repo":
			// Flag with separate value: -R value or --repo value
			if i+1 < len(args) {
				repo = args[i+1]
				gotRepo = true
			}
			break loop
		case strings.HasPrefix(arg, "--repo="):
			// Flag with inline value: --repo=value
			repo = strings.TrimPrefix(arg, "--repo=")
			gotRepo = true
			break loop
		}
	}

	if !gotRepo {
		return "", nil
	}

	// Validate repo override value
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", fmt.Errorf("the repo override flag is provided without a value")
	}

	if strings.HasPrefix(repo, "-") {
		return "", fmt.Errorf("the repo override flag is provided without a valid value %q. Could this be another flag?", repo)
	}

	// Return environment variable if no flag found
	return repo, nil
}
