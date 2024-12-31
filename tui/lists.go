package tui

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/joncrangle/podcasts-sync/internal"
)

// StyleSet contains a matched set of title and description styles
type StyleSet struct {
	titleStyle       lipgloss.Style
	descriptionStyle lipgloss.Style
}

type customDelegate struct {
	list.DefaultDelegate
}

var (
	baseListStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			Padding(1)

	focusedListStyle = baseListStyle.
				BorderForeground(lipgloss.Color(Pink))
)

func listKeyMap() list.KeyMap {
	return list.KeyMap{}
}

func newCustomDelegate() customDelegate {
	d := list.NewDefaultDelegate()
	return customDelegate{DefaultDelegate: d}
}

// createStyleSet creates a matched set of title and description styles
func createStyleSet(m list.Model, isTitle bool, color lipgloss.Color, borderStyle lipgloss.Border) StyleSet {
	base := list.NewDefaultItemStyles()
	if isTitle {
		style := base.SelectedTitle.
			Foreground(color).
			BorderStyle(borderStyle).
			BorderForeground(color).
			Bold(true).
			BorderLeft(true).
			PaddingRight(3).
			Width(m.Width())
		descStyle := base.SelectedDesc.
			Foreground(color).
			BorderStyle(borderStyle).
			BorderForeground(color).
			Faint(true).
			BorderLeft(true).
			PaddingRight(3).
			Width(m.Width())
		return StyleSet{titleStyle: style, descriptionStyle: descStyle}
	}

	style := base.NormalTitle.
		Foreground(lipgloss.Color(Text)).
		BorderLeft(false).
		Padding(0, 2, 0, 2).
		Width(m.Width())
	descStyle := base.SelectedDesc.
		Foreground(lipgloss.Color(Subtext0)).
		BorderLeft(false).
		Faint(true).
		Padding(0, 2, 0, 2).
		Width(m.Width())
	return StyleSet{titleStyle: style, descriptionStyle: descStyle}
}

func (d customDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	var (
		title       string
		description string
		styleSet    StyleSet
	)

	isFocused := m.Index() == index

	switch i := listItem.(type) {
	case internal.PodcastEpisode:
		styleSet = d.getPodcastStyles(m, i.Selected, isFocused)
		title = i.Title()
		description = i.Description()

	case internal.USBDrive:
		styleSet = d.getDefaultStyles(m, isFocused)
		title = i.Title()
		description = i.Description()

	case internal.Debug:
		styleSet = d.getDefaultStyles(m, isFocused)
		title = i.Title()
		description = i.Description()

	default:
		return
	}

	content := d.renderContent(title, description, styleSet)
	fmt.Fprint(w, content)
}

func (d customDelegate) getPodcastStyles(m list.Model, isSelected, isFocused bool) StyleSet {
	switch {
	case isSelected && isFocused:
		styles := createStyleSet(m, true, lipgloss.Color(Flamingo), lipgloss.ThickBorder())
		styles.titleStyle = styles.titleStyle.
			BorderForeground(lipgloss.Color(Peach)).
			Foreground(lipgloss.Color(Pink))
		styles.descriptionStyle = styles.descriptionStyle.
			BorderForeground(lipgloss.Color(Peach)).
			Foreground(lipgloss.Color(Pink))
		return styles
	case isSelected:
		return createStyleSet(m, true, lipgloss.Color(Flamingo), lipgloss.ThickBorder())
	case isFocused:
		return createStyleSet(m, true, lipgloss.Color(Mauve), lipgloss.NormalBorder())
	default:
		return createStyleSet(m, false, lipgloss.Color(Text), lipgloss.NormalBorder())
	}
}

func (d customDelegate) getDefaultStyles(m list.Model, isFocused bool) StyleSet {
	if isFocused {
		return createStyleSet(m, true, lipgloss.Color(Mauve), lipgloss.NormalBorder())
	}
	return createStyleSet(m, false, lipgloss.Color(Text), lipgloss.NormalBorder())
}

func (d customDelegate) renderContent(title, description string, styles StyleSet) string {
	renderedTitle := styles.titleStyle.Render(title)
	renderedDesc := styles.descriptionStyle.Render(description)
	return lipgloss.JoinVertical(lipgloss.Left, renderedTitle, renderedDesc)
}

func createList(title string, kind string) list.Model {
	l := list.New([]list.Item{}, newCustomDelegate(), 0, 0)
	l.Title = title
	l.Help = createHelp()
	l.KeyMap = listKeyMap()
	l.SetFilteringEnabled(false)

	// Set list styles
	l.Styles.NoItems = list.DefaultStyles().NoItems.
		Foreground(lipgloss.Color(Text)).
		PaddingLeft(2)
	l.Styles.Title = l.Styles.Title.
		Align(lipgloss.Left).
		Background(lipgloss.Color(MauveDarker)).
		Foreground(lipgloss.Color(Text)).
		Bold(true)

	// Configure kind-specific settings
	switch kind {
	case "mac":
		l.SetStatusBarItemName("podcast", "podcasts")
		l.AdditionalShortHelpKeys = func() []key.Binding {
			return []key.Binding{keys.Space, keys.Sync, keys.SyncAll}
		}
	case "drive":
		l.SetStatusBarItemName("podcast", "podcasts")
		l.AdditionalShortHelpKeys = func() []key.Binding {
			return []key.Binding{keys.Space, keys.Delete, keys.DeleteAll}
		}
	case "select":
		l.SetStatusBarItemName("drive", "drives")
		l.AdditionalShortHelpKeys = func() []key.Binding {
			return []key.Binding{keys.Enter, keys.Escape, keys.Quit}
		}
	}
	return l
}
