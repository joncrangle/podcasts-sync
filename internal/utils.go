package internal

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// FormatBytes returns a human-readable representation of a byte count
func FormatBytes(bytes int64) string {
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

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// isSystemHiddenFile checks if a file is a macOS/system hidden file that should be ignored
func isSystemHiddenFile(name string) bool {
	hiddenFiles := []string{
		".DS_Store",
		".Spotlight-V100",
		".Trashes",
		".fseventsd",
		".TemporaryItems",
		".VolumeIcon.icns",
		".com.apple.timemachine.donotpresent",
		".DocumentRevisions-V100",
		".PKInstallSandboxManager",
	}

	for _, hidden := range hiddenFiles {
		if name == hidden {
			return true
		}
	}

	// Also check for hidden temp files
	return strings.HasPrefix(name, "._")
}

// cleanupSystemHiddenFiles removes system hidden files from a directory
func cleanupSystemHiddenFiles(dirPath string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if isSystemHiddenFile(entry.Name()) {
			filePath := filepath.Join(dirPath, entry.Name())
			_ = os.Remove(filePath) // Best effort - ignore errors
		}
	}
}

func isDirEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, err
	}

	// Count non-hidden files
	visibleCount := 0
	for _, entry := range entries {
		if !isSystemHiddenFile(entry.Name()) {
			visibleCount++
		}
	}

	return visibleCount == 0, nil
}

// USBDrivesEqual compare two slices of USB drive names for equality
func USBDrivesEqual(a, b []USBDrive) bool {
	if len(a) != len(b) {
		return false
	}

	sort.Slice(a, func(i, j int) bool {
		return a[i].MountPath < a[j].MountPath
	})
	sort.Slice(b, func(i, j int) bool {
		return b[i].MountPath < b[j].MountPath
	})

	for i := range a {
		if a[i].MountPath != b[i].MountPath {
			return false
		}
	}
	return true
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

func formatEpisodeName(episode PodcastEpisode) string {
	template := defaultDirTemplate
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

// Returns the SHA256 checksum of a file
func getChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

var audioExtensions = map[string]bool{
	".mp3":  true,
	".m4a":  true,
	".wav":  true,
	".aac":  true,
	".ogg":  true,
	".flac": true,
}

// Check if a file is an audio file based on its extension
func isAudioFile(path string) bool {
	filename := filepath.Base(path)
	if strings.HasPrefix(filename, ".") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	return audioExtensions[ext]
}

// Convert file URI to a file path manually
func convertFileURIToPath(fileURI string) (string, error) {
	parsedURL, err := url.Parse(fileURI)
	if err != nil {
		return "", err
	}

	if parsedURL.Scheme != "file" {
		return "", fmt.Errorf("unsupported URI scheme: %s", parsedURL.Scheme)
	}

	// Decode the path to handle escaped characters
	return url.PathUnescape(parsedURL.Path)
}

// Parse episode metadata from a file path based on a template
func parseEpisodeFromPath(path string, template DirectoryTemplate) (PodcastEpisode, error) {
	episode := PodcastEpisode{
		FilePath: path,
	}

	// Extract show name from parent directory
	dir := filepath.Dir(path)
	episode.ShowName = filepath.Base(dir)

	// Get filename without extension
	filename := filepath.Base(path)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	// Convert date format to regex pattern
	dateRegex := dateFormatToRegex(template.DateFormat)

	// Create regex pattern from template
	pattern := template.EpisodeFormat
	pattern = regexp.QuoteMeta(pattern)

	// Replace template placeholders with capture groups
	pattern = strings.ReplaceAll(pattern, regexp.QuoteMeta("{date}"), fmt.Sprintf("(%s)", dateRegex))
	pattern = strings.ReplaceAll(pattern, regexp.QuoteMeta("{title}"), `(.+)`)
	pattern = strings.ReplaceAll(pattern, regexp.QuoteMeta("{show}"), `.+`)

	re, err := regexp.Compile(`^` + pattern + `$`)
	if err != nil {
		episode.ZTitle = nameWithoutExt
		return episode, nil
	}

	matches := re.FindStringSubmatch(nameWithoutExt)
	if matches == nil {
		episode.ZTitle = nameWithoutExt
		return episode, nil
	}

	// Find positions of placeholders in template
	placeholderPos := getPlaceholderPositions(template.EpisodeFormat)

	// Extract date and title from matches based on their positions
	for i, match := range matches[1:] {
		pos := i + 1 // account for full match at index 0
		switch pos {
		case placeholderPos["date"]:
			parsed, err := time.Parse(template.DateFormat, match)
			if err != nil {
				return episode, fmt.Errorf("failed to parse date: %w", err)
			}
			episode.Published = parsed
		case placeholderPos["title"]:
			episode.ZTitle = match
			if template.SanitizeNames {
				episode.ZTitle = strings.ReplaceAll(episode.ZTitle, "-", " ")
			}
		}
	}

	return episode, nil
}

// Convert Go time format to regex pattern
func dateFormatToRegex(format string) string {
	// Map of Go date format characters to regex patterns
	datePatterns := map[string]string{
		"2006":    `\d{4}`,
		"06":      `\d{2}`,
		"01":      `\d{2}`,
		"1":       `\d{1,2}`,
		"02":      `\d{2}`,
		"2":       `\d{1,2}`,
		"15":      `\d{2}`,
		"3":       `\d{1,2}`,
		"04":      `\d{2}`,
		"4":       `\d{1,2}`,
		"05":      `\d{2}`,
		"5":       `\d{1,2}`,
		"PM":      `[AP]M`,
		"pm":      `[ap]m`,
		"Monday":  `[A-Za-z]+`,
		"Mon":     `[A-Za-z]+`,
		"January": `[A-Za-z]+`,
		"Jan":     `[A-Za-z]+`,
		"_2":      `\s?\d{1,2}`,
		"_02":     `\s?\d{2}`,
	}

	// Escape the format string for regex
	regex := regexp.QuoteMeta(format)

	// Replace each date pattern with its regex equivalent
	// Sort keys by length in descending order to handle overlapping patterns
	patterns := make([]string, 0, len(datePatterns))
	for k := range datePatterns {
		patterns = append(patterns, k)
	}
	sort.Slice(patterns, func(i, j int) bool {
		return len(patterns[i]) > len(patterns[j])
	})

	for _, pattern := range patterns {
		regex = strings.ReplaceAll(regex, pattern, datePatterns[pattern])
	}

	return regex
}

// Get positions of placeholders in template
func getPlaceholderPositions(template string) map[string]int {
	positions := make(map[string]int)
	placeholders := []string{"date", "title", "show"}

	pos := 1 // Start at 1 since regex matches have full match at index 0
	for _, placeholder := range placeholders {
		if strings.Contains(template, "{"+placeholder+"}") {
			positions[placeholder] = pos
			pos++
		}
	}

	return positions
}
