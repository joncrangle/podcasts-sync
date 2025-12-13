package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bogem/id3v2/v2"
)

// CleanupID3TempFiles removes any orphaned temporary files created by the id3v2 library.
// The library creates temporary files named "{originalFile}-id3v2" during the save process.
// If the atomic rename fails (common on USB drives with FAT32), these temp files remain.
func CleanupID3TempFiles(filePath string) error {
	// Check for the standard temp file pattern used by id3v2 library
	tempFile := filePath + "-id3v2"
	if _, err := os.Stat(tempFile); err == nil {
		if removeErr := os.Remove(tempFile); removeErr != nil {
			return fmt.Errorf("failed to remove temp file %s: %w", tempFile, removeErr)
		}
	}

	// Also check for any .id3 files (less common but possible)
	id3File := filePath + ".id3"
	if _, err := os.Stat(id3File); err == nil {
		if removeErr := os.Remove(id3File); removeErr != nil {
			return fmt.Errorf("failed to remove .id3 file %s: %w", id3File, removeErr)
		}
	}

	return nil
}

// VerifyNoTempFiles checks if any temporary ID3 files exist and returns an error if found.
func VerifyNoTempFiles(filePath string) error {
	tempFile := filePath + "-id3v2"
	if _, err := os.Stat(tempFile); err == nil {
		return fmt.Errorf("temp file still exists: %s", tempFile)
	}

	id3File := filePath + ".id3"
	if _, err := os.Stat(id3File); err == nil {
		return fmt.Errorf(".id3 file still exists: %s", id3File)
	}

	return nil
}

// AddID3Tags adds metadata from the Apple Podcasts database to an audio file.
// This is best-effort; errors are returned but should not fail the sync operation.
//
// The function implements several safeguards to prevent duplicate files:
// 1. Cleans up any existing temp files before starting
// 2. Sets ID3v2.3 for maximum compatibility with older players
// 3. Verifies no temp files remain after save
// 4. Retries once on failure with cleanup
func AddID3Tags(filePath string, episode PodcastEpisode) error {
	// Only process MP3 files (ID3 tags are MP3-specific)
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".mp3" {
		return nil // Not an error, just not applicable
	}

	// Pre-check: Clean up any existing temp files from previous failed attempts
	_ = CleanupID3TempFiles(filePath)

	// Attempt to add tags with retry logic
	err := addID3TagsOnce(filePath, episode)
	if err != nil {
		// Retry once after cleanup and brief delay
		// This handles transient filesystem issues on USB drives
		time.Sleep(100 * time.Millisecond)
		_ = CleanupID3TempFiles(filePath)
		err = addID3TagsOnce(filePath, episode)
	}

	// Post-check: Verify no temp files remain regardless of success/failure
	// If temp files exist, attempt cleanup and return error
	if verifyErr := VerifyNoTempFiles(filePath); verifyErr != nil {
		cleanupErr := CleanupID3TempFiles(filePath)
		if cleanupErr != nil {
			return fmt.Errorf("temp files remain after tagging and cleanup failed: %w (original error: %v)", cleanupErr, err)
		}
		// If cleanup succeeded but we had an error, return the original error
		if err != nil {
			return err
		}
	}

	return err
}

// addID3TagsOnce performs a single attempt at adding ID3 tags to a file.
func addID3TagsOnce(filePath string, episode PodcastEpisode) error {
	// Open the file for tag editing
	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("failed to open file for tagging: %w", err)
	}
	defer tag.Close()

	// Set ID3v2.3 for maximum compatibility with older car/portable MP3 players
	// v2.3 is more widely supported than v2.4
	tag.SetVersion(3)

	// Set title (episode name)
	if episode.ZTitle != "" {
		tag.SetTitle(episode.ZTitle)
	}

	// Set artist and album to show name
	if episode.ShowName != "" {
		tag.SetArtist(episode.ShowName)
		tag.SetAlbum(episode.ShowName)
	}

	// Set genre to Podcast
	tag.SetGenre("Podcast")

	// Set year from publish date
	if !episode.Published.IsZero() {
		tag.SetYear(episode.Published.Format("2006"))
	}

	// Set comment with publish date in readable format
	if !episode.Published.IsZero() {
		comment := id3v2.CommentFrame{
			Encoding:    id3v2.EncodingUTF8,
			Language:    "eng",
			Description: "Published",
			Text:        episode.Published.Format("2006-01-02"),
		}
		tag.AddCommentFrame(comment)
	}

	// Save the tags
	// Note: The id3v2 library creates a temp file (filePath + "-id3v2"),
	// writes the new tag + music data to it, then atomically renames it.
	// On some filesystems (especially FAT32 USB drives), the rename can fail,
	// leaving both the original and temp file. Our cleanup logic handles this.
	if err := tag.Save(); err != nil {
		return fmt.Errorf("failed to save tags: %w", err)
	}

	return nil
}
