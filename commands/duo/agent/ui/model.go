package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	"gitlab.com/gitlab-org/cli/commands/duo/agent/api"
)

type model struct {
	userInput       textinput.Model
	messageViewport viewport.Model

	chatMessages   []api.CheckpointChatMessage
	err            error
	messageChannel chan string
}

const width = 80

func InitialModel(messageChannel chan string) model {
	userInput := textinput.New()
	userInput.Focus()
	userInput.Width = width

	messageViewport := viewport.New(width, 20)

	return model{
		userInput:       userInput,
		messageChannel:  messageChannel,
		messageViewport: messageViewport,
	}
}
