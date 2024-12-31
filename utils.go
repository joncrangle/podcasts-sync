package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

type errMsg struct {
	err error
}

func (e errMsg) Error() string {
	return e.err.Error()
}

type ComparisonResult struct {
	SizeDiff    int64
	HashMatches bool
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatDuration(duration time.Duration) string {
	hours := int(duration.Hours())
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60
	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func sanitizeName(name string) string {
	// Replace invalid characters with safe alternatives
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "",
		"?", "",
		"\"", "'",
		"<", "",
		">", "",
		"|", "-",
		"&", "and",
	)

	// Remove or replace any other problematic characters
	name = replacer.Replace(name)
	name = strings.TrimSpace(name)

	// Ensure name isn't too long for filesystem
	if len(name) > 255 {
		name = name[:255]
	}

	return name
}

func comparePodcasts(local, drive PodcastEpisode) ComparisonResult {
	return ComparisonResult{
		SizeDiff:    drive.FileSize - local.FileSize,
		HashMatches: local.MD5Hash == drive.MD5Hash,
	}
}

func formatEpisodeName(template DirectoryTemplate, episode PodcastEpisode) string {
	name := template.EpisodeFormat

	name = strings.ReplaceAll(name, "{title}", episode.ZTitle)
	name = strings.ReplaceAll(name, "{date}", episode.Published.Format(template.DateFormat))
	name = strings.ReplaceAll(name, "{show}", episode.ShowName)

	if template.SanitizeNames {
		name = sanitizeName(name)
	}

	// Ensure proper extension
	ext := filepath.Ext(episode.FilePath)
	if !strings.HasSuffix(name, ext) {
		name += ext
	}

	return name
}
