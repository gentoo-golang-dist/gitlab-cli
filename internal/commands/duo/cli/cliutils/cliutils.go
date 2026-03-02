package cliutils

import "os"

// BuildDuoCLIEnv builds the environment variables for the Duo CLI process.
// This allows the Duo CLI team to track usage sources via GITLAB_DUO_DISTRIBUTION.
func BuildDuoCLIEnv() []string {
	env := os.Environ()
	env = append(env, "GITLAB_DUO_DISTRIBUTION=glab")
	return env
}
