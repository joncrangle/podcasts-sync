package internal

type PodcastMatcher struct {
	podcastsBySize map[int64][]*PodcastEpisode
}

// Creates a new PodcastMatcher instance
func NewPodcastMatcher(podcastsBySize map[int64][]*PodcastEpisode) *PodcastMatcher {
	return &PodcastMatcher{
		podcastsBySize: podcastsBySize,
	}
}

// Attempts to match a podcast with its local counterpart
func (pm *PodcastMatcher) Match(podcast *PodcastEpisode) error {
	sizeMatches := pm.podcastsBySize[podcast.FileSize]

	if len(sizeMatches) == 1 {
		updatePodcastMatch(podcast, sizeMatches[0])
		return nil
	}

	if len(sizeMatches) > 1 {
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
