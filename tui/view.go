package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/joncrangle/podcasts-sync/internal"
)

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Map state to view renderer
	viewRenderers := map[state]func() string{
		driveSelection: m.renderDriveSelection,
		debug:          m.renderDebug,
		transferring:   m.renderTransfer,
		confirm:        m.renderConfirm,
		normal:         m.renderNormal,
	}

	if renderer, ok := viewRenderers[m.state]; ok {
		return renderer()
	}
	return ""
}

func (m Model) renderDriveSelection() string {
	popup := popupStyle.Render(m.driveSelector.View())
	return m.centerInWindow(popup)
}

func (m Model) renderDebug() string {
	popup := debugStyle(m.debug.View())
	return m.centerInWindow(popup)
}

func (m Model) renderTransfer() string {
	progressBar := m.progress.View()
	progressInfo := m.formatProgressInfo(progressBar)
	help := m.createHelp(progressBar, m.transferHelp.View(m.transferKeys))

	progress := lipgloss.JoinVertical(lipgloss.Left,
		progressBar,
		progressInfo,
		help,
	)

	popup := popupStyle.Padding(3).Render(progress)
	return m.centerInWindow(popup)
}

func (m Model) formatProgressInfo(progressBar string) string {
	return progressInfoStyle.Width(lipgloss.Width(progressBar)).Render(fmt.Sprintf(
		"\nTransferring: %s\n"+
			"Progress: %d/%d files\n"+
			"Speed: %.1f MB/s\n"+
			"Remaining: %s\n"+
			"Transferred: %s / %s\n",
		m.transferProgress.CurrentFile,
		m.transferProgress.FilesDone,
		m.transferProgress.TotalFiles,
		m.transferProgress.Speed/1024/1024,
		m.transferProgress.TimeRemaining.Round(time.Second),
		internal.FormatBytes(m.transferProgress.BytesTransferred),
		internal.FormatBytes(m.transferProgress.TotalBytes),
	))
}

func (m Model) renderConfirm() string {
	text := "Are you sure you want to delete the selected file(s)?\n\n\n"
	help := m.createHelp(text, m.confirmHelp.View(m.confirmKeys))
	popup := popupStyle.Render(text + help)
	return m.centerInWindow(popup)
}

func (m Model) renderNormal() string {
	// Create fixed-size components at their natural size
	header := m.createHeader()
	help := m.createHelp(m.width, m.help.View(m.keys))

	var errorSection string
	if m.errorMsg != "" {
		errorSection = errorStyle(m.errorMsg)
	}

	// Calculate space used by fixed components
	fixedHeight := lipgloss.Height(header) + lipgloss.Height(help)
	if errorSection != "" {
		fixedHeight += lipgloss.Height(errorSection)
	}

	// Account for appStyle margins (1 top + 1 bottom = 2)
	fixedHeight += 2

	// Calculate remaining space for lists
	availableForLists := m.height - fixedHeight
	if availableForLists < 3 {
		availableForLists = 3
	}

	// Create lists with constrained height
	lists := m.createListsWithConstrainedHeight(availableForLists)

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		errorSection,
		lists,
		help,
	)

	styledContent := appStyle.Render(content)
	contentHeight := lipgloss.Height(styledContent)

	// If content is still too tall after constraining lists, reduce list height further
	if contentHeight > m.height {
		excessHeight := contentHeight - m.height
		newListHeight := availableForLists - excessHeight
		if newListHeight < 3 {
			newListHeight = 3
		}

		// Recreate with even smaller list height
		lists = m.createListsWithConstrainedHeight(newListHeight)
		content = lipgloss.JoinVertical(lipgloss.Left,
			header,
			errorSection,
			lists,
			help,
		)
		styledContent = appStyle.Render(content)
	}

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Top,
		styledContent)
}

func (m Model) createHeader() string {
	title := "🎵 Podcasts Sync 🎤"

	// Try three-part layout if there's enough space
	driveInfo := m.formatDriveInfo()
	debug := m.formatDebugInfo()
	titleRender := headingStyle(title)

	// Calculate if we have enough space
	totalNeeded := lipgloss.Width(driveInfo) + lipgloss.Width(debug) + lipgloss.Width(titleRender) + 9

	if totalNeeded <= m.width {
		centeredTitle := lipgloss.PlaceHorizontal(
			m.width-lipgloss.Width(driveInfo)-lipgloss.Width(debug)-lipgloss.Width(titleRender)-9,
			lipgloss.Center,
			titleRender,
		)

		return lipgloss.JoinHorizontal(lipgloss.Top, driveInfo, centeredTitle, debug)
	}

	// Fallback: just center the title
	styledTitle := headingStyle(title)
	contentWidth := m.width - 8 // Account for appStyle margins

	return lipgloss.NewStyle().
		Width(contentWidth).
		Align(lipgloss.Center).
		Render(styledTitle)
}

func (m Model) formatDriveInfo() string {
	info := "No drives detected"
	if m.currentDrive.Name != "" {
		info = fmt.Sprintf("Drive: %s > %s",
			m.currentDrive.Name,
			m.currentDrive.Folder)
	}
	return driveStyle(info)
}

func (m Model) formatDebugInfo() string {
	if os.Getenv("DEBUG") == "true" {
		return debugTitleStyle("DEBUG MODE")
	}
	return ""
}

func (m Model) createListsWithConstrainedHeight(availableHeight int) string {
	// Reserve space for help text that will be rendered outside the list
	helpHeight := 2
	viewportHeight := availableHeight - helpHeight

	// Set viewport size to fill available space minus help text
	m.macPodcasts.SetSize(m.listWidth, viewportHeight)
	m.macPodcasts.Styles.NoItems = m.macPodcasts.Styles.NoItems.Width(m.listWidth).Height(viewportHeight)
	m.drivePodcasts.SetSize(m.listWidth, viewportHeight)
	m.drivePodcasts.Styles.NoItems = m.drivePodcasts.Styles.NoItems.Width(m.listWidth).Height(viewportHeight)

	macList := m.createMacList(availableHeight)
	driveList := m.createDriveList(availableHeight)
	return lipgloss.JoinHorizontal(lipgloss.Top, macList, driveList)
}

func (m Model) createMacList(height int) string {
	style := baseListStyle
	if m.focusIndex == 0 {
		style = focusedListStyle
	}

	macListContent := m.macPodcasts.View()
	help := m.createHelp(m.listWidth, m.macPodcasts.Help.View(macHelpKeys))

	// Check if list is empty - if so, no padding needed as the list handles its own height
	if len(m.macPodcasts.Items()) == 0 {
		content := lipgloss.JoinVertical(lipgloss.Left, macListContent, help)
		return style.Width(m.listWidth).Height(height).MarginRight(2).Render(content)
	}

	listContentHeight := lipgloss.Height(macListContent)
	totalContentHeight := listContentHeight - 4
	paddingCount := max(0, height-totalContentHeight)
	padding := strings.Repeat("\n", paddingCount)

	content := lipgloss.JoinVertical(lipgloss.Left, macListContent, padding, help)
	return style.Width(m.listWidth).Height(height).MarginRight(2).Render(content)
}

func (m Model) createDriveList(height int) string {
	style := baseListStyle
	if m.focusIndex != 0 {
		style = focusedListStyle
	}

	driveListContent := m.drivePodcasts.View()
	help := m.createHelp(m.listWidth, m.drivePodcasts.Help.View(driveHelpKeys))

	// Check if list is empty - if so, no padding needed as the list handles its own height
	if len(m.drivePodcasts.Items()) == 0 {
		content := lipgloss.JoinVertical(lipgloss.Left, driveListContent, help)
		return style.Width(m.listWidth).Height(height).MarginLeft(2).Render(content)
	}

	listContentHeight := lipgloss.Height(driveListContent)
	totalContentHeight := listContentHeight - 4
	paddingCount := max(0, height-totalContentHeight)
	padding := strings.Repeat("\n", paddingCount)

	content := lipgloss.JoinVertical(lipgloss.Left, driveListContent, padding, help)
	return style.Width(m.listWidth).Height(height).MarginLeft(2).Render(content)
}

func (m Model) createHelp(width any, helpText string) string {
	var w int
	switch v := width.(type) {
	case int:
		w = v
	case string:
		w = lipgloss.Width(v)
	}
	return lipgloss.Place(w, 1, lipgloss.Center, lipgloss.Center, helpStyle(helpText))
}

func (m Model) centerInWindow(content string) string {
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		content)
}
