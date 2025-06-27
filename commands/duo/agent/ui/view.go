package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/philistino/teacup/markdown"
)

func (m model) View() string {
	var errorMessageStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("9"))

	view := strings.Builder{}

	view.WriteString(m.messageViewport.View())
	view.WriteRune('\n')
	view.WriteString(m.userInput.View())
	if m.err != nil {
		view.WriteString(errorMessageStyle.Render(m.err.Error()))
	}

	return view.String()
}

func (m model) formatChatMessages() (string, error) {
	var userStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("12"))

	var aiStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("5"))

	messages := strings.Builder{}

	for _, msg := range m.chatMessages {
		contentMarkdown, err := markdown.RenderMarkdown(80, msg.Content)
		if err != nil {
			return "", err
		}

		if msg.MessageType == "agent" {
			messages.WriteString(aiStyle.Render("Duo: ") + contentMarkdown + "\n")
		} else {
			messages.WriteString(userStyle.Render("User: ") + contentMarkdown + "\n")
		}
	}

	return messages.String(), nil
}
