package internal

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bogem/id3v2/v2"
)

// AddID3Tags adds metadata from the Apple Podcasts database to an audio file.
// This is best-effort; errors are returned but should not fail the sync operation.
func AddID3Tags(filePath string, episode PodcastEpisode) error {
	// Only process MP3 files (ID3 tags are MP3-specific)
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".mp3" {
		return nil // Not an error, just not applicable
	}

	// Open the file for tag editing
	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
	if err != nil {
		return fmt.Errorf("failed to open file for tagging: %w", err)
	}
	defer tag.Close()

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
	if err := tag.Save(); err != nil {
		return fmt.Errorf("failed to save tags: %w", err)
	}

	return nil
}
