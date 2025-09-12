package tui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

	"github.com/joncrangle/podcasts-sync/internal"
)

func TestInitialModel(t *testing.T) {
	model := InitialModel()

	if model.state != normal {
		t.Errorf("Expected initial state to be normal, got %v", model.state)
	}

	if model.focusIndex != 0 {
		t.Errorf("Expected initial focus index to be 0, got %d", model.focusIndex)
	}

	if !model.loading.macPodcasts || !model.loading.drivePodcasts || !model.loading.drives {
		t.Error("Expected all loading states to be true initially")
	}
}

func TestModelUpdate_WindowSize(t *testing.T) {
	model := InitialModel()
	msg := tea.WindowSizeMsg{Width: 100, Height: 50}

	updatedModel, _ := model.Update(msg)
	m := updatedModel.(Model)

	if m.width != 100 {
		t.Errorf("Expected width to be 100, got %d", m.width)
	}

	if m.height != 50 {
		t.Errorf("Expected height to be 50, got %d", m.height)
	}
}

func TestModelUpdate_MacPodcasts(t *testing.T) {
	model := InitialModel()
	testPodcasts := []internal.PodcastEpisode{
		{
			ZTitle:    "Test Episode 1",
			ShowName:  "Test Show",
			FilePath:  "/test/path1.mp3",
			Published: time.Now(),
			Selected:  false,
			FileSize:  1024,
			OnDrive:   false,
		},
		{
			ZTitle:    "Test Episode 2",
			ShowName:  "Test Show",
			FilePath:  "/test/path2.mp3",
			Published: time.Now(),
			Selected:  false,
			FileSize:  2048,
			OnDrive:   true,
		},
	}

	msg := MacPodcastsMsg(testPodcasts)
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*Model)

	if len(m.podcasts) != 2 {
		t.Errorf("Expected 2 podcasts, got %d", len(m.podcasts))
	}

	if m.loading.macPodcasts {
		t.Error("Expected macPodcasts loading to be false after update")
	}

	if m.podcasts[0].ZTitle != "Test Episode 1" {
		t.Errorf("Expected first podcast title to be 'Test Episode 1', got %s", m.podcasts[0].ZTitle)
	}
}

func TestModelUpdate_DriveUpdate(t *testing.T) {
	model := InitialModel()
	model.loading.macPodcasts = false // Allow drive updates to proceed
	testDrives := []internal.USBDrive{
		{
			Name:      "Test Drive",
			MountPath: "/Volumes/TestDrive",
			Folder:    "podcasts",
		},
	}

	msg := DriveUpdatedMsg(testDrives)
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*Model)

	if len(m.drives) != 1 {
		t.Errorf("Expected 1 drive, got %d", len(m.drives))
	}

	if m.loading.drives {
		t.Error("Expected drives loading to be false after update")
	}
}

func TestModelUpdate_ErrorHandling(t *testing.T) {
	model := InitialModel()
	model.state = transferring

	testErr := &mockError{msg: "test error"}
	msg := ErrMsg{err: testErr}
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*Model)

	if m.state != normal {
		t.Errorf("Expected state to be normal after error, got %v", m.state)
	}

	if m.errorMsg != "test error" {
		t.Errorf("Expected error message to be 'test error', got %s", m.errorMsg)
	}
}

type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}

func TestModelUpdate_FocusNavigation(t *testing.T) {
	model := InitialModel()

	// Test tab navigation
	tabMsg := tea.KeyMsg{Type: tea.KeyTab}
	updatedModel, _ := model.Update(tabMsg)
	m := updatedModel.(*Model)

	if m.focusIndex != 1 {
		t.Errorf("Expected focus index to be 1 after tab, got %d", m.focusIndex)
	}

	// Test left arrow
	leftMsg := tea.KeyMsg{Type: tea.KeyLeft}
	updatedModel, _ = m.Update(leftMsg)
	m = updatedModel.(*Model)

	if m.focusIndex != 0 {
		t.Errorf("Expected focus index to be 0 after left arrow, got %d", m.focusIndex)
	}

	// Test right arrow
	rightMsg := tea.KeyMsg{Type: tea.KeyRight}
	updatedModel, _ = m.Update(rightMsg)
	m = updatedModel.(*Model)

	if m.focusIndex != 1 {
		t.Errorf("Expected focus index to be 1 after right arrow, got %d", m.focusIndex)
	}
}

func TestModelUpdate_StateTransitions(t *testing.T) {
	model := InitialModel()

	// Test drive selection
	selectDriveMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}}
	updatedModel, _ := model.Update(selectDriveMsg)
	m := updatedModel.(*Model)

	if m.state != driveSelection {
		t.Errorf("Expected state to be driveSelection, got %v", m.state)
	}

	// Test escape back to normal
	escapeMsg := tea.KeyMsg{Type: tea.KeyEscape}
	updatedModel, _ = m.Update(escapeMsg)
	m = updatedModel.(*Model)

	if m.state != normal {
		t.Errorf("Expected state to be normal after escape, got %v", m.state)
	}
}

func TestPodcastSelection(t *testing.T) {
	model := InitialModel()
	testPodcasts := []internal.PodcastEpisode{
		{
			ZTitle:   "Test Episode",
			ShowName: "Test Show",
			FilePath: "/test/path.mp3",
			Selected: false,
		},
	}

	// Set up podcasts
	msg := MacPodcastsMsg(testPodcasts)
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(*Model)

	// Test podcast selection with space
	spaceMsg := tea.KeyMsg{Type: tea.KeySpace}
	updatedModel, _ = m.Update(spaceMsg)
	m = updatedModel.(*Model)

	if len(m.podcasts) > 0 && !m.podcasts[0].Selected {
		t.Error("Expected podcast to be selected after space key")
	}
}

func TestModelWithTeatest(t *testing.T) {
	// Initialize model
	model := InitialModel()

	// Create a test model
	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))

	// Send a window resize message
	tm.Send(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Use a short wait instead of WaitFinished since the model runs continuously
	time.Sleep(100 * time.Millisecond)

	// Read the output to verify it rendered
	output := tm.Output()
	buf := make([]byte, 100)
	n, _ := output.Read(buf)
	if n == 0 {
		t.Error("Expected some output from the model")
	}

	// Quit the test cleanly
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
}

func TestKeyboardNavigation(t *testing.T) {
	model := InitialModel()

	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(80, 24))

	// Test navigation keys
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	tm.Send(tea.KeyMsg{Type: tea.KeyLeft})
	tm.Send(tea.KeyMsg{Type: tea.KeyRight})

	// Use a short wait instead of WaitFinished since the model runs continuously
	time.Sleep(100 * time.Millisecond)

	// Verify the model rendered something
	output := tm.Output()
	buf := make([]byte, 100)
	n, _ := output.Read(buf)
	if n == 0 {
		t.Error("Expected some output from the model")
	}

	// Quit the test cleanly
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
}
