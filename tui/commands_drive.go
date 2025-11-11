package tui

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/joncrangle/podcasts-sync/internal"
)

type (
	DriveUpdatedMsg  []internal.USBDrive
	DrivesPollMsg    struct{}
	DrivePodcastsMsg struct {
		Podcasts      []internal.PodcastEpisode
		PodcastsDrive []internal.PodcastEpisode
	}
)

type FileOpMsg struct {
	Operation string // "sync" or "delete"
	Msg       internal.FileOp
}

type syncManager struct {
	mu       sync.Mutex
	msgChan  chan internal.FileOp
	tm       *internal.TransferManager
	stopping atomic.Bool
	syncer   *internal.PodcastSync
}

func newSyncManager() *syncManager {
	return &syncManager{
		syncer: internal.NewPodcastSync(),
	}
}

func (sm *syncManager) start(episodes []internal.PodcastEpisode, drive internal.USBDrive) tea.Cmd {
	return func() tea.Msg {
		sm.mu.Lock()
		sm.stopping.Store(false)
		// Larger buffer size to handle frequent progress updates smoothly
		// With 16ms updates, we need more buffer capacity
		sm.msgChan = make(chan internal.FileOp, 200)
		ch := sm.msgChan
		sm.mu.Unlock()

		go func() {
			sm.mu.Lock()
			sm.tm = sm.syncer.StartSync(episodes, drive, ch)
			sm.mu.Unlock()
		}()

		// Wait for first message with timeout to prevent hanging
		select {
		case msg, ok := <-ch:
			if !ok {
				return FileOpMsg{
					Operation: "sync",
					Msg:       internal.FileOp{Complete: true},
				}
			}
			if msg.Error != nil {
				return ErrMsg{msg.Error}
			}
			return FileOpMsg{
				Operation: "sync",
				Msg:       msg,
			}
		case <-time.After(5 * time.Second):
			// Timeout waiting for first message
			return ErrMsg{fmt.Errorf("timeout waiting for sync to start")}
		}
	}
}

func (sm *syncManager) wait() tea.Cmd {
	return func() tea.Msg {
		sm.mu.Lock()
		if sm.msgChan == nil {
			sm.mu.Unlock()
			return FileOpMsg{
				Operation: "sync",
				Msg:       internal.FileOp{Complete: true},
			}
		}
		ch := sm.msgChan
		sm.mu.Unlock()

		// Get the latest message by draining any backlog
		var msg internal.FileOp
		var ok bool

		// First, get at least one message (blocking)
		msg, ok = <-ch
		if !ok {
			return FileOpMsg{
				Operation: "sync",
				Msg:       internal.FileOp{Complete: true},
			}
		}

		// Then drain any buffered messages to get the latest
		for {
			select {
			case newMsg, stillOpen := <-ch:
				if !stillOpen {
					break
				}
				msg = newMsg
			default:
				// No more messages buffered
				goto done
			}
		}

	done:
		if msg.Error != nil {
			return ErrMsg{msg.Error}
		}
		return FileOpMsg{
			Operation: "sync",
			Msg:       msg,
		}
	}
}

func (sm *syncManager) cancel() tea.Cmd {
	return func() tea.Msg {
		sm.mu.Lock()
		defer sm.mu.Unlock()

		sm.stopping.Store(true)
		if sm.tm != nil {
			sm.tm.Stop()
			sm.tm = nil
		}
		if sm.msgChan != nil {
			// Capture channel before clearing it to avoid race
			ch := sm.msgChan
			sm.msgChan = nil
			// Don't close immediately - let any pending messages drain
			go func() {
				time.Sleep(10 * time.Millisecond)
				// Safe close - only this goroutine has access to ch
				defer func() {
					_ = recover() // Ignore panic from closing already-closed channel
				}()
				close(ch)
			}()
		}
		return FileOpMsg{
			Operation: "sync",
			Msg:       internal.FileOp{Complete: true},
		}
	}
}

var (
	driveManager = internal.NewDriveManager("/Volumes", internal.DirectoryTemplate{})
	scanner      = internal.NewPodcastScanner(internal.DirectoryTemplate{})
)

func pollDrivesCmd(milliseconds int) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(time.Duration(milliseconds) * time.Millisecond)
		return DrivesPollMsg{}
	}
}

func getDrives() tea.Msg {
	drives, err := driveManager.DetectDrives()
	if err != nil {
		return ErrMsg{err}
	}
	return DriveUpdatedMsg(drives)
}

func getDrivePodcasts(drive internal.USBDrive, podcasts []internal.PodcastEpisode) tea.Cmd {
	return func() tea.Msg {
		updatedPodcasts := make([]internal.PodcastEpisode, len(podcasts))
		copy(updatedPodcasts, podcasts)
		podcastsBySize := buildPodcastSizeMap(updatedPodcasts)

		podcastsDrive, err := scanner.ScanDrive(drive, podcastsBySize)
		if err != nil {
			return ErrMsg{err}
		}

		return DrivePodcastsMsg{
			Podcasts:      updatedPodcasts,
			PodcastsDrive: podcastsDrive,
		}
	}
}

func buildPodcastSizeMap(podcasts []internal.PodcastEpisode) map[int64][]*internal.PodcastEpisode {
	podcastsBySize := make(map[int64][]*internal.PodcastEpisode)
	for i := range podcasts {
		if podcasts[i].FileSize > 0 {
			podcastsBySize[podcasts[i].FileSize] = append(
				podcastsBySize[podcasts[i].FileSize],
				&podcasts[i],
			)
		}
	}
	return podcastsBySize
}

func deletePodcasts(episodes []internal.PodcastEpisode) tea.Cmd {
	return func() tea.Msg {
		syncer := internal.NewPodcastSync()
		msg := syncer.DeleteSelected(episodes)
		if msg.Error != nil {
			return ErrMsg{msg.Error}
		}
		return FileOpMsg{
			Operation: "delete",
			Msg:       msg,
		}
	}
}
