package internal

import (
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

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

type TransferManager struct {
	totalBytes       int64
	baseOffset       int64 // bytes completed from previous files
	currentFileBytes int64 // bytes transferred in current file
	progress         *TransferProgress
	ch               chan<- FileOp
	pw               *ProgressWriter
	mu               sync.Mutex
}

type FileOp struct {
	Progress TransferProgress
	Complete bool
	Error    error
}

const (
	defaultUpdateInterval       = 33 * time.Millisecond // 30fps
	defaultSpeedSmoothingFactor = 0.2                   // Moderate smoothing
	minSpeedRecalcInterval      = 200 * time.Millisecond
	minElapsedForSpeedSample    = 200 * time.Millisecond
	maxTimeBetweenUpdates       = 1 * time.Second // Force update after this time
)

type ProgressWriter struct {
	total                  int64
	atomicBytesTransferred int64
	progress               *TransferProgress
	ch                     chan<- FileOp
	startTime              time.Time
	lastSent               time.Time
	stopping               atomic.Bool

	// Mutex for protecting progress struct updates
	muProgress sync.Mutex

	// Speed calculation
	muLastSample         sync.Mutex
	lastSampleTime       time.Time
	bytesAtLastSample    int64
	currentSmoothedSpeed float64

	// Buffered updates
	lastSentBytes        int64
	lastSentProgress     float64
	minBytesThreshold    int64
	minProgressThreshold float64

	wg       sync.WaitGroup
	stopCh   chan struct{}
	stopOnce sync.Once
}

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

	// Create a progress writer that tracks continuous progress
	tm.pw = NewProgressWriter(totalBytes, progress, ch)

	return tm
}

func (tm *TransferManager) StartFile(filename string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.currentFileBytes = 0
	tm.progress.CurrentFile = filename

	// Update total bytes transferred to smooth base
	tm.progress.BytesTransferred = tm.baseOffset

	// Sync the progress writer's atomic counter
	if tm.pw != nil {
		atomic.StoreInt64(&tm.pw.atomicBytesTransferred, tm.baseOffset)
	}
}

func (tm *TransferManager) CompleteFile(fileSize int64) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Add completed file to base offset
	tm.baseOffset += fileSize
	tm.currentFileBytes = 0
	tm.progress.FilesDone++

	// Ensure BytesTransferred matches baseOffset
	tm.progress.BytesTransferred = tm.baseOffset

	// Sync the progress writer's atomic counter
	if tm.pw != nil {
		atomic.StoreInt64(&tm.pw.atomicBytesTransferred, tm.baseOffset)
	}
}

func (tm *TransferManager) Write(p []byte) (int, error) {
	tm.mu.Lock()
	tm.currentFileBytes += int64(len(p))
	// Update total bytes to baseOffset + current file progress
	tm.progress.BytesTransferred = tm.baseOffset + tm.currentFileBytes
	tm.mu.Unlock()

	// Forward to the actual progress writer for UI updates, but also update its atomic counter
	n, err := tm.pw.Write(p)
	if err == nil {
		// Sync the progress writer's atomic counter with our tracking
		atomic.StoreInt64(&tm.pw.atomicBytesTransferred, tm.progress.BytesTransferred)
	}
	return n, err
}

func (tm *TransferManager) Stop() {
	if tm.pw != nil {
		tm.pw.Stop()
	}
}

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

	// For smooth updates, use smaller thresholds
	minBytesThreshold := int64(64 * 1024) // 64KB for smooth updates
	if total > 0 {
		// Use 0.05% of total size or 64KB, whichever is larger
		calculated := int64(float64(total) * 0.0005) // 0.05%
		if calculated > minBytesThreshold {
			minBytesThreshold = calculated
		}
		// Cap at 1MB for very large transfers
		if minBytesThreshold > 1024*1024 {
			minBytesThreshold = 1024 * 1024
		}
	}

	pw := &ProgressWriter{
		total:     total,
		progress:  progress,
		ch:        ch,
		startTime: now,
		lastSent:  now,

		// Initialize speed calculation state
		lastSampleTime:       now,
		bytesAtLastSample:    progress.BytesTransferred,
		currentSmoothedSpeed: 0,

		// Use smaller thresholds for smoother updates
		minBytesThreshold:    minBytesThreshold,
		minProgressThreshold: 0.005, // 0.5% for smoother updates
		lastSentBytes:        progress.BytesTransferred,
		lastSentProgress:     progress.CurrentProgress,

		stopCh: make(chan struct{}),
	}

	// Initialize atomic counter with current progress
	atomic.StoreInt64(&pw.atomicBytesTransferred, progress.BytesTransferred)

	pw.wg.Add(1)
	go pw.senderLoop(defaultUpdateInterval, defaultSpeedSmoothingFactor)

	return pw
}

func (pw *ProgressWriter) Stop() {
	pw.stopOnce.Do(func() {
		pw.stopping.Store(true)
		close(pw.stopCh)
		pw.wg.Wait()
	})
}

func (pw *ProgressWriter) IsStopped() bool {
	return pw.stopping.Load()
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	if pw.stopping.Load() {
		return 0, os.ErrClosed
	}
	n := len(p)

	// Note: The TransferManager now controls the atomic counter
	// so we don't update it here anymore to avoid double counting

	return n, nil
}

func (pw *ProgressWriter) shouldSendUpdate(currentBytes int64, currentProgress float64, isFinalUpdate bool) bool {
	if isFinalUpdate {
		return true
	}

	// Send if significant bytes transferred
	bytesDiff := currentBytes - pw.lastSentBytes
	if bytesDiff >= pw.minBytesThreshold {
		return true
	}

	// Send if significant progress change
	progressDiff := math.Abs(currentProgress - pw.lastSentProgress)
	if progressDiff >= pw.minProgressThreshold {
		return true
	}

	// Send if it's been a while (prevent stalling)
	if time.Since(pw.lastSent) > maxTimeBetweenUpdates {
		return true
	}

	return false
}

func (pw *ProgressWriter) senderLoop(updateInterval time.Duration, speedSmoothingFactor float64) {
	defer pw.wg.Done()
	ticker := time.NewTicker(updateInterval)
	defer ticker.Stop()

	running := true
	for running {
		select {
		case <-pw.stopCh:
			running = false
			pw.performUpdateAndSend(true, speedSmoothingFactor)
		case <-ticker.C:
			if pw.stopping.Load() {
				running = false
				continue
			}

			// Use actual bytes for completion check
			actualBytes := atomic.LoadInt64(&pw.atomicBytesTransferred)
			isComplete := (actualBytes >= pw.total && pw.total >= 0) || (pw.total == 0 && actualBytes == 0)
			pw.performUpdateAndSend(isComplete, speedSmoothingFactor)

			if isComplete {
				running = false
			}
		}
	}
}

func (pw *ProgressWriter) performUpdateAndSend(isFinalUpdate bool, _ float64) {
	now := time.Now()

	// Get actual bytes
	actualBytes := atomic.LoadInt64(&pw.atomicBytesTransferred)

	// Protect progress struct updates with mutex
	pw.muProgress.Lock()

	// Use actual bytes for accurate display
	pw.progress.BytesTransferred = actualBytes

	// 1. Update CurrentProgress using actual bytes for accuracy
	if pw.total > 0 {
		pw.progress.CurrentProgress = math.Min(1.0, float64(actualBytes)/float64(pw.total))
	} else {
		// If total is 0, we're done (either empty file or completion)
		pw.progress.CurrentProgress = 1.0
	}

	// 2. Calculate Speed - simple approach
	pw.muLastSample.Lock()
	elapsedSinceLastSample := now.Sub(pw.lastSampleTime)
	bytesSinceLastSample := actualBytes - pw.bytesAtLastSample
	shouldRecalculateSpeed := isFinalUpdate || elapsedSinceLastSample >= minSpeedRecalcInterval

	if shouldRecalculateSpeed && elapsedSinceLastSample >= minElapsedForSpeedSample && bytesSinceLastSample > 0 {
		instantSpeed := float64(bytesSinceLastSample) / elapsedSinceLastSample.Seconds()
		instantSpeed = math.Max(0, instantSpeed)

		if pw.currentSmoothedSpeed == 0 {
			// First speed calculation - use overall average for better accuracy
			overallElapsed := now.Sub(pw.progress.StartTime).Seconds()
			if overallElapsed > 1.0 && actualBytes > 0 {
				pw.currentSmoothedSpeed = float64(actualBytes) / overallElapsed
			} else {
				pw.currentSmoothedSpeed = instantSpeed
			}
		} else {
			// Use exponential moving average
			pw.currentSmoothedSpeed = (defaultSpeedSmoothingFactor * instantSpeed) + ((1 - defaultSpeedSmoothingFactor) * pw.currentSmoothedSpeed)
		}

		pw.currentSmoothedSpeed = math.Max(0, pw.currentSmoothedSpeed)
		pw.bytesAtLastSample = actualBytes
		pw.lastSampleTime = now

	} else if isFinalUpdate && pw.currentSmoothedSpeed == 0 {
		// Final update fallback
		overallElapsed := now.Sub(pw.progress.StartTime).Seconds()
		if overallElapsed > 0 {
			pw.currentSmoothedSpeed = math.Max(0, float64(actualBytes)/overallElapsed)
		}
	}

	pw.progress.Speed = pw.currentSmoothedSpeed
	pw.muLastSample.Unlock()

	// 3. Send the update only if it meets our buffering criteria
	if pw.ch != nil && pw.shouldSendUpdate(actualBytes, pw.progress.CurrentProgress, isFinalUpdate) {
		op := FileOp{
			Progress: *pw.progress,
			Complete: (actualBytes >= pw.total && pw.total >= 0) || (pw.total == 0 && actualBytes == 0),
			Error:    nil,
		}

		sendSuccessful := false
		func() {
			defer func() {
				if r := recover(); r != nil {
					// Channel was closed, ignore
					return
				}
			}()
			select {
			case pw.ch <- op:
				sendSuccessful = true
			case <-pw.stopCh:
				// Stop requested
			case <-time.After(defaultUpdateInterval):
				// Timeout - don't block
			}
		}()

		if sendSuccessful {
			pw.lastSent = now
			pw.lastSentBytes = actualBytes
			pw.lastSentProgress = pw.progress.CurrentProgress
		}
	}

	// Unlock the progress mutex
	pw.muProgress.Unlock()
}
