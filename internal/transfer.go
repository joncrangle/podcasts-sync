package internal

import (
	"os"
	"sync/atomic"
	"time"
)

type TransferProgress struct {
	CurrentFile      string
	CurrentProgress  float64
	BytesTransferred int64
	TotalBytes       int64
	Speed            float64 // bytes per second
	TimeRemaining    time.Duration
	StartTime        time.Time
	FilesDone        int
	TotalFiles       int
}

type FileOp struct {
	Progress TransferProgress
	Complete bool
	Error    error
}

// ProgressWriter implements io.Writer to track copy progress
type ProgressWriter struct {
	total     int64
	progress  *TransferProgress
	ch        chan<- FileOp
	startTime time.Time
	lastSent  time.Time
	stopping  atomic.Bool
}

func safeClose(ch chan<- FileOp) {
	if ch != nil {
		// Recover in case of a panic due to a closed channel
		defer func() {
			if r := recover(); r != nil {
				return
			}
		}()
		close(ch)
	}
}

func safeSend(ch chan<- FileOp, msg FileOp) {
	defer func() {
		if r := recover(); r != nil {
			// Channel might be closed
			return
		}
	}()
	if ch != nil {
		select {
		case ch <- msg:
		default:
			// If the channel is full, stop the transfer and don't send
			return
		}
	}
}

func NewProgressWriter(total int64, progress *TransferProgress, ch chan<- FileOp) *ProgressWriter {
	return &ProgressWriter{
		total:     total,
		progress:  progress,
		ch:        ch,
		startTime: time.Now(),
	}
}

func (pw *ProgressWriter) Stop() {
	pw.stopping.Store(true)
}

func (pw *ProgressWriter) IsStopped() bool {
	return pw.stopping.Load()
}

func (pw *ProgressWriter) LastUpdate() time.Time {
	return pw.lastSent
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	if pw.stopping.Load() {
		return 0, os.ErrClosed
	}

	// HACK: Tiny delay for better rendering
	time.Sleep(1 * time.Millisecond)
	// DEBUG: Add artificial delay for testing
	if os.Getenv("DEBUG") == "true" {
		time.Sleep(15 * time.Millisecond)
	}

	// Update transferred bytes
	pw.progress.BytesTransferred += int64(n)
	pw.progress.CurrentProgress = float64(pw.progress.BytesTransferred) / float64(pw.total)

	// Calculate speed and time remaining
	elapsed := time.Since(pw.startTime).Seconds()
	if elapsed > 0 {
		pw.progress.Speed = float64(pw.progress.BytesTransferred) / elapsed
		bytesRemaining := pw.total - pw.progress.BytesTransferred
		if pw.progress.Speed > 0 {
			secsRemaining := float64(bytesRemaining) / pw.progress.Speed
			pw.progress.TimeRemaining = time.Duration(secsRemaining) * time.Second
		}
	}

	// Send progress update to the channel every 100ms
	if !pw.stopping.Load() && time.Since(pw.lastSent) >= 50*time.Millisecond {
		fileOp := FileOp{
			Progress: *pw.progress,
			Complete: false,
			Error:    nil,
		}

		if pw.progress.BytesTransferred >= pw.total {
			fileOp.Complete = true
		}

		safeSend(pw.ch, fileOp)
		pw.lastSent = time.Now()
	}

	return n, nil
}
