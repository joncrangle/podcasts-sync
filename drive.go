package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type USBDrive struct {
	Name      string
	MountPath string
}

type driveUpdatedMsg []USBDrive

func (p USBDrive) Title() string       { return p.Name }
func (p USBDrive) Description() string { return p.MountPath }
func (p USBDrive) FilterValue() string { return p.Name }

type DirectoryTemplate struct {
	ShowNameFormat string // e.g., "{show}"
	EpisodeFormat  string // e.g., "{date} - {title}"
	DateFormat     string // e.g., "2006-01-02"
	SanitizeNames  bool
	CreateIndex    bool
}

type DirectoryHealth struct {
	EmptyDirs       []string
	MalformedPaths  []string
	OrphanedFiles   []string
	Permissions     []string
	Recommendations []string
}

var defaultDirTemplate = DirectoryTemplate{
	ShowNameFormat: "{show}",
	EpisodeFormat:  "{date} - {title}",
	DateFormat:     "2006-01-02",
	SanitizeNames:  true,
}

func detectDrives() tea.Msg {
	volumesDir := "/Volumes"
	entries, err := os.ReadDir(volumesDir)
	if err != nil {
		return errMsg{err}
	}

	var drives []USBDrive
	for _, entry := range entries {
		if entry.Name() == "Macintosh HD" {
			continue
		}

		drives = append(drives, USBDrive{
			Name:      entry.Name(),
			MountPath: filepath.Join(volumesDir, entry.Name()),
		})
	}

	return driveUpdatedMsg(drives)
}

func validateDirectoryStructure(drive USBDrive) (*DirectoryHealth, error) {
	health := &DirectoryHealth{}
	podcastDir := filepath.Join(drive.MountPath, "podcasts")

	err := filepath.Walk(podcastDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(podcastDir, path)
		if err != nil {
			return err
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		if info.IsDir() {
			// Check for empty directories
			entries, err := os.ReadDir(path)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				health.EmptyDirs = append(health.EmptyDirs, relPath)
			}

			// Check directory name format
			// dirName := filepath.Base(path)
			//TODO: implement validateDirNameFormat
			// if !validateDirNameFormat(dirName) {
			// 	health.MalformedPaths = append(health.MalformedPaths, relPath)
			// }
		} else {
			// Check file placement and naming
			parentDir := filepath.Dir(path)
			if parentDir == podcastDir {
				health.OrphanedFiles = append(health.OrphanedFiles, relPath)
			}

			// Check file permissions
			if info.Mode().Perm()&0222 == 0 {
				health.Permissions = append(health.Permissions, relPath)
			}
		}

		return nil
	})

	return health, err
}

func setupPodcastDirectory(drive USBDrive) error {
	podcastDir := filepath.Join(drive.MountPath, "podcasts")

	// Check if directory exists
	info, err := os.Stat(podcastDir)
	if os.IsNotExist(err) {
		// Create directory if it doesn't exist
		if err := os.MkdirAll(podcastDir, 0755); err != nil {
			return fmt.Errorf("failed to create podcasts directory: %w", err)
		}
		return nil
	} else if err != nil {
		return fmt.Errorf("error checking podcasts directory: %w", err)
	}

	// Verify it's a directory if it exists
	if !info.IsDir() {
		return fmt.Errorf("'podcasts' exists but is not a directory")
	}

	return nil
}

func loadDrivePodcasts(drive USBDrive) tea.Cmd {
	return func() tea.Msg {
		podcastDir := filepath.Join(drive.MountPath, "podcasts")

		// Check if podcasts directory exists
		if _, err := os.Stat(podcastDir); os.IsNotExist(err) {
			// Directory doesn't exist yet, return empty list
			return []PodcastEpisode{}
		} else if err != nil {
			return errMsg{fmt.Errorf("error accessing podcasts directory: %w", err)}
		}

		var episodes []PodcastEpisode
		err := filepath.Walk(podcastDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && (filepath.Ext(path) == ".mp3" || filepath.Ext(path) == ".m4a") {
				relPath, err := filepath.Rel(podcastDir, path)
				if err != nil {
					return err
				}

				showName := filepath.Dir(relPath)
				if showName == "." {
					showName = "Unsorted"
				}

				episodes = append(episodes, PodcastEpisode{
					ZTitle:    filepath.Base(path),
					ShowName:  showName,
					FilePath:  path,
					Published: info.ModTime(),
					FileSize:  info.Size(),
					OnDrive:   true,
				})
			}
			return nil
		})

		if err != nil {
			return errMsg{fmt.Errorf("error scanning podcasts directory: %w", err)}
		}

		return episodes
	}
}

func startSync(episodes []list.Item, drive USBDrive) tea.Cmd {
	return func() tea.Msg {
		podcastDir := filepath.Join(drive.MountPath, "podcasts")
		if err := os.MkdirAll(podcastDir, 0755); err != nil {
			return fileOpCompleteMsg{"sync", err}
		}

		for _, item := range episodes {
			episode := item.(PodcastEpisode)
			if !episode.Selected {
				continue
			}

			// Create show directory
			showDir := filepath.Join(podcastDir, episode.ShowName)
			if err := os.MkdirAll(showDir, 0755); err != nil {
				return fileOpCompleteMsg{"sync", err}
			}

			// Copy file
			srcFile, err := os.Open(episode.FilePath)
			if err != nil {
				return fileOpCompleteMsg{"sync", err}
			}
			defer srcFile.Close()

			destPath := filepath.Join(showDir, filepath.Base(episode.FilePath))
			destFile, err := os.Create(destPath)
			if err != nil {
				return fileOpCompleteMsg{"sync", err}
			}
			defer destFile.Close()

			// Copy with progress updates
			buffer := make([]byte, 1024*1024) // 1MB buffer
			var copied int64
			for {
				n, err := srcFile.Read(buffer)
				if err != nil && err != io.EOF {
					return fileOpCompleteMsg{"sync", err}
				}
				if n == 0 {
					break
				}

				if _, err := destFile.Write(buffer[:n]); err != nil {
					return fileOpCompleteMsg{"sync", err}
				}

				copied += int64(n)
				progress := float64(copied) / float64(episode.FileSize)
				return fileOpMsg{"copy", progress, episode.ZTitle}
			}
		}

		return fileOpCompleteMsg{"sync", nil}
	}
}

func handleBatchSync(batch BatchOperation, drive USBDrive) tea.Msg {
	// Setup and validate directory structure
	if err := setupPodcastDirectory(drive); err != nil {
		return errMsg{err}
	}

	progress := TransferProgress{}

	// Proceed with sync operation
	for i, episode := range batch.Episodes {
		showDirName := sanitizeName(episode.ShowName)
		showDir := filepath.Join(drive.MountPath, "podcasts", showDirName)
		if err := os.MkdirAll(showDir, 0755); err != nil {
			return errMsg{err}
		}

		progress.CurrentFile = episode.ZTitle
		progress.FilesDone = i

		// Create directory structure
		destDir := filepath.Join(drive.MountPath, "podcasts", episode.ShowName)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return errMsg{err}
		}

		// Open source file
		src, err := os.Open(episode.FilePath)
		if err != nil {
			return errMsg{err}
		}
		defer src.Close()

		// Create destination file
		destPath := filepath.Join(destDir, filepath.Base(episode.FilePath))
		dest, err := os.Create(destPath)
		if err != nil {
			return errMsg{err}
		}
		defer dest.Close()

		// Copy with progress tracking
		buffer := make([]byte, 1024*1024) // 1MB buffer
		totalBytes := episode.FileSize
		progress := TransferProgress{
			CurrentFile: episode.ZTitle,
			StartTime:   time.Now(),
			TotalBytes:  totalBytes,
		}

		for {
			n, err := src.Read(buffer)
			if err == io.EOF {
				break
			}
			if err != nil {
				return errMsg{err}
			}

			if _, err := dest.Write(buffer[:n]); err != nil {
				return errMsg{err}
			}

			progress.BytesTransferred += int64(n)
			progress.CurrentProgress = float64(progress.BytesTransferred) / float64(totalBytes)

			// Calculate speed and time remaining
			elapsed := time.Since(progress.StartTime).Seconds()
			progress.Speed = float64(progress.BytesTransferred) / elapsed

			bytesRemaining := totalBytes - progress.BytesTransferred
			if progress.Speed > 0 {
				secsRemaining := float64(bytesRemaining) / progress.Speed
				progress.TimeRemaining = time.Duration(secsRemaining) * time.Second
			}

			// Send progress update
			return progressUpdateMsg{Progress: progress}
		}

		// Calculate and store MD5 hash for the transferred file
		if hash, err := calculateMD5(destPath); err == nil {
			batch.Episodes[i].MD5Hash = hash
		}
	}

	return fileOpCompleteMsg{"sync", nil}
}

func deleteSelected(episodes []list.Item, drive USBDrive) tea.Cmd {
	return func() tea.Msg {
		for _, item := range episodes {
			episode := item.(PodcastEpisode)
			if !episode.Selected {
				continue
			}

			if err := os.Remove(episode.FilePath); err != nil {
				return fileOpCompleteMsg{"delete", err}
			}

			return fileOpMsg{"delete", 1.0, episode.ZTitle}
		}

		return fileOpCompleteMsg{"delete", nil}
	}
}
