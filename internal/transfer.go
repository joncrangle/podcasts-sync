// Package internal provides core functionality for podcast synchronization,
// including USB drive detection, podcast matching, and file transfer progress tracking.
package internal

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// TransferProgress represents the current state of a file transfer operation.
// All fields are safe to read, but writes should be coordinated through TransferManager.
type TransferProgress struct {
	CurrentFile      string
	CurrentProgress  float64
	BytesTransferred int64
	TotalBytes       int64
	Speed            float64 // bytes per second
	StartTime        time.Time
	FilesDone        int
	TotalFiles       int
}

// TransferManager coordinates file transfer progress tracking across multiple files.
// It maintains accurate byte counts and delegates UI updates to ProgressWriter.
// Safe for concurrent use - all public methods are protected by mutex or atomic operations.
type TransferManager struct {
	totalBytes       int64
	baseOffset       int64 // bytes completed from previous files
	currentFileBytes int64 // bytes transferred in current file
	progress         *TransferProgress
	ch               chan<- FileOp
	pw               *ProgressWriter
	mu               sync.Mutex
}

// FileOp represents a file operation update sent through channels.
type FileOp struct {
	Progress TransferProgress
	Complete bool
	Error    error
}

const (
	// Update frequency and timing
	defaultUpdateInterval    = 33 * time.Millisecond  // 30fps for smooth UI updates
	maxTimeBetweenUpdates    = 200 * time.Millisecond // force update every 200ms for responsive UI
	minSpeedRecalcInterval   = 200 * time.Millisecond // minimum time between speed recalculations
	minElapsedForSpeedSample = 200 * time.Millisecond // minimum elapsed time for valid speed sample

	// Speed calculation
	defaultSpeedSmoothingFactor = 0.2 // moderate exponential smoothing (lower = smoother, higher = more responsive)

	// Progress update thresholds for reducing unnecessary UI updates
	minBytesThresholdBase    = 32 * 1024  // 32KB base threshold
	maxBytesThreshold        = 512 * 1024 // 512KB maximum threshold
	bytesThresholdPercent    = 0.0005     // 0.05% of total bytes
	progressThresholdPercent = 0.003      // 0.3% progress change
)

// ProgressWriter handles asynchronous progress updates and speed calculations.
// It runs a background goroutine that periodically sends progress updates through a channel.
// Thread-safe: uses atomic operations for byte counting and mutexes for progress updates.
// IMPORTANT: Callers MUST call Stop() to clean up the background goroutine.
type ProgressWriter struct {
	total                  int64
	atomicBytesTransferred atomic.Int64
	progress               *TransferProgress
	ch                     chan<- FileOp
	startTime              time.Time
	lastSent               time.Time
	stopping               atomic.Bool

	// Mutex for protecting progress struct updates
	muProgress sync.Mutex

	// Speed calculation state
	muLastSample         sync.Mutex
	lastSampleTime       time.Time
	bytesAtLastSample    int64
	currentSmoothedSpeed float64

	// Update throttling to reduce unnecessary UI updates
	lastSentBytes        int64
	lastSentProgress     float64
	minBytesThreshold    int64
	minProgressThreshold float64

	wg       sync.WaitGroup
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewTransferManager creates a new TransferManager for tracking file transfer progress.
// It automatically starts a background ProgressWriter for UI updates.
// totalBytes: total bytes to transfer across all files
// totalFiles: total number of files to transfer
// ch: channel for sending progress updates (caller owns, TransferManager will not close it)
func NewTransferManager(totalBytes int64, totalFiles int, ch chan<- FileOp) *TransferManager {
	progress := &TransferProgress{
		TotalBytes: totalBytes,
		TotalFiles: totalFiles,
		StartTime:  time.Now(),
	}

	tm := &TransferManager{
		totalBytes: totalBytes,
		progress:   progress,
		ch:         ch,
	}

	tm.pw = NewProgressWriter(totalBytes, progress, ch)

	return tm
}

// StartFile marks the beginning of a new file transfer.
// Resets current file progress and updates the progress display.
func (tm *TransferManager) StartFile(filename string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.currentFileBytes = 0

	// Update progress struct safely (ProgressWriter also reads this)
	if tm.pw != nil {
		tm.pw.muProgress.Lock()
		tm.progress.CurrentFile = filename
		tm.progress.BytesTransferred = tm.baseOffset
		tm.pw.muProgress.Unlock()
	} else {
		tm.progress.CurrentFile = filename
		tm.progress.BytesTransferred = tm.baseOffset
	}

	if tm.pw != nil {
		tm.pw.atomicBytesTransferred.Store(tm.baseOffset)
	}
}

// CompleteFile marks a file transfer as complete and updates base offset.
func (tm *TransferManager) CompleteFile(fileSize int64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.baseOffset += fileSize
	tm.currentFileBytes = 0

	// Update progress struct safely
	if tm.pw != nil {
		tm.pw.muProgress.Lock()
		tm.progress.FilesDone++
		tm.progress.BytesTransferred = tm.baseOffset
		tm.pw.muProgress.Unlock()
	} else {
		tm.progress.FilesDone++
		tm.progress.BytesTransferred = tm.baseOffset
	}

	if tm.pw != nil {
		tm.pw.atomicBytesTransferred.Store(tm.baseOffset)
	}
}

// Write implements io.Writer for tracking bytes transferred during file copy.
// This method is called by io.Copy and similar functions.
func (tm *TransferManager) Write(p []byte) (int, error) {
	n := len(p)

	// Update our tracking
	tm.mu.Lock()
	tm.currentFileBytes += int64(n)
	newTotal := tm.baseOffset + tm.currentFileBytes

	// Update progress struct safely
	if tm.pw != nil {
		tm.pw.muProgress.Lock()
		tm.progress.BytesTransferred = newTotal
		tm.pw.muProgress.Unlock()
	} else {
		tm.progress.BytesTransferred = newTotal
	}
	tm.mu.Unlock()

	// Update the atomic counter for the progress writer
	if tm.pw != nil {
		tm.pw.atomicBytesTransferred.Store(newTotal)
	}

	return n, nil
}

// Stop gracefully shuts down the progress writer.
// Blocks until all background goroutines have exited.
// Safe to call multiple times.
func (tm *TransferManager) Stop() {
	if tm.pw != nil {
		tm.pw.Stop()
	}
}

// IsStopped returns whether the transfer manager has been stopped.
func (tm *TransferManager) IsStopped() bool {
	if tm.pw != nil {
		return tm.pw.IsStopped()
	}
	return false
}

func safeClose(ch chan<- FileOp) {
	if ch != nil {
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
			return
		}
	}()
	if ch != nil {
		select {
		case ch <- msg:
		default:
			return
		}
	}
}

// NewProgressWriter creates a new ProgressWriter that sends periodic updates through ch.
// Starts a background goroutine for asynchronous updates.
// The caller must call Stop() to clean up resources.
func NewProgressWriter(total int64, progress *TransferProgress, ch chan<- FileOp) *ProgressWriter {
	now := time.Now()

	progress.TotalBytes = total
	progress.StartTime = now
	if total > 0 && progress.BytesTransferred > 0 {
		progress.CurrentProgress = float64(progress.BytesTransferred) / float64(total)
	} else if total == 0 {
		progress.CurrentProgress = 1.0
	} else {
		progress.CurrentProgress = 0.0
	}
	progress.Speed = 0.0

	// Calculate dynamic threshold based on total size for smooth updates
	minBytesThreshold := int64(minBytesThresholdBase)
	if total > 0 {
		calculated := int64(float64(total) * bytesThresholdPercent)
		if calculated > minBytesThreshold {
			minBytesThreshold = calculated
		}
		if minBytesThreshold > maxBytesThreshold {
			minBytesThreshold = maxBytesThreshold
		}
	}

	pw := &ProgressWriter{
		total:     total,
		progress:  progress,
		ch:        ch,
		startTime: now,
		lastSent:  now,

		lastSampleTime:       now,
		bytesAtLastSample:    progress.BytesTransferred,
		currentSmoothedSpeed: 0,

		minBytesThreshold:    minBytesThreshold,
		minProgressThreshold: progressThresholdPercent,
		lastSentBytes:        progress.BytesTransferred,
		lastSentProgress:     progress.CurrentProgress,

		stopCh: make(chan struct{}),
	}

	pw.wg.Add(1)
	go pw.senderLoop()

	pw.atomicBytesTransferred.Store(progress.BytesTransferred)

	return pw
}

// Stop gracefully shuts down the progress writer's background goroutine.
// Sends a final update before stopping. Safe to call multiple times.
func (pw *ProgressWriter) Stop() {
	pw.stopOnce.Do(func() {
		pw.stopping.Store(true)
		close(pw.stopCh)
		pw.wg.Wait()
	})
}

// IsStopped returns whether the progress writer has been stopped.
func (pw *ProgressWriter) IsStopped() bool {
	return pw.stopping.Load()
}

// Write is a no-op implementation for interface compatibility.
// TransferManager handles all byte counting using atomic operations.
func (pw *ProgressWriter) Write(p []byte) (int, error) {
	if pw.stopping.Load() {
		return 0, nil
	}
	return len(p), nil
}

// isTransferComplete checks whether the transfer has completed.
func (pw *ProgressWriter) isTransferComplete(actualBytes int64) bool {
	return (actualBytes >= pw.total && pw.total > 0) || pw.total == 0
}

// shouldSendUpdate determines if a progress update should be sent based on thresholds.
func (pw *ProgressWriter) shouldSendUpdate(currentBytes int64, currentProgress float64, isFinalUpdate bool) bool {
	if isFinalUpdate {
		return true
	}

	bytesDiff := currentBytes - pw.lastSentBytes
	if bytesDiff >= pw.minBytesThreshold {
		return true
	}

	progressDiff := math.Abs(currentProgress - pw.lastSentProgress)
	if progressDiff >= pw.minProgressThreshold {
		return true
	}

	if time.Since(pw.lastSent) > maxTimeBetweenUpdates {
		return true
	}

	return false
}

// senderLoop runs in a background goroutine, periodically sending progress updates.
func (pw *ProgressWriter) senderLoop() {
	defer pw.wg.Done()
	ticker := time.NewTicker(defaultUpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-pw.stopCh:
			actualBytes := pw.atomicBytesTransferred.Load()
			pw.performUpdateAndSend(actualBytes, true)
			return
		case <-ticker.C:
			if pw.stopping.Load() {
				return
			}

			actualBytes := pw.atomicBytesTransferred.Load()
			isComplete := pw.isTransferComplete(actualBytes)

			pw.performUpdateAndSend(actualBytes, isComplete)

			if isComplete {
				return
			}
		}
	}
}

// performUpdateAndSend calculates progress and speed, then sends updates if thresholds are met.
// actualBytes: the current number of bytes transferred
// isFinalUpdate: true if this is the final update before stopping
func (pw *ProgressWriter) performUpdateAndSend(actualBytes int64, isFinalUpdate bool) {
	now := time.Now()

	pw.muProgress.Lock()
	pw.progress.BytesTransferred = actualBytes

	if pw.total > 0 {
		pw.progress.CurrentProgress = math.Min(1.0, float64(actualBytes)/float64(pw.total))
	} else {
		pw.progress.CurrentProgress = 1.0
	}

	// Calculate speed
	pw.muLastSample.Lock()
	elapsedSinceLastSample := now.Sub(pw.lastSampleTime)
	bytesSinceLastSample := actualBytes - pw.bytesAtLastSample
	shouldRecalculateSpeed := isFinalUpdate || elapsedSinceLastSample >= minSpeedRecalcInterval

	if shouldRecalculateSpeed && elapsedSinceLastSample >= minElapsedForSpeedSample && bytesSinceLastSample > 0 {
		instantSpeed := float64(bytesSinceLastSample) / elapsedSinceLastSample.Seconds()
		instantSpeed = math.Max(0, instantSpeed)

		if pw.currentSmoothedSpeed == 0 {
			overallElapsed := now.Sub(pw.progress.StartTime).Seconds()
			if overallElapsed > 1.0 && actualBytes > 0 {
				pw.currentSmoothedSpeed = float64(actualBytes) / overallElapsed
			} else {
				pw.currentSmoothedSpeed = instantSpeed
			}
		} else {
			pw.currentSmoothedSpeed = (defaultSpeedSmoothingFactor * instantSpeed) +
				((1 - defaultSpeedSmoothingFactor) * pw.currentSmoothedSpeed)
		}

		pw.currentSmoothedSpeed = math.Max(0, pw.currentSmoothedSpeed)
		pw.bytesAtLastSample = actualBytes
		pw.lastSampleTime = now

	} else if isFinalUpdate && pw.currentSmoothedSpeed == 0 {
		overallElapsed := now.Sub(pw.progress.StartTime).Seconds()
		if overallElapsed > 0 {
			pw.currentSmoothedSpeed = math.Max(0, float64(actualBytes)/overallElapsed)
		}
	}

	pw.progress.Speed = pw.currentSmoothedSpeed
	pw.muLastSample.Unlock()

	// Send update if needed
	shouldSend := pw.shouldSendUpdate(actualBytes, pw.progress.CurrentProgress, isFinalUpdate)

	if pw.ch != nil && shouldSend {
		isComplete := pw.isTransferComplete(actualBytes)
		op := FileOp{
			Progress: *pw.progress,
			Complete: isComplete,
			Error:    nil,
		}

		// Send with timeout to avoid blocking, with panic recovery
		sendSuccessful := false
		if !pw.stopping.Load() {
			func() {
				defer func() {
					// Recover from panic if channel is closed
					if r := recover(); r != nil {
						return
					}
				}()
				select {
				case pw.ch <- op:
					sendSuccessful = true
				case <-time.After(defaultUpdateInterval):
					// Timeout - don't block
				}
			}()
		}

		if sendSuccessful {
			pw.lastSent = now
		}
	}

	// Always update tracking values after checking thresholds to ensure
	// correct incremental calculations on next tick, regardless of send success
	if shouldSend {
		pw.lastSentBytes = actualBytes
		pw.lastSentProgress = pw.progress.CurrentProgress
	}

	pw.muProgress.Unlock()
}
