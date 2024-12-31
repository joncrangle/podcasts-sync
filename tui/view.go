package tui

import (
	"fmt"
	"os"
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
	header := m.createHeader()
	lists := m.createLists()
	help := m.createHelp(m.width, m.help.View(m.keys))

	var errorSection string
	if m.errorMsg != "" {
		errorSection = errorStyle(m.errorMsg)
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		header,
		errorSection,
		lists,
		help,
	)

	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Top,
		appStyle.Render(content))
}

func (m Model) createHeader() string {
	driveInfo := m.formatDriveInfo()
	debug := m.formatDebugInfo()
	titleRender := headingStyle("🎵 Podcasts Sync 🎤")

	title := lipgloss.PlaceHorizontal(
		m.width-lipgloss.Width(driveInfo)-lipgloss.Width(debug)-lipgloss.Width(titleRender)-14,
		lipgloss.Center,
		titleRender,
	)

	return lipgloss.JoinHorizontal(lipgloss.Top, driveInfo, title, debug)
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

func (m Model) createLists() string {
	m.macPodcasts.SetSize(m.listWidth, m.listHeight)
	m.macPodcasts.Styles.NoItems = m.macPodcasts.Styles.NoItems.Width(m.listWidth).Height(m.listHeight)
	m.drivePodcasts.SetSize(m.listWidth, m.listHeight)
	m.drivePodcasts.Styles.NoItems = m.drivePodcasts.Styles.NoItems.Width(m.listWidth).Height(m.listHeight)

	macListHeight := lipgloss.Height(m.macPodcasts.View())
	driveListHeight := lipgloss.Height(m.drivePodcasts.View())
	height := max(macListHeight, driveListHeight)
	macList := m.createMacList(height)
	driveList := m.createDriveList(height)

	return lipgloss.JoinHorizontal(lipgloss.Top, macList, driveList)
}

func (m Model) createMacList(height int) string {
	style := baseListStyle
	if m.focusIndex == 0 {
		style = focusedListStyle
	}
	return style.Width(m.listWidth).Height(height + 2).MarginRight(2).Render(m.macPodcasts.View())
}

func (m Model) createDriveList(height int) string {
	style := baseListStyle
	if m.focusIndex != 0 {
		style = focusedListStyle
	}
	return style.Width(m.listWidth).Height(height + 2).MarginLeft(2).Render(m.drivePodcasts.View())
}

func (m Model) createHelp(width interface{}, helpText string) string {
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
