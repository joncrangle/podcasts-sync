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
	defaultUpdateInterval       = 50 * time.Millisecond
	defaultSpeedSmoothingFactor = 0.15
	minSpeedRecalcInterval      = 750 * time.Millisecond
	minElapsedForSpeedSample    = 100 * time.Millisecond
)

type ProgressWriter struct {
	total     int64
	progress  *TransferProgress
	ch        chan<- FileOp
	startTime time.Time
	lastSent  time.Time
	stopping  atomic.Bool

	muLastSample         sync.Mutex
	lastSampleTime       time.Time
	bytesAtLastSample    int64
	currentSmoothedSpeed float64

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

		// Initialize speed calculation state for the new file
		lastSampleTime:       now,
		bytesAtLastSample:    progress.BytesTransferred, // Start sampling from current known transferred
		currentSmoothedSpeed: 0,                         // Reset smoothed speed for new writer
		stopCh:               make(chan struct{}),
	}

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

func (pw *ProgressWriter) LastUpdate() time.Time {
	return pw.lastSent
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	if pw.stopping.Load() {
		return 0, os.ErrClosed
	}
	n := len(p)
	pw.progress.BytesTransferred += int64(n)
	return n, nil
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
				// final update will be handled by stopCh path or already occurred if transfer completed
				continue
			}
			isComplete := (pw.progress.BytesTransferred >= pw.total && pw.total >= 0) || (pw.total == 0 && pw.progress.BytesTransferred == 0)
			pw.performUpdateAndSend(isComplete, speedSmoothingFactor)

			if isComplete {
				running = false // Stop the loop
			}
		}
	}
}

func (pw *ProgressWriter) performUpdateAndSend(isFinalUpdate bool, speedSmoothingFactor float64) {
	now := time.Now()
	currentBytes := pw.progress.BytesTransferred

	// 1. Update CurrentProgress
	if pw.total > 0 {
		pw.progress.CurrentProgress = math.Min(1.0, float64(currentBytes)/float64(pw.total))
	} else if currentBytes == 0 { // 0 total bytes, 0 transferred (empty file)
		pw.progress.CurrentProgress = 1.0
	} else {
		pw.progress.CurrentProgress = 0.0
	}

	// 2. Calculate and Update Speed (EMA)
	pw.muLastSample.Lock()
	elapsedSinceLastSample := now.Sub(pw.lastSampleTime)
	bytesSinceLastSample := currentBytes - pw.bytesAtLastSample
	shouldRecalculateSpeed := isFinalUpdate || elapsedSinceLastSample >= minSpeedRecalcInterval

	if shouldRecalculateSpeed && elapsedSinceLastSample >= minElapsedForSpeedSample {
		instantSpeed := 0.0
		if elapsedSinceLastSample.Seconds() > 0 { // Avoid division by zero
			instantSpeed = float64(bytesSinceLastSample) / elapsedSinceLastSample.Seconds()
		}
		instantSpeed = math.Max(0, instantSpeed) // Ensure non-negative

		if pw.currentSmoothedSpeed == 0 && pw.bytesAtLastSample == pw.progress.StartTime.UnixNano() {
			// Special handling for the very first sample: use overall average if available and sensible

			overallElapsed := now.Sub(pw.progress.StartTime).Seconds()
			if overallElapsed > 0.5 && (currentBytes-(pw.progress.BytesTransferred-bytesSinceLastSample)) > 0 {
				overallSpeed := float64(currentBytes-(pw.progress.BytesTransferred-bytesSinceLastSample)) / overallElapsed
				if pw.currentSmoothedSpeed == 0 && pw.lastSampleTime.Equal(pw.startTime) {
					pw.currentSmoothedSpeed = overallSpeed
				} else { // Blend instant with previous smoothed
					pw.currentSmoothedSpeed = (speedSmoothingFactor * instantSpeed) + ((1 - speedSmoothingFactor) * pw.currentSmoothedSpeed)
				}
			} else {
				// Not enough overall data, or very first sample, use instant speed directly if previous EMA was 0
				if pw.currentSmoothedSpeed == 0 {
					pw.currentSmoothedSpeed = instantSpeed
				} else {
					pw.currentSmoothedSpeed = (speedSmoothingFactor * instantSpeed) + ((1 - speedSmoothingFactor) * pw.currentSmoothedSpeed)
				}
			}
		} else {
			// Standard EMA update
			pw.currentSmoothedSpeed = (speedSmoothingFactor * instantSpeed) + ((1 - speedSmoothingFactor) * pw.currentSmoothedSpeed)
		}

		pw.currentSmoothedSpeed = math.Max(0, pw.currentSmoothedSpeed) // Ensure non-negative

		// Update sample tracking info only if we actually used this sample for EMA
		pw.bytesAtLastSample = currentBytes
		pw.lastSampleTime = now
	} else if isFinalUpdate && pw.currentSmoothedSpeed == 0 {
		// If it's a final update and we never got a good speed, calculate overall average as a last resort
		overallElapsed := now.Sub(pw.progress.StartTime).Seconds()
		if overallElapsed > 0 {
			pw.currentSmoothedSpeed = math.Max(0, float64(currentBytes-(pw.progress.BytesTransferred-bytesSinceLastSample))/overallElapsed)
		}
	}

	pw.progress.Speed = pw.currentSmoothedSpeed
	pw.muLastSample.Unlock()

	// 3. Calculate and Update TimeRemaining
	if pw.progress.Speed > 1e-9 { // Check for speed effectively > 0 to avoid division by zero/inf
		bytesRemaining := pw.total - currentBytes
		if bytesRemaining <= 0 {
			pw.progress.TimeRemaining = 0
		} else {
			secondsRemaining := float64(bytesRemaining) / pw.progress.Speed
			// Cap time remaining to a very large value
			if secondsRemaining > float64(time.Hour*24*365/time.Second) { // Cap at 1 year
				pw.progress.TimeRemaining = time.Duration(math.MaxInt64 / 2)
			} else {
				pw.progress.TimeRemaining = time.Duration(secondsRemaining * float64(time.Second))
			}
		}
	} else if currentBytes < pw.total && pw.total > 0 {
		pw.progress.TimeRemaining = time.Duration(math.MaxInt64) // Effectively infinite
	} else {
		pw.progress.TimeRemaining = 0 // Completed or 0 total bytes
	}

	// 4. Send the update
	if pw.ch != nil {
		op := FileOp{
			Progress: *pw.progress,
			Complete: (currentBytes >= pw.total && pw.total >= 0) || (pw.total == 0 && currentBytes == 0),
			Error:    nil,
		}

		sendSuccessful := false
		func() {
			defer func() {
				_ = recover() // ignore panic if channel is closed
			}()
			select {
			case pw.ch <- op:
				sendSuccessful = true
			case <-pw.stopCh:
			case <-time.After(defaultUpdateInterval / 2): // Timeout
			}
		}()
		if sendSuccessful {
			// Only update lastSent if send was successful, and do it under lock
			// if LastUpdate() could be called concurrently from a different goroutine.
			// For simplicity here, assuming LastUpdate is not called with high concurrency
			// against this specific write.
			// pw.muLastSample.Lock()
			pw.lastSent = time.Now()
			// pw.muLastSample.Unlock()
		}
	}
}
