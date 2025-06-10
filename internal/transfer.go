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

const (
	defaultUpdateInterval       = 16 * time.Millisecond // 60fps
	defaultSpeedSmoothingFactor = 0.1
	minSpeedRecalcInterval      = 100 * time.Millisecond
	minElapsedForSpeedSample    = 50 * time.Millisecond
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

	// Interpolation for smooth progress
	muInterpolation    sync.Mutex
	lastRealBytes      int64
	lastRealTime       time.Time
	interpolationSpeed float64

	wg       sync.WaitGroup
	stopCh   chan struct{}
	stopOnce sync.Once
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
	progress.TimeRemaining = time.Duration(math.MaxInt64)

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

		// Initialize interpolation state
		lastRealBytes:      progress.BytesTransferred,
		lastRealTime:       now,
		interpolationSpeed: 0,

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
	newBytes := atomic.AddInt64(&pw.atomicBytesTransferred, int64(n))

	// Update interpolation baseline when actual data arrives
	now := time.Now()
	pw.muInterpolation.Lock()
	pw.lastRealBytes = newBytes
	pw.lastRealTime = now
	// Use current smoothed speed for interpolation
	pw.muLastSample.Lock()
	pw.interpolationSpeed = pw.currentSmoothedSpeed
	pw.muLastSample.Unlock()
	pw.muInterpolation.Unlock()

	return n, nil
}

// simulateTransfer simulates a transfer for files that already exist
func (pw *ProgressWriter) simulateTransfer(bytes int64) {
	if pw.stopping.Load() {
		return
	}

	newBytes := atomic.AddInt64(&pw.atomicBytesTransferred, bytes)

	// Update interpolation baseline
	now := time.Now()
	pw.muInterpolation.Lock()
	pw.lastRealBytes = newBytes
	pw.lastRealTime = now
	pw.muInterpolation.Unlock()

	// Force an immediate progress update for simulation
	pw.performUpdateAndSend(false, defaultSpeedSmoothingFactor)
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

			// Use actual bytes for completion check, not interpolated
			actualBytes := atomic.LoadInt64(&pw.atomicBytesTransferred)
			isComplete := (actualBytes >= pw.total && pw.total >= 0) || (pw.total == 0 && actualBytes == 0)
			pw.performUpdateAndSend(isComplete, speedSmoothingFactor)

			if isComplete {
				running = false
			}
		}
	}
}

func (pw *ProgressWriter) performUpdateAndSend(isFinalUpdate bool, speedSmoothingFactor float64) {
	now := time.Now()

	// Get actual bytes
	actualBytes := atomic.LoadInt64(&pw.atomicBytesTransferred)

	// Protect progress struct updates with mutex
	pw.muProgress.Lock()

	// Use actual bytes for reliable percentage calculation
	pw.progress.BytesTransferred = actualBytes

	// 1. Update CurrentProgress using actual bytes for accuracy
	if pw.total > 0 {
		pw.progress.CurrentProgress = math.Min(1.0, float64(actualBytes)/float64(pw.total))
	} else {
		// If total is 0, we're done (either empty file or completion)
		pw.progress.CurrentProgress = 1.0
	}

	// 2. Calculate Speed using actual bytes (for accuracy)
	pw.muLastSample.Lock()
	elapsedSinceLastSample := now.Sub(pw.lastSampleTime)
	bytesSinceLastSample := actualBytes - pw.bytesAtLastSample
	shouldRecalculateSpeed := isFinalUpdate || elapsedSinceLastSample >= minSpeedRecalcInterval

	if shouldRecalculateSpeed && elapsedSinceLastSample >= minElapsedForSpeedSample {
		instantSpeed := 0.0
		if elapsedSinceLastSample.Seconds() > 0 {
			instantSpeed = float64(bytesSinceLastSample) / elapsedSinceLastSample.Seconds()
		}
		instantSpeed = math.Max(0, instantSpeed)

		if pw.currentSmoothedSpeed == 0 && pw.lastSampleTime.Equal(pw.progress.StartTime) {
			// First sample: use overall average if available
			overallElapsed := now.Sub(pw.progress.StartTime).Seconds()
			if overallElapsed > 0.5 && actualBytes > 0 {
				overallSpeed := float64(actualBytes) / overallElapsed
				pw.currentSmoothedSpeed = overallSpeed
			} else {
				pw.currentSmoothedSpeed = instantSpeed
			}
		} else {
			// Standard EMA update
			pw.currentSmoothedSpeed = (speedSmoothingFactor * instantSpeed) + ((1 - speedSmoothingFactor) * pw.currentSmoothedSpeed)
		}

		pw.currentSmoothedSpeed = math.Max(0, pw.currentSmoothedSpeed)
		pw.bytesAtLastSample = actualBytes
		pw.lastSampleTime = now

		// Update interpolation speed with new smoothed speed
		pw.muInterpolation.Lock()
		pw.interpolationSpeed = pw.currentSmoothedSpeed
		pw.muInterpolation.Unlock()

	} else if isFinalUpdate && pw.currentSmoothedSpeed == 0 {
		// Final update fallback
		overallElapsed := now.Sub(pw.progress.StartTime).Seconds()
		if overallElapsed > 0 {
			pw.currentSmoothedSpeed = math.Max(0, float64(actualBytes)/overallElapsed)
		}
	}

	pw.progress.Speed = pw.currentSmoothedSpeed
	pw.muLastSample.Unlock()

	// 3. Calculate TimeRemaining based on actual bytes remaining
	if pw.progress.Speed > 1e-9 {
		bytesRemaining := pw.total - actualBytes
		if bytesRemaining <= 0 {
			pw.progress.TimeRemaining = 0
		} else {
			secondsRemaining := float64(bytesRemaining) / pw.progress.Speed
			if secondsRemaining > float64(time.Hour*24*365/time.Second) {
				pw.progress.TimeRemaining = time.Duration(math.MaxInt64 / 2)
			} else {
				pw.progress.TimeRemaining = time.Duration(secondsRemaining * float64(time.Second))
			}
		}
	} else if actualBytes < pw.total && pw.total > 0 {
		pw.progress.TimeRemaining = time.Duration(math.MaxInt64)
	} else {
		pw.progress.TimeRemaining = 0
	}

	// 4. Send the update - use actual bytes for completion check
	if pw.ch != nil {
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
			pw.lastSent = time.Now()
		}
	}

	// Unlock the progress mutex
	pw.muProgress.Unlock()
}
