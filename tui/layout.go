package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Helper method to update window dimensions
func (m *Model) updateLayoutDimensions() tea.Cmd {
	horizontalMargins := 8
	verticalMargins := 14

	if m.errorMsg != "" {
		verticalMargins += 2
	}

	contentWidth := m.width - horizontalMargins
	contentHeight := m.height - verticalMargins

	listHeight := m.calculateListHeight(contentHeight)
	m.listWidth = contentWidth/2 - 4
	m.listHeight = listHeight

	m.debug.SetSize(contentWidth, contentHeight)
	m.driveSelector.SetSize(40, 18)
	m.driveSelector.Styles.TitleBar = m.driveSelector.Styles.TitleBar.
		Width(40).
		Align(lipgloss.Center)
	m.progress.Width = m.listWidth

	if m.dbgEnabled {
		return addDebugMsg("Window Size",
			fmt.Sprintf("width: %d, height: %d, contentWidth: %d, contentHeight: %d, listWidth: %d, listHeight: %d",
				m.width, m.height, contentWidth, contentHeight, m.listWidth, m.listHeight))
	}

	return nil
}

func (m *Model) calculateListHeight(contentHeight int) int {
	switch {
	case m.height <= 30:
		return 8
	case m.height <= 35:
		return 12
	case m.height <= 40:
		return 17
	case m.width <= 80 || m.height <= 50:
		return 20
	case m.width <= 100 || m.height <= 60:
		return 30
	case m.width <= 123:
		return 35
	case m.width <= 141:
		return 40
	default:
		return contentHeight - 7
	}
}
