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

func (d USBDrive) Title() string       { return d.Name }
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

// Creates a new DriveManager instance
func NewDriveManager(volumesPath string, template DirectoryTemplate) *DriveManager {
	if template == (DirectoryTemplate{}) {
		template = defaultDirTemplate
	}
	return &DriveManager{
		volumesPath: volumesPath,
		template:    template,
	}
}

// Finds all mounted USB drives except Macintosh HD
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

// Creates a new PodcastScanner instance
func NewPodcastScanner(template DirectoryTemplate) *PodcastScanner {
	if template == (DirectoryTemplate{}) {
		template = defaultDirTemplate
	}
	return &PodcastScanner{template: template}
}

// Scans a drive for podcasts and returns matched episodes
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
	progress *ProgressWriter
}

// Creates a new PodcastSync instance
func NewPodcastSync() *PodcastSync {
	return &PodcastSync{}
}

// Begins the podcast synchronization process
func (ps *PodcastSync) StartSync(episodes []PodcastEpisode, drive USBDrive, ch chan<- FileOp) *ProgressWriter {
	totalBytes := calculateTotalBytes(episodes)
	progress := initializeProgress(totalBytes, len(episodes))

	// Send initial progress
	ch <- newFileOp(progress, false, nil)

	podcastDir := filepath.Join(drive.MountPath, drive.Folder)
	if err := os.MkdirAll(podcastDir, 0o755); err != nil {
		ch <- newFileOp(TransferProgress{}, false, err)
		close(ch)
		return nil
	}

	ps.progress = NewProgressWriter(totalBytes, &progress, ch)
	go ps.syncEpisodes(episodes, podcastDir, progress, ch)

	return ps.progress
}

// Removes selected episodes from the drive
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

func calculateTotalBytes(episodes []PodcastEpisode) int64 {
	var total int64
	for _, episode := range episodes {
		if episode.Selected {
			total += episode.FileSize
		}
	}
	return total
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

func (ps *PodcastSync) syncEpisodes(episodes []PodcastEpisode, podcastDir string, progress TransferProgress, ch chan<- FileOp) {
	defer safeClose(ch)

	for _, episode := range episodes {
		if ps.progress.IsStopped() {
			break
		}

		if !episode.Selected {
			continue
		}

		if err := ps.syncEpisode(episode, podcastDir, &progress); err != nil {
			safeSend(ch, newFileOp(TransferProgress{}, false, err))
			return
		}
	}

	safeSend(ch, newFileOp(progress, true, nil))
}

func (ps *PodcastSync) syncEpisode(episode PodcastEpisode, podcastDir string, progress *TransferProgress) error {
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
		progress.BytesTransferred += episode.FileSize
		progress.FilesDone++
		return nil
	}

	return ps.copyEpisode(episode, filePath, destPath)
}

func (ps *PodcastSync) copyEpisode(episode PodcastEpisode, srcPath, destPath string) error {
	ps.progress.progress.CurrentFile = episode.ZTitle

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

	if _, err := io.Copy(io.MultiWriter(destFile, ps.progress), srcFile); err != nil {
		if ps.progress.IsStopped() {
			ps.cleanup(destPath, filepath.Dir(destPath))
			return nil
		}
		return err
	}

	ps.progress.progress.FilesDone++
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
		if dirErr := os.Remove(dir); dirErr != nil {
			if !os.IsNotExist(dirErr) && *syncError == nil {
				continue
			}
		}
	}
}
