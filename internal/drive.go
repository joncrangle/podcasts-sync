package internal

import (
	"io"
	"os"
	"path/filepath"
	"time"
)

type USBDrive struct {
	Name      string
	MountPath string
	Folder    string
}

func (d USBDrive) Title() string { return d.Name }

func (d USBDrive) Description() string { return d.MountPath }

func (d USBDrive) FilterValue() string { return d.Name }

type DirectoryTemplate struct {
	ShowNameFormat string
	EpisodeFormat  string
	DateFormat     string
	SanitizeNames  bool
	CreateIndex    bool
}

var defaultDirTemplate = DirectoryTemplate{
	ShowNameFormat: "{show}",
	EpisodeFormat:  "{date} - {title}",
	DateFormat:     "2006-01-02",
	SanitizeNames:  true,
}

type DriveManager struct {
	volumesPath string
	template    DirectoryTemplate
}

// NewDriveManager creates a new DriveManager instance
func NewDriveManager(volumesPath string, template DirectoryTemplate) *DriveManager {
	if template == (DirectoryTemplate{}) {
		template = defaultDirTemplate
	}
	return &DriveManager{
		volumesPath: volumesPath,
		template:    template,
	}
}

// DetectDrives finds all mounted USB drives except Macintosh HD
func (dm *DriveManager) DetectDrives() ([]USBDrive, error) {
	entries, err := os.ReadDir(dm.volumesPath)
	if err != nil {
		return nil, err
	}

	var drives []USBDrive
	for _, entry := range entries {
		if entry.Name() == "Macintosh HD" {
			continue
		}

		mountPath := filepath.Join(dm.volumesPath, entry.Name())
		if isReadableDrive(mountPath) {
			drives = append(drives, USBDrive{
				Name:      entry.Name(),
				MountPath: mountPath,
				Folder:    "podcasts",
			})
		}
	}

	return drives, nil
}

type PodcastScanner struct {
	template DirectoryTemplate
}

// NewPodcastScanner creates a new PodcastScanner instance
func NewPodcastScanner(template DirectoryTemplate) *PodcastScanner {
	if template == (DirectoryTemplate{}) {
		template = defaultDirTemplate
	}
	return &PodcastScanner{template: template}
}

// ScanDrive scans a drive for podcasts and returns matched episodes
func (ps *PodcastScanner) ScanDrive(drive USBDrive, podcastsBySize map[int64][]*PodcastEpisode) ([]PodcastEpisode, error) {
	podcastsChan := make(chan PodcastEpisode)
	errorsChan := make(chan error, 1)

	go func() {
		defer close(podcastsChan)
		defer close(errorsChan)
		if err := ps.scanDirectory(drive, podcastsChan); err != nil {
			errorsChan <- err
		}
	}()

	var episodes []PodcastEpisode
	matcher := NewPodcastMatcher(podcastsBySize)

	for podcast := range podcastsChan {
		if err := matcher.Match(&podcast); err != nil {
			continue
		}
		episodes = append(episodes, podcast)
	}

	select {
	case err := <-errorsChan:
		if err != nil {
			return nil, err
		}
	default:
	}

	return episodes, nil
}

type PodcastSync struct {
	tm *TransferManager
}

// NewPodcastSync creates a new PodcastSync instance
func NewPodcastSync() *PodcastSync {
	return &PodcastSync{}
}

// StartSync begins the podcast synchronization process
func (ps *PodcastSync) StartSync(episodes []PodcastEpisode, drive USBDrive, ch chan<- FileOp) *TransferManager {
	// Ensure FileSize is set for all episodes before calculating totalBytes
	updatedEpisodes, err := LoadLocalPodcasts(episodes)
	if err == nil {
		episodes = updatedEpisodes
	}

	// Validate and fix missing FileSizes
	for i, episode := range episodes {
		if episode.Selected && episode.FileSize == 0 {
			if filePath, err := convertFileURIToPath(episode.FilePath); err == nil {
				if stat, err := os.Stat(filePath); err == nil {
					episodes[i].FileSize = stat.Size()
				}
			}
		}
	}

	podcastDir := filepath.Join(drive.MountPath, drive.Folder)
	if err := os.MkdirAll(podcastDir, 0o755); err != nil {
		ch <- newFileOp(TransferProgress{}, false, err)
		close(ch)
		return nil
	}

	// Calculate actual totals based on files that need to be transferred
	actualTotalBytes, actualTotalFiles := ps.calculateActualTotals(episodes, podcastDir)

	// Send initial progress with actual totals
	progress := initializeProgress(actualTotalBytes, actualTotalFiles)
	ch <- newFileOp(progress, false, nil)

	// Stop any existing TransferManager before creating a new one
	// This ensures the old senderLoop goroutine is fully stopped
	if ps.tm != nil {
		ps.tm.Stop()
		ps.tm = nil
	}

	ps.tm = NewTransferManager(actualTotalBytes, actualTotalFiles, ch)
	go ps.syncEpisodes(episodes, podcastDir, ch)

	return ps.tm
}

// DeleteSelected removes selected episodes from the drive
func (ps *PodcastSync) DeleteSelected(episodes []PodcastEpisode) FileOp {
	visitedDirs := make(map[string]bool)
	var syncError error

	// Delete files
	for _, episode := range episodes {
		if !episode.Selected {
			continue
		}

		dir := filepath.Dir(episode.FilePath)
		visitedDirs[dir] = true

		if err := os.Remove(episode.FilePath); err != nil && syncError == nil {
			syncError = err
		}
	}

	// Clean up empty directories
	ps.cleanupEmptyDirs(visitedDirs, &syncError)

	return newFileOp(TransferProgress{}, true, syncError)
}

func isReadableDrive(path string) bool {
	_, err := os.ReadDir(path)
	return err == nil
}

func initializeProgress(totalBytes int64, totalFiles int) TransferProgress {
	return TransferProgress{
		TotalBytes: totalBytes,
		TotalFiles: totalFiles,
		StartTime:  time.Now(),
	}
}

func newFileOp(progress TransferProgress, complete bool, err error) FileOp {
	return FileOp{
		Progress: progress,
		Complete: complete,
		Error:    err,
	}
}

func (ps *PodcastScanner) scanDirectory(drive USBDrive, results chan<- PodcastEpisode) error {
	podcastDir := filepath.Join(drive.MountPath, drive.Folder)
	if _, err := os.Stat(podcastDir); os.IsNotExist(err) {
		return nil
	}

	return filepath.Walk(podcastDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !isAudioFile(path) {
			return err
		}

		episode, err := parseEpisodeFromPath(path, ps.template)
		if err != nil {
			return err
		}

		episode.FileSize = info.Size()
		results <- episode
		return nil
	})
}

func (ps *PodcastSync) syncEpisodes(episodes []PodcastEpisode, podcastDir string, ch chan<- FileOp) {
	// Capture the current TransferManager in a local variable
	// This prevents issues if ps.tm is overwritten by a new StartSync() call
	tm := ps.tm

	defer func() {
		// Stop the TransferManager first to shut down ProgressWriter
		if tm != nil {
			tm.Stop()
		}
		// Now safe to close the channel
		safeClose(ch)
	}()

	for _, episode := range episodes {
		if tm != nil && tm.IsStopped() {
			break
		}

		if !episode.Selected {
			continue
		}

		if err := ps.syncEpisode(episode, podcastDir); err != nil {
			safeSend(ch, newFileOp(TransferProgress{}, false, err))
			return
		}
	}

	safeSend(ch, newFileOp(*ps.tm.progress, true, nil))
}

func (ps *PodcastSync) syncEpisode(episode PodcastEpisode, podcastDir string) error {
	filePath, err := convertFileURIToPath(episode.FilePath)
	if err != nil {
		return err
	}

	showDir := filepath.Join(podcastDir, sanitizeName(episode.ShowName))
	if err := os.MkdirAll(showDir, 0o755); err != nil {
		return err
	}

	destPath := filepath.Join(showDir, formatEpisodeName(episode))
	if exists, _ := fileExists(destPath); exists {
		// File exists - skip it entirely since it's not counted in totals
		return nil
	}

	return ps.copyEpisode(episode, filePath, destPath)
}

func (ps *PodcastSync) copyEpisode(episode PodcastEpisode, srcPath, destPath string) error {
	ps.tm.StartFile(episode.ZTitle)

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy with periodic syncs for progress visibility
	// Using MultiWriter for atomic writes to both file and progress tracker
	const bufSize = 256 * 1024            // 256KB buffer
	const syncInterval = 16 * 1024 * 1024 // Sync every 16MB for better performance

	buf := make([]byte, bufSize)
	writer := io.MultiWriter(destFile, ps.tm)

	var bytesWrittenSinceSync int64

	for {
		nr, er := srcFile.Read(buf)
		if nr > 0 {
			nw, ew := writer.Write(buf[0:nr])
			if ew != nil {
				if ps.tm.IsStopped() {
					ps.cleanup(destPath, filepath.Dir(destPath))
					return nil
				}
				return ew
			}
			if nr != nw {
				return io.ErrShortWrite
			}

			bytesWrittenSinceSync += int64(nw)

			// Sync periodically to ensure progress is visible on slow USB drives
			// Less frequent syncs (16MB) reduce blocking I/O overhead
			if bytesWrittenSinceSync >= syncInterval {
				if err := destFile.Sync(); err != nil {
					return err
				}
				bytesWrittenSinceSync = 0
			}
		}
		if er != nil {
			if er != io.EOF {
				if ps.tm.IsStopped() {
					ps.cleanup(destPath, filepath.Dir(destPath))
					return nil
				}
				return er
			}
			break
		}
	}

	// Final sync to ensure all data is written
	if err := destFile.Sync(); err != nil {
		return err
	}

	// Mark file as completed
	ps.tm.CompleteFile(episode.FileSize)

	// Add ID3 tags with metadata from Apple Podcasts (best-effort)
	// This won't fail the sync if tagging fails
	_ = AddID3Tags(destPath, episode)

	return nil
}

func (ps *PodcastSync) cleanup(filePath, dirPath string) {
	_ = os.Remove(filePath)
	if empty, _ := isDirEmpty(dirPath); empty {
		_ = os.Remove(dirPath)
	}
}

func (ps *PodcastSync) cleanupEmptyDirs(dirs map[string]bool, syncError *error) {
	for dir := range dirs {
		if empty, err := isDirEmpty(dir); err == nil && empty {
			if dirErr := os.Remove(dir); dirErr != nil {
				if !os.IsNotExist(dirErr) && *syncError == nil {
					*syncError = dirErr
				}
			}
		}
	}
}

// calculateActualTotals checks which files need to be transferred and returns actual totals
func (ps *PodcastSync) calculateActualTotals(episodes []PodcastEpisode, podcastDir string) (int64, int) {
	var totalBytes int64
	var totalFiles int

	for _, episode := range episodes {
		if !episode.Selected {
			continue
		}

		showDir := filepath.Join(podcastDir, sanitizeName(episode.ShowName))
		destPath := filepath.Join(showDir, formatEpisodeName(episode))

		// Only count files that don't already exist
		if exists, _ := fileExists(destPath); !exists {
			totalBytes += episode.FileSize
			totalFiles++
		}
	}

	return totalBytes, totalFiles
}
