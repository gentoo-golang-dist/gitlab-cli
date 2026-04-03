package file

import (
	"github.com/xanzy/go-gitlab"
	"gitlab.com/gitlab-org/cli/internal/config"
	"gitlab.com/gitlab-org/cli/internal/glinstance"
	"gitlab.com/gitlab-org/cli/pkg/iostreams"
)

type FileManager struct {
	io     *iostreams.IOStreams
	config config.Config
	gl     *gitlab.Client
}

func NewFileManager(io *iostreams.IOStreams, cfg config.Config, gl *gitlab.Client) *FileManager {
	return &FileManager{
		io:     io,
		config: cfg,
		gl:     gl,
	}
}
