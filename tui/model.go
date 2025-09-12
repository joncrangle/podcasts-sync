// Package tui implements the terminal user interface for the podcasts-sync application.
package tui

import (
	"os"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/joncrangle/podcasts-sync/internal"
)

type state int

const (
	normal state = iota
	driveSelection
	transferring
	confirm
	debug
)

type Loading struct {
	macPodcasts   bool
	drivePodcasts bool
	drives        bool
}

type Model struct {
	loading          Loading
	state            state
	width            int
	height           int
	listWidth        int
	listHeight       int
	macPodcasts      list.Model
	drivePodcasts    list.Model
	driveSelector    list.Model
	debug            list.Model
	help             help.Model
	confirmHelp      help.Model
	transferHelp     help.Model
	keys             KeyMap
	confirmKeys      ConfirmKeyMap
	transferKeys     TransferKeyMap
	progress         progress.Model
	transferSpinner  spinner.Model
	syncManager      *syncManager
	podcasts         []internal.PodcastEpisode
	podcastsDrive    []internal.PodcastEpisode
	currentDrive     internal.USBDrive
	drives           []internal.USBDrive
	debugMsgs        []internal.Debug
	focusIndex       int // 0 = mac list, 1 = drive list
	transferProgress internal.TransferProgress
	statusMsg        string
	errorMsg         string
	dbgEnabled       bool
}

func InitialModel() Model {
	dbgEnabled := os.Getenv("DEBUG") == "true"
	return Model{
		loading:          Loading{macPodcasts: true, drivePodcasts: true, drives: true},
		state:            normal,
		width:            0,
		height:           0,
		listWidth:        0,
		listHeight:       0,
		macPodcasts:      createList("Mac Podcasts", "mac"),
		drivePodcasts:    createList("Drive Podcasts", "drive"),
		driveSelector:    createList("USB Drives", "select"),
		debug:            createList("Debug", "select"),
		help:             createHelp(),
		confirmHelp:      createHelp(),
		transferHelp:     createHelp(),
		keys:             keys,
		confirmKeys:      confirmKeys,
		transferKeys:     transferKeys,
		progress:         createProgress(),
		transferSpinner:  createSpinner(),
		syncManager:      newSyncManager(),
		podcasts:         []internal.PodcastEpisode{},
		podcastsDrive:    []internal.PodcastEpisode{},
		currentDrive:     internal.USBDrive{},
		drives:           []internal.USBDrive{},
		debugMsgs:        []internal.Debug{},
		focusIndex:       0,
		transferProgress: internal.TransferProgress{},
		statusMsg:        "",
		errorMsg:         "",
		dbgEnabled:       dbgEnabled,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		getMacPodcasts,
		getDrives,
		pollDrivesCmd(5000), // Poll drives every 5 seconds
		m.transferSpinner.Tick,
	)
}
