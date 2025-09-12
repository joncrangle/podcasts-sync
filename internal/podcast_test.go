package internal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPodcastEpisode_Title(t *testing.T) {
	tests := []struct {
		name     string
		episode  PodcastEpisode
		expected string
	}{
		{
			name: "episode not on drive",
			episode: PodcastEpisode{
				ZTitle:  "Test Episode",
				OnDrive: false,
			},
			expected: "Test Episode",
		},
		{
			name: "episode on drive",
			episode: PodcastEpisode{
				ZTitle:  "Test Episode",
				OnDrive: true,
			},
			expected: "✓ Test Episode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.episode.Title()
			if got != tt.expected {
				t.Errorf("Title() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPodcastEpisode_Description(t *testing.T) {
	publishTime := time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC)
	duration := 30 * time.Minute

	tests := []struct {
		name     string
		episode  PodcastEpisode
		expected string
	}{
		{
			name: "basic episode with show name only",
			episode: PodcastEpisode{
				ShowName: "Test Show",
			},
			expected: "Test Show",
		},
		{
			name: "episode with show name and publish date",
			episode: PodcastEpisode{
				ShowName:  "Test Show",
				Published: publishTime,
			},
			expected: "Test Show • 2023-12-25",
		},
		{
			name: "episode with show name, publish date, and duration",
			episode: PodcastEpisode{
				ShowName:  "Test Show",
				Published: publishTime,
				Duration:  duration,
			},
			expected: "Test Show • 2023-12-25 • 30:00",
		},
		{
			name: "episode with show name and duration only",
			episode: PodcastEpisode{
				ShowName: "Test Show",
				Duration: duration,
			},
			expected: "Test Show • 30:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.episode.Description()
			if got != tt.expected {
				t.Errorf("Description() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPodcastEpisode_FilterValue(t *testing.T) {
	episode := PodcastEpisode{
		ZTitle: "Test Episode",
	}

	expected := "Test Episode"
	got := episode.FilterValue()

	if got != expected {
		t.Errorf("FilterValue() = %v, want %v", got, expected)
	}
}

func TestLoadLocalPodcasts(t *testing.T) {
	// Create a temporary file for testing
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "test.mp3")

	content := []byte("test podcast content")
	err := os.WriteFile(tempFile, content, 0o644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create test episodes
	episodes := []PodcastEpisode{
		{
			ZTitle:   "Test Episode 1",
			ShowName: "Test Show",
			FilePath: "file://" + tempFile,
		},
		{
			ZTitle:   "Non-existent Episode",
			ShowName: "Test Show",
			FilePath: "file:///non/existent/path.mp3",
		},
	}

	result, err := LoadLocalPodcasts(episodes)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check that the existing file has its size set
	if result[0].FileSize != int64(len(content)) {
		t.Errorf("Expected file size %d, got %d", len(content), result[0].FileSize)
	}

	// Check that the non-existent file has size 0
	if result[1].FileSize != 0 {
		t.Errorf("Expected file size 0 for non-existent file, got %d", result[1].FileSize)
	}
}

func TestConvertFileURIToPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		hasError bool
	}{
		{
			name:     "valid file URI",
			input:    "file:///Users/test/Music/podcast.mp3",
			expected: "/Users/test/Music/podcast.mp3",
			hasError: false,
		},
		{
			name:     "file URI with encoded spaces",
			input:    "file:///Users/test/My%20Music/podcast.mp3",
			expected: "/Users/test/My Music/podcast.mp3",
			hasError: false,
		},
		{
			name:     "non-file URI",
			input:    "http://example.com/podcast.mp3",
			expected: "",
			hasError: true,
		},
		{
			name:     "invalid URI",
			input:    "not-a-uri",
			expected: "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertFileURIToPath(tt.input)

			if tt.hasError && err == nil {
				t.Error("Expected error, got nil")
			}

			if !tt.hasError && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if got != tt.expected {
				t.Errorf("convertFileURIToPath() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected string
	}{
		{
			name:     "30 minutes",
			duration: 30 * time.Minute,
			expected: "30:00",
		},
		{
			name:     "1 hour 15 minutes",
			duration: 1*time.Hour + 15*time.Minute,
			expected: "01:15:00",
		},
		{
			name:     "45 seconds",
			duration: 45 * time.Second,
			expected: "00:45",
		},
		{
			name:     "2 hours 30 minutes 45 seconds",
			duration: 2*time.Hour + 30*time.Minute + 45*time.Second,
			expected: "02:30:45",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatDuration(tt.duration)
			if got != tt.expected {
				t.Errorf("formatDuration() = %v, want %v", got, tt.expected)
			}
		})
	}
}
