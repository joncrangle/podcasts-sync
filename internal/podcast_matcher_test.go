package internal

import (
	"path/filepath"
	"testing"
	"time"
)

func TestBuildExpectedDrivePath(t *testing.T) {
	tests := []struct {
		name     string
		episode  *PodcastEpisode
		expected string
	}{
		{
			name: "basic episode",
			episode: &PodcastEpisode{
				ZTitle:    "Episode Title",
				ShowName:  "Podcast Show",
				Published: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
				FilePath:  "/path/to/file.mp3",
			},
			expected: filepath.Join("Podcast Show", "2024-01-15 - Episode Title.mp3"),
		},
		{
			name: "episode with special characters",
			episode: &PodcastEpisode{
				ZTitle:    "Episode: Test/File?",
				ShowName:  "Show & Name",
				Published: time.Date(2024, 3, 20, 0, 0, 0, 0, time.UTC),
				FilePath:  "/path/to/file.mp3",
			},
			expected: filepath.Join("Show and Name", "2024-03-20 - Episode- Test-File.mp3"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildExpectedDrivePath(tt.episode)
			if result != tt.expected {
				t.Errorf("buildExpectedDrivePath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCanonicalizePathForMatching(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "full path with multiple components",
			path:     "/Volumes/Drive/Podcasts/Show Name/2024-01-15 - Episode.mp3",
			expected: filepath.Join("Show Name", "2024-01-15 - Episode.mp3"),
		},
		{
			name:     "relative path with show and episode",
			path:     "Show Name/2024-01-15 - Episode.mp3",
			expected: filepath.Join("Show Name", "2024-01-15 - Episode.mp3"),
		},
		{
			name:     "single component path",
			path:     "episode.mp3",
			expected: "episode.mp3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := canonicalizePathForMatching(tt.path)
			if result != tt.expected {
				t.Errorf("canonicalizePathForMatching() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMatchByPath(t *testing.T) {
	// Create local episodes
	localEpisode1 := &PodcastEpisode{
		ZTitle:    "Episode 1",
		ShowName:  "Test Show",
		Published: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		FilePath:  "/local/Test Show/2024-01-15 - Episode 1.mp3",
		FileSize:  1000,
	}

	localEpisode2 := &PodcastEpisode{
		ZTitle:    "Episode 2",
		ShowName:  "Test Show",
		Published: time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
		FilePath:  "/local/Test Show/2024-01-16 - Episode 2.mp3",
		FileSize:  2000,
	}

	podcastsBySize := map[int64][]*PodcastEpisode{
		1000: {localEpisode1},
		2000: {localEpisode2},
	}

	matcher := NewPodcastMatcher(podcastsBySize)

	tests := []struct {
		name          string
		drivePodcast  *PodcastEpisode
		expectMatch   bool
		expectedTitle string
	}{
		{
			name: "exact path match",
			drivePodcast: &PodcastEpisode{
				FilePath: "/Volumes/Drive/Test Show/2024-01-15 - Episode 1.mp3",
				FileSize: 999, // Different size to test path-based matching
			},
			expectMatch:   true,
			expectedTitle: "Episode 1",
		},
		{
			name: "no path match",
			drivePodcast: &PodcastEpisode{
				FilePath: "/Volumes/Drive/Other Show/2024-01-15 - Different Episode.mp3",
				FileSize: 1000,
			},
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matcher.matchByPath(tt.drivePodcast)
			if result != tt.expectMatch {
				t.Errorf("matchByPath() = %v, want %v", result, tt.expectMatch)
			}
			if tt.expectMatch && tt.drivePodcast.ZTitle != tt.expectedTitle {
				t.Errorf("matched episode title = %v, want %v", tt.drivePodcast.ZTitle, tt.expectedTitle)
			}
		})
	}
}

func TestMatchByDuration(t *testing.T) {
	tests := []struct {
		name         string
		drivePodcast *PodcastEpisode
		matches      []*PodcastEpisode
		expectMatch  bool
	}{
		{
			name: "exact duration match",
			drivePodcast: &PodcastEpisode{
				Duration: 1800, // 30 minutes
			},
			matches: []*PodcastEpisode{
				{Duration: 1800, ZTitle: "Match 1"},
				{Duration: 2400, ZTitle: "Match 2"},
			},
			expectMatch: true,
		},
		{
			name: "duration within tolerance (2%)",
			drivePodcast: &PodcastEpisode{
				Duration: 1800, // 30 minutes
			},
			matches: []*PodcastEpisode{
				{Duration: 1820, ZTitle: "Match 1"}, // ~1% difference
				{Duration: 2400, ZTitle: "Match 2"},
			},
			expectMatch: true,
		},
		{
			name: "duration outside tolerance",
			drivePodcast: &PodcastEpisode{
				Duration: 1800, // 30 minutes
			},
			matches: []*PodcastEpisode{
				{Duration: 2000, ZTitle: "Match 1"}, // >10% difference
				{Duration: 2400, ZTitle: "Match 2"},
			},
			expectMatch: false,
		},
		{
			name: "drive podcast has no duration",
			drivePodcast: &PodcastEpisode{
				Duration: 0,
			},
			matches: []*PodcastEpisode{
				{Duration: 1800, ZTitle: "Match 1"},
			},
			expectMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matcher := &PodcastMatcher{}
			result := matcher.matchByDuration(tt.drivePodcast, tt.matches)
			if result != tt.expectMatch {
				t.Errorf("matchByDuration() = %v, want %v", result, tt.expectMatch)
			}
		})
	}
}

func TestMatch_CascadingStrategy(t *testing.T) {
	// Setup local episodes
	localEpisode1 := &PodcastEpisode{
		ZTitle:    "Episode 1",
		ShowName:  "Test Show",
		Published: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		FilePath:  "/local/Test Show/2024-01-15 - Episode 1.mp3",
		FileSize:  1000,
		Duration:  1800,
	}

	localEpisode2 := &PodcastEpisode{
		ZTitle:    "Episode 2",
		ShowName:  "Test Show",
		Published: time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
		FilePath:  "/local/Test Show/2024-01-16 - Episode 2.mp3",
		FileSize:  1000, // Same size as episode1
		Duration:  2400,
	}

	podcastsBySize := map[int64][]*PodcastEpisode{
		1000: {localEpisode1, localEpisode2},
	}

	matcher := NewPodcastMatcher(podcastsBySize)

	tests := []struct {
		name          string
		drivePodcast  *PodcastEpisode
		expectMatch   bool
		expectedTitle string
	}{
		{
			name: "path-based match (priority 1)",
			drivePodcast: &PodcastEpisode{
				FilePath: "/Volumes/Drive/Test Show/2024-01-15 - Episode 1.mp3",
				FileSize: 999, // Different size
				Duration: 0,   // No duration
			},
			expectMatch:   true,
			expectedTitle: "Episode 1",
		},
		{
			name: "duration-based match (priority 3, when size matches multiple)",
			drivePodcast: &PodcastEpisode{
				FilePath: "/Volumes/Drive/Different/Path.mp3",
				FileSize: 1000, // Matches both episodes
				Duration: 2400, // Matches episode2
			},
			expectMatch:   true,
			expectedTitle: "Episode 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := matcher.Match(tt.drivePodcast)
			if err != nil {
				t.Fatalf("Match() error = %v", err)
			}
			if tt.drivePodcast.OnDrive != tt.expectMatch {
				t.Errorf("Match() OnDrive = %v, want %v", tt.drivePodcast.OnDrive, tt.expectMatch)
			}
			if tt.expectMatch && tt.drivePodcast.ZTitle != tt.expectedTitle {
				t.Errorf("matched episode title = %v, want %v", tt.drivePodcast.ZTitle, tt.expectedTitle)
			}
		})
	}
}

func TestNewPodcastMatcher(t *testing.T) {
	episode1 := &PodcastEpisode{
		ZTitle:    "Episode 1",
		ShowName:  "Test Show",
		Published: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		FilePath:  "/local/file1.mp3",
		FileSize:  1000,
	}

	episode2 := &PodcastEpisode{
		ZTitle:    "Episode 2",
		ShowName:  "Test Show",
		Published: time.Date(2024, 1, 16, 0, 0, 0, 0, time.UTC),
		FilePath:  "/local/file2.mp3",
		FileSize:  2000,
	}

	podcastsBySize := map[int64][]*PodcastEpisode{
		1000: {episode1},
		2000: {episode2},
	}

	matcher := NewPodcastMatcher(podcastsBySize)

	// Verify the matcher was created with correct data
	if matcher.podcastsBySize == nil {
		t.Error("podcastsBySize is nil")
	}
	if matcher.podcastsByPath == nil {
		t.Error("podcastsByPath is nil")
	}

	// Verify path index was built
	if len(matcher.podcastsByPath) != 2 {
		t.Errorf("podcastsByPath length = %d, want 2", len(matcher.podcastsByPath))
	}
}
