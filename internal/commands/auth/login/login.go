package login

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/MakeNowJust/heredoc/v2"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"gitlab.com/gitlab-org/cli/internal/api"
	"gitlab.com/gitlab-org/cli/internal/cmdutils"
	"gitlab.com/gitlab-org/cli/internal/commands/auth/authutils"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/git"
	"gitlab.com/gitlab-org/cli/internal/glinstance"
	"gitlab.com/gitlab-org/cli/internal/iostreams"
	"gitlab.com/gitlab-org/cli/internal/mcpannotations"
	"gitlab.com/gitlab-org/cli/internal/oauth2"
)

type LoginOptions struct {
	IO              *iostreams.IOStreams
	Config          func() config.Config
	apiClient       func(repoHost string) (*api.Client, error)
	defaultHostname string

	Interactive bool

	Hostname string
	Token    string
	JobToken string

	ApiHost     string
	ApiProtocol string
	GitProtocol string

	UseKeyring bool
}

var opts *LoginOptions

func NewCmdLogin(f cmdutils.Factory) *cobra.Command {
	opts = &LoginOptions{
		IO:              f.IO(),
		Config:          f.Config,
		apiClient:       f.ApiClient,
		defaultHostname: f.DefaultHostname(),
	}

	var tokenStdin bool

	cmd := &cobra.Command{
		Use:   "login",
		Args:  cobra.ExactArgs(0),
		Short: "Authenticate with a GitLab instance.",
		Long: heredoc.Docf(`
			Authenticate with a GitLab instance.
			You can pass in a token on standard input by using %[1]s--stdin%[1]s.
			The minimum required scopes for the token are: %[1]sapi%[1]s, %[1]swrite_repository%[1]s.
			Configuration and credentials are stored in the global configuration file (default %[1]s~/.config/glab-cli/config.yml%[1]s)

			When running in interactive mode inside a Git repository, %[1]sglab%[1]s will automatically detect
			GitLab instances from your Git remotes and present them as options, saving you from having to
			manually type the hostname.
		`, "`"),
		Example: heredoc.Docf(`
			# Start interactive setup
			# (If in a Git repository, glab will detect and suggest GitLab instances from remotes)
			$ glab auth login

			# Authenticate against %[1]sgitlab.com%[1]s by reading the token from a file
			$ glab auth login --stdin < myaccesstoken.txt

			# Authenticate with GitLab Self-Managed or GitLab Dedicated
			$ glab auth login --hostname salsa.debian.org

			# Non-interactive setup
			$ glab auth login --hostname gitlab.example.org --token glpat-xxx --api-host gitlab.example.org:3443 --api-protocol https --git-protocol ssh

			# Non-interactive setup reading token from a file
			$ glab auth login --hostname gitlab.example.org --api-host gitlab.example.org:3443 --api-protocol https --git-protocol ssh  --stdin < myaccesstoken.txt

			# Non-interactive CI/CD setup
			$ glab auth login --hostname $CI_SERVER_HOST --job-token $CI_JOB_TOKEN
		`, "`"),
		Annotations: map[string]string{
			mcpannotations.Exclude: "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !opts.IO.PromptEnabled() && !tokenStdin && opts.Token == "" && opts.JobToken == "" {
				return &cmdutils.FlagError{Err: errors.New("'--stdin', '--token', or '--job-token' required when not running interactively.")}
			}

			if opts.JobToken != "" && (opts.Token != "" || tokenStdin) {
				return &cmdutils.FlagError{Err: errors.New("specify one of '--job-token' or '--token' or '--stdin'. You cannot use more than one of these at the same time.")}
			}

			if opts.Token != "" && tokenStdin {
				return &cmdutils.FlagError{Err: errors.New("specify one of '--token' or '--stdin'. You cannot use both flags at the same time.")}
			}

			if tokenStdin {
				defer opts.IO.In.Close()
				token, err := io.ReadAll(opts.IO.In)
				if err != nil {
					return fmt.Errorf("failed to read token from STDIN: %w", err)
				}
				opts.Token = strings.TrimSpace(string(token))
			}

			if opts.IO.PromptEnabled() && opts.Token == "" && opts.JobToken == "" && opts.IO.IsaTTY {
				opts.Interactive = true
			}

			if cmd.Flags().Changed("hostname") {
				if err := hostnameValidator(opts.Hostname); err != nil {
					return &cmdutils.FlagError{Err: fmt.Errorf("error parsing '--hostname': %w", err)}
				}
			}

			if !opts.Interactive && opts.Hostname == "" {
				opts.Hostname = glinstance.DefaultHostname
			}

			if opts.Interactive && (opts.ApiHost != "" || opts.ApiProtocol != "" || opts.GitProtocol != "") {
				return &cmdutils.FlagError{Err: errors.New("api-host, api-protocol, and git-protocol can only be used in non-interactive mode.")}
			}

			if err := loginRun(cmd.Context(), opts); err != nil {
				return cmdutils.WrapError(err, "Could not sign in!")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&opts.Hostname, "hostname", "", "", "The hostname of the GitLab instance to authenticate with.")
	cmd.Flags().StringVarP(&opts.Token, "token", "t", "", "Your GitLab access token.")
	cmd.Flags().StringVarP(&opts.JobToken, "job-token", "j", "", "CI job token.")
	cmd.Flags().BoolVar(&tokenStdin, "stdin", false, "Read token from standard input.")
	cmd.Flags().BoolVar(&opts.UseKeyring, "use-keyring", false, "Store token in your operating system's keyring.")
	cmd.Flags().StringVarP(&opts.ApiHost, "api-host", "a", "", "API host url.")
	cmd.Flags().StringVarP(&opts.ApiProtocol, "api-protocol", "p", "", "API protocol: https, http")
	cmd.Flags().StringVarP(&opts.GitProtocol, "git-protocol", "g", "", "Git protocol: ssh, https, http")

	return cmd
}

func loginRun(ctx context.Context, opts *LoginOptions) error {
	c := opts.IO.Color()
	cfg := opts.Config()

	// Enable keyring mode if requested - do this once at the beginning
	// so all authentication methods benefit from it
	if opts.UseKeyring {
		if err := cfg.Set(opts.Hostname, "use_keyring", "true"); err != nil {
			return err
		}
	}

	if opts.Token != "" {
		if opts.Hostname == "" {
			return errors.New("empty hostname would leak `oauth_token`")
		}

		// Split hostname and subfolder
		hostname, subfolder := splitHostnameAndSubfolder(opts.Hostname)

		err := cfg.Set(hostname, "token", opts.Token)
		if err != nil {
			return err
		}

		if token := config.GetFromEnv("token"); token != "" {
			fmt.Fprintf(opts.IO.StdErr, "%s One of %s environment variables is set. If you don't want to use it for glab, unset it.\n", c.Yellow("WARNING:"), strings.Join(config.EnvKeyEquivalence("token"), ", "))
		}

		if opts.ApiHost != "" {
			err := cfg.Set(hostname, "api_host", opts.ApiHost)
			if err != nil {
				return err
			}
		}

		if subfolder != "" {
			err := cfg.Set(hostname, "subfolder", subfolder)
			if err != nil {
				return err
			}
		}

		if opts.ApiProtocol != "" {
			err := cfg.Set(hostname, "api_protocol", opts.ApiProtocol)
			if err != nil {
				return err
			}
		}

		if opts.GitProtocol != "" {
			err := cfg.Set(hostname, "git_protocol", opts.GitProtocol)
			if err != nil {
				return err
			}
		}

		return cfg.Write()
	}

	if opts.JobToken != "" {
		if opts.Hostname == "" {
			return errors.New("empty hostname would leak `oauth_token`")
		}

		// Split hostname and subfolder
		hostname, subfolder := splitHostnameAndSubfolder(opts.Hostname)

		err := cfg.Set(hostname, "job_token", opts.JobToken)
		if err != nil {
			return err
		}

		if opts.ApiHost != "" {
			err := cfg.Set(hostname, "api_host", opts.ApiHost)
			if err != nil {
				return err
			}
		}

		if subfolder != "" {
			err := cfg.Set(hostname, "subfolder", subfolder)
			if err != nil {
				return err
			}
		}

		if opts.ApiProtocol != "" {
			err := cfg.Set(hostname, "api_protocol", opts.ApiProtocol)
			if err != nil {
				return err
			}
		}

		if opts.GitProtocol != "" {
			err := cfg.Set(hostname, "git_protocol", opts.GitProtocol)
			if err != nil {
				return err
			}
		}

		return cfg.Write()
	}

	// Split hostname into base hostname and subfolder if present
	var subfolder string
	hostname, _ := splitHostnameAndSubfolder(opts.Hostname)
	apiHostname := hostname

	if opts.ApiHost != "" {
		apiHostname = opts.ApiHost
	}

	isSelfHosted := false

	if hostname == "" {
		// Try to detect GitLab hosts from git remotes
		detectedHosts, detectErr := detectGitLabHosts(cfg)

		if detectErr == nil && len(detectedHosts) > 0 {
			// We have detected hosts, present them to the user
			options := make([]string, 0, len(detectedHosts)+1)
			for _, host := range detectedHosts {
				options = append(options, host.String())
			}
			options = append(options, promptLoginDifferentHostname)

			var selectedOption string
			err := opts.IO.Select(ctx, &selectedOption, "Found GitLab instances in git remotes. Select one:", options)
			if err != nil {
				return fmt.Errorf("could not prompt: %w", err)
			}

			// Check if user selected "Enter a different hostname"
			if selectedOption == promptLoginDifferentHostname {
				// Fall back to manual entry
				hostname = opts.defaultHostname
				apiHostname = hostname

				hostnameInput := huh.NewInput().
					Title("GitLab hostname:").
					Value(&hostname).
					Placeholder(opts.defaultHostname).
					Validate(func(s string) error {
						return hostnameValidator(s)
					})
				err := opts.IO.Run(ctx, hostnameInput)
				if err != nil {
					return fmt.Errorf("could not prompt: %w", err)
				}

				// Set default for API hostname
				if apiHostname == opts.defaultHostname {
					apiHostname = hostname
				}

				apiHostnameInput := huh.NewInput().
					Title("API hostname:").
					Description("For instances with a different hostname for the API endpoint.").
					Value(&apiHostname).
					Placeholder(hostname).
					Validate(func(s string) error {
						return hostnameValidator(s)
					})
				err = opts.IO.Run(ctx, apiHostnameInput)
				if err != nil {
					return fmt.Errorf("could not prompt: %w", err)
				}
			} else {
				// User selected a detected host - find it in the list
				for _, host := range detectedHosts {
					if host.String() == selectedOption {
						hostname = host.hostname
						apiHostname = hostname
						break
					}
				}
			}
		} else {
			// No detected hosts or detection failed, fall back to original behavior
			options := []string{}
			if hosts, err := cfg.Hosts(); err == nil {
				options = append(options, hosts...)
			}
			if !slices.Contains(options, opts.defaultHostname) {
				options = append(options, opts.defaultHostname)
			}
			options = append(options, promptSelfManagedOrDedicatedInstance)

			var selectedOption string
			err := opts.IO.Select(ctx, &selectedOption, "What GitLab instance do you want to sign in to?", options)
			if err != nil {
				return fmt.Errorf("could not prompt: %w", err)
			}

			isSelfHosted = selectedOption == promptSelfManagedOrDedicatedInstance

			if isSelfHosted {
				hostname = opts.defaultHostname
				apiHostname = hostname

				hostnameInput := huh.NewInput().
					Title("GitLab hostname:").
					Value(&hostname).
					Placeholder(opts.defaultHostname).
					Validate(func(s string) error {
						return hostnameValidator(s)
					})
				err := opts.IO.Run(ctx, hostnameInput)
				if err != nil {
					return fmt.Errorf("could not prompt: %w", err)
				}

				// Set default for API hostname
				if apiHostname == opts.defaultHostname {
					apiHostname = hostname
				}

				apiHostnameInput := huh.NewInput().
					Title("API hostname:").
					Description("For instances with a different hostname for the API endpoint.").
					Value(&apiHostname).
					Placeholder(hostname).
					Validate(func(s string) error {
						return hostnameValidator(s)
					})
				err = opts.IO.Run(ctx, apiHostnameInput)
				if err != nil {
					return fmt.Errorf("could not prompt: %w", err)
				}
			} else {
				hostname = selectedOption
				apiHostname = hostname
			}
		}
	} else {
		isSelfHosted = glinstance.IsSelfHosted(hostname)
	}

	fmt.Fprintf(opts.IO.StdErr, "- Signing into %s\n", hostname)

	if token := config.GetFromEnv("token"); token != "" {
		fmt.Fprintf(opts.IO.StdErr, "%s One of %s environment variables is set. If you don't want to use it for glab, unset it.\n", c.Yellow("WARNING:"), strings.Join(config.EnvKeyEquivalence("token"), ", "))
	}
	existingToken, _, _ := cfg.GetWithSource(hostname, "token", false)

	if existingToken != "" && opts.Interactive {
		apiClient, err := opts.apiClient(hostname)
		if err != nil {
			return err
		}

		user, _, err := apiClient.Lab().Users.CurrentUser()
		if err == nil {
			username := user.Username
			keepGoing := false // default value
			confirm := huh.NewConfirm().
				Title(fmt.Sprintf(
					"You're already logged into %s as %s. Do you want to re-authenticate?",
					hostname,
					username)).
				Value(&keepGoing)
			err = opts.IO.Run(ctx, confirm)
			if err != nil {
				return fmt.Errorf("could not prompt: %w", err)
			}

			if !keepGoing {
				return nil
			}
		}
	}

	var (
		loginType                string
		containerRegistryDomains string
	)

	if opts.Interactive {
		loginTypeOptions := []string{promptLoginTypeToken, promptLoginTypeWeb}
		err := opts.IO.Select(ctx, &loginType, "How would you like to sign in?", loginTypeOptions)
		if err != nil {
			return fmt.Errorf("could not get sign-in type: %w", err)
		}

		containerRegistryDomains = defaultContainerRegistryDomainsString(hostname)
		containerRegistryInput := huh.NewInput().
			Title("What domains does this host use for the container registry and image dependency proxy?").
			Value(&containerRegistryDomains).
			Placeholder(defaultContainerRegistryDomainsString(hostname))
		err = opts.IO.Run(ctx, containerRegistryInput)
		if err != nil {
			return fmt.Errorf("could not get container registry domains: %w", err)
		}
	}

	var token string
	var err error
	if strings.EqualFold(loginType, promptLoginTypeToken) {
		token, err = showTokenPrompt(ctx, opts.IO, hostname)
		if err != nil {
			return err
		}
	} else {
		client, err := opts.apiClient(hostname)
		if err != nil {
			return err
		}

		token, err = oauth2.StartFlow(ctx, cfg, opts.IO.StdErr, client.HTTPClient(), hostname)
		if err != nil {
			return err
		}
	}

	// Re-split hostname in case it was changed by prompts
	hostname, subfolder = splitHostnameAndSubfolder(hostname)

	if err := cfg.Set(hostname, "token", token); err != nil {
		return err
	}

	if err := setContainerRegistryDomains(cfg, hostname, containerRegistryDomains); err != nil {
		return err
	}

	if hostname == "" {
		return errors.New("empty hostname would leak the token")
	}

	if err := cfg.Set(hostname, "api_host", apiHostname); err != nil {
		return err
	}

	// Set subfolder if present
	if subfolder != "" {
		if err := cfg.Set(hostname, "subfolder", subfolder); err != nil {
			return err
		}
	}

	// Detect and optionally set SSH host
	sshHost := detectSSHHost(hostname)
	if sshHost != "" && sshHost != hostname {
		// Found different SSH host
		if opts.Interactive {
			var setSSHHost bool
			prompt := fmt.Sprintf("Detected SSH hostname: %s. Use this for SSH operations?", sshHost)
			confirmInput := huh.NewConfirm().
				Title(prompt).
				Value(&setSSHHost).
				Affirmative("Yes").
				Negative("No")
			err := opts.IO.Run(ctx, confirmInput)
			if err != nil {
				return fmt.Errorf("could not prompt: %w", err)
			}
			if setSSHHost {
				if err := cfg.Set(hostname, "ssh_host", sshHost); err != nil {
					return err
				}
			}
		} else {
			// In non-interactive mode, auto-set if detected
			if err := cfg.Set(hostname, "ssh_host", sshHost); err != nil {
				return err
			}
		}
	}

	gitProtocol := "https"
	apiProtocol := "https"

	glabExecutable := "glab"
	if exe, err := os.Executable(); err == nil {
		glabExecutable = exe
	}
	credentialFlow := &authutils.GitCredentialFlow{Executable: glabExecutable}

	if opts.Interactive {
		gitProtocolOptions := []string{promptProtocolSSH, promptProtocolHTTPS, promptProtocolHTTP}
		gitProtocol = promptProtocolHTTPS // Set default
		err = opts.IO.Select(ctx, &gitProtocol, "Choose default Git protocol:", gitProtocolOptions)
		if err != nil {
			return fmt.Errorf("could not prompt: %w", err)
		}

		gitProtocol = strings.ToLower(gitProtocol)
		if opts.Interactive && gitProtocol != "ssh" {
			if err := credentialFlow.Prompt(ctx, opts.IO, hostname, gitProtocol); err != nil {
				return err
			}
		}

		if isSelfHosted {
			apiProtocolOptions := []string{promptProtocolHTTPS, promptProtocolHTTP}
			apiProtocol = promptProtocolHTTPS // Set default
			err = opts.IO.Select(ctx, &apiProtocol, "Choose host API protocol:", apiProtocolOptions)
			if err != nil {
				return fmt.Errorf("could not prompt: %w", err)
			}

			apiProtocol = strings.ToLower(apiProtocol)
		}

		fmt.Fprintf(opts.IO.StdErr, "- glab config set -h %s git_protocol %s\n", hostname, gitProtocol)
		if err := cfg.Set(hostname, "git_protocol", gitProtocol); err != nil {
			return err
		}

		fmt.Fprintf(opts.IO.StdErr, "%s Configured Git protocol.\n", c.GreenCheck())

		fmt.Fprintf(opts.IO.StdErr, "- glab config set -h %s api_protocol %s\n", hostname, apiProtocol)
		if err := cfg.Set(hostname, "api_protocol", apiProtocol); err != nil {
			return err
		}

		fmt.Fprintf(opts.IO.StdErr, "%s Configured API protocol.\n", c.GreenCheck())
	}
	apiClient, err := opts.apiClient(hostname)
	if err != nil {
		return err
	}

	user, _, err := apiClient.Lab().Users.CurrentUser()
	if err != nil {
		return fmt.Errorf("error using API: %w", err)
	}
	username := user.Username

	if err := cfg.Set(hostname, "user", username); err != nil {
		return err
	}

	err = cfg.Write()
	if err != nil {
		return err
	}

	if credentialFlow.ShouldSetup() {
		err := credentialFlow.Setup(hostname, gitProtocol, username, token)
		if err != nil {
			return err
		}
	}

	fmt.Fprintf(opts.IO.StdErr, "%s Logged in as %s\n", c.GreenCheck(), c.Bold(username))
	fmt.Fprintf(opts.IO.StdErr, "%s Configuration saved to %s\n", c.GreenCheck(), config.ConfigFile())
	fmt.Fprintf(opts.IO.StdErr, "  - Host: %s\n", hostname)
	if subfolder != "" {
		fmt.Fprintf(opts.IO.StdErr, "  - Subfolder: %s\n", subfolder)
	}
	if sshHostValue, _ := cfg.Get(hostname, "ssh_host"); sshHostValue != "" {
		fmt.Fprintf(opts.IO.StdErr, "  - SSH host: %s\n", sshHostValue)
	}

	return nil
}

func hostnameValidator(v any) error {
	s, ok := v.(string)
	if !ok {
		return errors.New("hostname must be a string")
	}

	if strings.TrimSpace(s) == "" {
		return errors.New("hostname cannot be empty")
	}

	// NOTE: adding a scheme here so that `url.Parse`
	// doesn't interpret the first segment before a colon
	// as a scheme. We never expect `v` to contain
	// a scheme anyways.
	val := fmt.Sprintf("https://%s", s)
	_, err := url.Parse(val)
	if err != nil {
		return fmt.Errorf("invalid hostname: %w", err)
	}

	return nil
}

func getAccessTokenTip(hostname string) string {
	return fmt.Sprintf(`
	The minimum required scopes are 'api' and 'write_repository'.
	Generate a personal access token at https://%s/-/user_settings/personal_access_tokens?scopes=api,write_repository`, hostname)
}

func showTokenPrompt(ctx context.Context, io *iostreams.IOStreams, hostname string) (string, error) {
	fmt.Fprintln(io.StdErr)
	fmt.Fprintln(io.StdErr, heredoc.Doc(getAccessTokenTip(hostname)))

	var token string
	tokenInput := huh.NewInput().
		Title("Paste your authentication token:").
		Value(&token).
		EchoMode(huh.EchoModePassword).
		Validate(func(s string) error {
			if s == "" {
				return fmt.Errorf("required")
			}
			return nil
		})
	err := io.Run(ctx, tokenInput)
	if err != nil {
		return "", fmt.Errorf("could not prompt: %w", err)
	}

	return token, nil
}

func defaultContainerRegistryDomainsString(hostname string) string {
	if !strings.Contains(hostname, ":") {
		return strings.Join(
			[]string{
				hostname,
				net.JoinHostPort(hostname, "443"),
				"registry." + hostname,
			}, ",")
	}

	return strings.Join(
		[]string{
			hostname,
			"registry." + hostname,
		}, ",")
}

func setContainerRegistryDomains(cfg config.Config, hostname string, domains string) error {
	return cfg.Set(hostname, "container_registry_domains", domains)
}

// splitHostnameAndSubfolder splits a hostname that may contain a subfolder path.
// Examples:
//   - "example.com" → ("example.com", "")
//   - "example.com/gitlab" → ("example.com", "gitlab")
//   - "example.com:3000/gitlab" → ("example.com:3000", "gitlab")
//   - "https://example.com/gitlab" → ("example.com", "gitlab")
func splitHostnameAndSubfolder(input string) (string, string) {
	// Ensure the input has a scheme for proper URL parsing
	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		input = "https://" + input
	}

	// Parse the URL
	u, err := url.Parse(input)
	if err != nil {
		// Fallback to string manipulation if parsing fails
		input = strings.TrimPrefix(input, "https://")
		input = strings.TrimPrefix(input, "http://")
		input = strings.TrimSuffix(input, "/")
		return glinstance.ExtractSubfolder(input)
	}

	// Use u.Host to preserve port information (e.g., "example.com:3000")
	hostname := u.Host
	subfolder := strings.Trim(u.Path, "/")

	return hostname, subfolder
}

// detectSSHHost attempts to detect the SSH hostname from git remotes.
// Returns empty string if not found or same as HTTP hostname.
func detectSSHHost(httpHostname string) string {
	// Check if we're in a git repository
	remotes, err := git.Remotes()
	if err != nil {
		return ""
	}

	// Look for SSH remotes and extract hostname
	for _, remote := range remotes {
		if remote.FetchURL != nil && remote.FetchURL.Scheme == "ssh" {
			sshHost := remote.FetchURL.Hostname()
			if sshHost != "" && sshHost != httpHostname {
				return sshHost
			}
		}
		// Also check SCP-style URLs (git@host:path)
		if remote.FetchURL != nil {
			// Try to parse as SCP-style: git@hostname:path
			urlStr := remote.FetchURL.String()
			if strings.Contains(urlStr, "@") && strings.Contains(urlStr, ":") && !strings.Contains(urlStr, "://") {
				// This looks like SCP-style
				parts := strings.SplitN(urlStr, "@", 2)
				if len(parts) == 2 {
					hostParts := strings.SplitN(parts[1], ":", 2)
					if len(hostParts) == 2 && hostParts[0] != httpHostname {
						return hostParts[0]
					}
				}
			}
		}
	}

	return ""
}
