package cmdutils

import (
	"fmt"
	"strings"

	gitlab "gitlab.com/gitlab-org/api/client-go"
	"gitlab.com/gitlab-org/cli/api"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/glrepo"
	"gitlab.com/gitlab-org/cli/pkg/git"
	"gitlab.com/gitlab-org/cli/pkg/glinstance"
	"gitlab.com/gitlab-org/cli/pkg/iostreams"
	"gitlab.com/gitlab-org/cli/pkg/utils"
)

// Factory is a way to obtain core tools for the commands.
// Safe for concurrent use.
type Factory interface {
	ApiClient(repoHost string, cfg config.Config) (*api.Client, error)
	HttpClient() (*gitlab.Client, error)
	BaseRepo() (glrepo.Interface, error)
	Remotes() (glrepo.Remotes, error)
	Config() config.Config
	Branch() (string, error)
	IO() *iostreams.IOStreams
	DefaultHostname() string
	BuildInfo() api.BuildInfo
}

type DefaultFactory struct {
	io              *iostreams.IOStreams
	config          config.Config
	buildInfo       api.BuildInfo
	defaultHostname string
	defaultProtocol string
	baseRepo        glrepo.Interface
}

func NewFactory(io *iostreams.IOStreams, resolveRepos bool, repository string, cfg config.Config, buildInfo api.BuildInfo) (*DefaultFactory, error) {
	f := &DefaultFactory{
		io:              io,
		config:          cfg,
		buildInfo:       buildInfo,
		defaultHostname: glinstance.DefaultHostname,
		defaultProtocol: glinstance.DefaultProtocol,
	}

	// resolve repository
	var baseRepo glrepo.Interface
	if repository != "" {
		r, err := glrepo.FromFullName(repository, f.defaultHostname)
		if err != nil {
			return nil, fmt.Errorf("failed to get full name from provided repository %q for default hostname %q", repository, f.defaultHostname)
		}
		baseRepo = r
	} else {
		remotes, err := f.Remotes()
		if err != nil {
			return nil, fmt.Errorf("failed to get remotes: %w", err)
		}
		if resolveRepos {
			baseRepo = remotes[0]
		} else {
			// TODO: is the code below even necessary? Can repo.RepoHost() be an empty string?!
			repoHost := f.defaultHostname
			if remotes[0].RepoHost() != "" {
				repoHost = remotes[0].RepoHost()
			}
			ac, err := api.NewClientWithCfg(f.defaultProtocol, repoHost, cfg, false, f.buildInfo.UserAgent())
			if err != nil {
				return nil, fmt.Errorf("failed to create new API client to reesolve repository: %w", err)
			}
			httpClient := ac.Lab()
			repoContext, err := glrepo.ResolveRemotesToRepos(remotes, httpClient, "", f.defaultHostname)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve remotes to repositories: %w", err)
			}
			r, err := repoContext.BaseRepo(f.io.PromptEnabled())
			if err != nil {
				return nil, fmt.Errorf("failed to get base repository from resolved remote repositories: %w", err)
			}
			baseRepo = r
		}
	}

	f.baseRepo = baseRepo
	f.defaultHostname = baseRepo.RepoHost()

	// Fetch the custom host config from env vars, then local config.yml, then global config,yml.
	customGLHost, _ := cfg.Get("", "host")
	if customGLHost != "" {
		if utils.IsValidURL(customGLHost) {
			var protocol string
			customGLHost, protocol = glinstance.StripHostProtocol(customGLHost)
			f.defaultProtocol = protocol
		}
		f.defaultHostname = customGLHost
	}

	return f, nil
}

func (f *DefaultFactory) DefaultHostname() string {
	return f.defaultHostname
}

func (f *DefaultFactory) ApiClient(repoHost string, cfg config.Config) (*api.Client, error) {
	if repoHost == "" {
		repoHost = f.defaultHostname
	}
	c, err := api.NewClientWithCfg(f.defaultProtocol, repoHost, cfg, false, f.buildInfo.UserAgent())
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (f *DefaultFactory) HttpClient() (*gitlab.Client, error) {
	cfg := f.Config()

	c, err := api.NewClientWithCfg(f.defaultProtocol, f.baseRepo.RepoHost(), cfg, false, f.buildInfo.UserAgent())
	if err != nil {
		return nil, err
	}

	return c.Lab(), nil
}

func (f *DefaultFactory) BaseRepo() (glrepo.Interface, error) {
	return f.baseRepo, nil
}

func (f *DefaultFactory) Remotes() (glrepo.Remotes, error) {
	hostOverride := ""
	if !strings.EqualFold(glinstance.DefaultHostname, f.defaultHostname) {
		hostOverride = f.defaultHostname
	}
	rr := &remoteResolver{
		readRemotes:     git.Remotes,
		getConfig:       f.Config,
		defaultHostname: f.defaultHostname,
	}
	fn := rr.Resolver(hostOverride)
	return fn()
}

func (f *DefaultFactory) Config() config.Config {
	return f.config
}

func (f *DefaultFactory) Branch() (string, error) {
	currentBranch, err := git.CurrentBranch()
	if err != nil {
		return "", fmt.Errorf("could not determine current branch: %w", err)
	}
	return currentBranch, nil
}

func (f *DefaultFactory) IO() *iostreams.IOStreams {
	return f.io
}

func (f *DefaultFactory) BuildInfo() api.BuildInfo {
	return f.buildInfo
}
