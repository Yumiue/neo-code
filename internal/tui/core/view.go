package core

import (
	"strings"

	"go-llm-demo/internal/tui/components"

	"github.com/charmbracelet/lipgloss"
)

func (m Model) View() string {
	if m.width < 20 || m.height < 6 {
		return "窗口太小"
	}

	statusHeight := 1
	helpHeight := 0
	if m.mode == ModeHelp {
		helpHeight = minInt(20, m.height-statusHeight-3)
	}

	inputContent := m.renderInputArea()
	inputHeight := countLines(inputContent)
	if inputHeight < 4 {
		inputHeight = 4
	}

	contentHeight := m.height - statusHeight - inputHeight - helpHeight
	if contentHeight < 3 {
		contentHeight = 3
	}

	statusBar := lipgloss.NewStyle().
		Height(statusHeight).
		Width(m.width).
		Render(components.StatusBar{
			Model:      m.activeModel,
			MemoryCnt:  m.memoryStats.TotalItems,
			Generating: m.generating,
			Width:      m.width,
		}.Render())

	viewportView := m.viewport
	viewportView.SetContent(m.renderChatContent())
	chatArea := lipgloss.NewStyle().
		Width(m.width).
		Height(contentHeight).
		Render(viewportView.View())

	inputArea := lipgloss.NewStyle().
		Width(m.width).
		Render(inputContent)

	if m.mode == ModeHelp {
		help := lipgloss.NewStyle().
			Width(m.width).
			Height(helpHeight).
			Render(components.RenderHelp(m.width))
		return lipgloss.JoinVertical(lipgloss.Left, statusBar, chatArea, help, inputArea)
	}

	return lipgloss.JoinVertical(lipgloss.Left, statusBar, chatArea, inputArea)
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func (m Model) renderInputArea() string {
	helpText := "[Enter换行 F5/F8发送 PgUp/PgDn滚动]"
	if !m.generating {
		helpText = "[Enter换行 F5/F8发送 Ctrl+V粘贴 PgUp/PgDn滚动]"
	}

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#5C6370")).
		Render(helpText)

	return m.textarea.View() + "\n" + footer
}

func (m Model) renderChatContent() string {
	return components.MessageList{Messages: m.toComponentMessages(), Width: m.viewport.Width}.Render()
}

func (m Model) toComponentMessages() []components.Message {
	messages := make([]components.Message, len(m.messages))
	for i, msg := range m.messages {
		messages[i] = components.Message{
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: msg.Timestamp,
			Streaming: msg.Streaming,
		}
	}
	return messages
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
