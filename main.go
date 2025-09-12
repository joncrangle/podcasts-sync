package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/joncrangle/podcasts-sync/tui"
)

var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "Show application version")
	showVersionShort := flag.Bool("v", false, "Show application version (short)")

	flag.Parse()

	if *showVersion || *showVersionShort {
		fmt.Println("Version:", version)
		os.Exit(0)
	}

	initialModel := tui.InitialModel()
	p := tea.NewProgram(initialModel, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Failed to start TUI application: %v\n", err)
		os.Exit(1)
	}
}
