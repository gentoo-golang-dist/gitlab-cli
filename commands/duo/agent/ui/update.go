package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			// Capture the user input
			input := m.userInput.Value()
			m.userInput.SetValue("")
			m.messageChannel <- input
			return m, nil
		default:

		}
	case tea.WindowSizeMsg:
		m.messageViewport.Width = msg.Width
		m.messageViewport.Height = msg.Height - 5
		m.userInput.Width = msg.Width
		msgs, _ := m.formatChatMessages()
		m.messageViewport.SetContent(msgs)
		m.messageViewport.GotoBottom()
	case AddError:
		m.err = msg
	case UpdateMessages:
		m.chatMessages = msg
	}

	msgs, _ := m.formatChatMessages()
	m.messageViewport.SetContent(msgs)
	m.messageViewport.GotoBottom()

	var cmd tea.Cmd
	m.userInput, cmd = m.userInput.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	m.messageViewport, cmd = m.messageViewport.Update(msg)
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}
