package internal

import (
	"path/filepath"
	"strings"
)

type PodcastMatcher struct {
	podcastsBySize map[int64][]*PodcastEpisode
	podcastsByPath map[string]*PodcastEpisode
}

// NewPodcastMatcher creates a new PodcastMatcher instance
func NewPodcastMatcher(podcastsBySize map[int64][]*PodcastEpisode) *PodcastMatcher {
	// Build path-based index from local episodes for fast path matching
	pathIndex := make(map[string]*PodcastEpisode)

	for _, episodes := range podcastsBySize {
		for _, ep := range episodes {
			// Create the expected drive path for this episode
			expectedPath := buildExpectedDrivePath(ep)
			pathIndex[expectedPath] = ep
		}
	}

	return &PodcastMatcher{
		podcastsBySize: podcastsBySize,
		podcastsByPath: pathIndex,
	}
}

// buildExpectedDrivePath constructs the expected drive path from episode metadata
func buildExpectedDrivePath(ep *PodcastEpisode) string {
	// Use the same formatting logic as when copying files
	showDir := sanitizeName(ep.ShowName)
	filename := formatEpisodeName(*ep)
	return filepath.Join(showDir, filename)
}

// canonicalizePathForMatching extracts the relative path from a full drive path
func canonicalizePathForMatching(fullPath string) string {
	// Extract the last two path components (show/episode)
	parts := strings.Split(filepath.ToSlash(fullPath), "/")
	if len(parts) >= 2 {
		return filepath.Join(parts[len(parts)-2], parts[len(parts)-1])
	}
	return filepath.Base(fullPath)
}

// matchByPath performs path-based lookup for drive files
func (pm *PodcastMatcher) matchByPath(podcast *PodcastEpisode) bool {
	drivePath := canonicalizePathForMatching(podcast.FilePath)
	if match, found := pm.podcastsByPath[drivePath]; found {
		updatePodcastMatch(podcast, match)
		return true
	}
	return false
}

// matchByDuration performs duration-based tiebreaking for multiple size matches
func (pm *PodcastMatcher) matchByDuration(podcast *PodcastEpisode, matches []*PodcastEpisode) bool {
	// Duration matching requires the drive podcast to have duration info
	// This would come from previous scans or metadata reading
	if podcast.Duration == 0 {
		return false
	}

	// Allow 2% tolerance for duration matching (encoding variations)
	tolerance := float64(podcast.Duration) * 0.02

	for _, match := range matches {
		if match.Duration == 0 {
			continue
		}
		diff := float64(podcast.Duration - match.Duration)
		if diff < 0 {
			diff = -diff
		}
		if diff <= tolerance {
			updatePodcastMatch(podcast, match)
			return true
		}
	}

	return false
}

// Match attempts to match a podcast with its local counterpart using a cascading strategy:
// 1. Path-based matching (fastest, works for tagged files)
// 2. Size-based matching (existing approach)
// 3. Duration-based tiebreaking (fast)
// 4. Checksum matching (slowest, final fallback)
func (pm *PodcastMatcher) Match(podcast *PodcastEpisode) error {
	// Try path-based matching first (fastest, handles tagged files)
	if pm.matchByPath(podcast) {
		return nil
	}

	// Fall back to size-based matching
	sizeMatches := pm.podcastsBySize[podcast.FileSize]

	if len(sizeMatches) == 1 {
		updatePodcastMatch(podcast, sizeMatches[0])
		return nil
	}

	if len(sizeMatches) > 1 {
		// Try duration-based matching for size collisions
		if pm.matchByDuration(podcast, sizeMatches) {
			return nil
		}

		// Fall back to checksum matching (slowest)
		return pm.matchByChecksum(podcast, sizeMatches)
	}

	return nil // No matches found
}

// Matches podcasts by comparing their checksums
func (pm *PodcastMatcher) matchByChecksum(podcast *PodcastEpisode, matches []*PodcastEpisode) error {
	checksum, err := getChecksum(podcast.FilePath)
	if err != nil {
		return err
	}

	for _, match := range matches {
		matchChecksum, err := getChecksum(match.FilePath)
		if err != nil {
			continue
		}
		if matchChecksum == checksum {
			updatePodcastMatch(podcast, match)
			return nil
		}
	}

	return nil // No checksum matches found
}

// Updates both the drive and local podcast information after a match
func updatePodcastMatch(podcast *PodcastEpisode, match *PodcastEpisode) {
	// Update drive podcast
	podcast.OnDrive = true
	podcast.ZTitle = match.ZTitle
	podcast.ShowName = match.ShowName
	podcast.Duration = match.Duration
	podcast.Published = match.Published

	// Update local podcast
	match.OnDrive = true
}
