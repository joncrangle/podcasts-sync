package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Helper method to update window dimensions
func (m *Model) updateLayoutDimensions() tea.Cmd {
	horizontalMargins := 8

	// Conservative estimate for non-list components
	// Header (~3-5 lines) + Help (~2 lines) + AppStyle margins (~2 lines) + buffer
	reservedHeight := 12
	if m.errorMsg != "" {
		reservedHeight += 2
	}

	// For very small terminals, reduce reserved space
	if m.height <= 20 {
		reservedHeight = 8
		if m.errorMsg != "" {
			reservedHeight += 2
		}
	}

	contentWidth := m.width - horizontalMargins
	availableHeightForLists := m.height - reservedHeight

	// Ensure reasonable minimum list height
	if availableHeightForLists < 5 {
		availableHeightForLists = 5
	}

	m.listWidth = contentWidth/2 - 4
	m.listHeight = availableHeightForLists

	m.debug.SetSize(contentWidth, availableHeightForLists)
	m.driveSelector.SetSize(40, 18)
	m.driveSelector.Styles.TitleBar = m.driveSelector.Styles.TitleBar.
		Width(40).
		Align(lipgloss.Center)
	m.progress.Width = m.listWidth

	if m.dbgEnabled {
		return addDebugMsg("Layout Debug",
			fmt.Sprintf("screen: %dx%d | reserved: %d | listHeight: %d",
				m.width, m.height, reservedHeight, m.listHeight))
	}

	return nil
}
