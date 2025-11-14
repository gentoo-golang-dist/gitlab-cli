# GitLab Duo CLI Integration

This document describes the integration between the GitLab CLI (`glab`) and the GitLab Duo CLI as a plugin.

## Overview

The `glab duo cli` command allows users to run the GitLab Duo CLI without needing to install it separately. The command:

1. Checks that Node.js version 22+ is installed
2. Prompts for user consent (with persistent preferences)
3. Optionally shares the GitLab authentication token
4. Executes the Duo CLI via `npx @gitlab/duo-cli`
5. Provides a fully interactive AI assistant session

## Requirements

- Node.js version 22 or higher

If Node.js is not installed or the version is too old, the command displays installation instructions.

## Usage

```bash
# Launch the interactive Duo CLI
glab duo cli

# View help for the glab duo cli command
glab duo cli --help
```

## User Experience

### First Run

On first run, users are prompted with two questions:

**1. Run the Duo CLI?**

- **Yes**: Run the Duo CLI this time (will prompt again next time)
- **No**: Cancel execution
- **Always**: Save preference to config and skip this prompt in the future

**2. Share your GitLab token with Duo CLI?**

- **Yes**: Share token this time only
- **No**: Don't share this time
- **Always**: Always share token (saves preference)
- **Never**: Never share token (saves preference)

### Subsequent Runs

If "Always" was selected for the run prompt, the command launches directly without prompting.

## Configuration

Preferences are stored in the glab config file (`~/.config/glab-cli/config.yml`):

```yaml
duo_cli_auto_run: "true"      # Skip run confirmation
duo_cli_share_token: "true"   # Always share token
```

### Resetting Preferences

```bash
glab config set duo_cli_auto_run false
glab config set duo_cli_share_token false
```

## Implementation Details

### npx Execution

The command uses `npx @gitlab/duo-cli` which:

- Automatically downloads and caches the package on first use
- Uses the latest version (or cached version if available)
- Doesn't require manual installation or updates
- Avoids permission issues with global npm installs

The Duo CLI is fully interactive and doesn't accept command-line arguments.

### Authentication

Token sharing works with:

- Config file tokens
- Keyring-stored tokens
- Environment variables (`GITLAB_TOKEN`, `GITLAB_ACCESS_TOKEN`)
- 1Password shell plugin: `alias glab='op plugin run -- glab'`

Token is passed via `GITLAB_TOKEN` environment variable to the Duo CLI.

### Stdin Handling

After `huh` prompts complete, the command reopens `/dev/tty` to provide a clean stdin connection for the interactive Duo CLI session.

## Testing

```bash
go test ./internal/commands/duo/cli/...
```

## Dependencies on Unmerged Changes

Includes changes from [MR !2549](https://gitlab.com/gitlab-org/cli/-/merge_requests/2549):

- `ErrUserCancelled` error in iostreams
- Updated `Run()` method for proper cancellation handling

## Related Issues

- [#1053](https://gitlab.com/gitlab-org/cli/-/issues/1053) - Allow CLI extensions to be installed
- [#7970](https://gitlab.com/gitlab-org/cli/-/issues/7970) - Duo CLI distribution discussion
