// Package internal provides internal utilities for managing podcast episodes on macOS.
package internal

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

// AppleEpochOffset is the difference between Apple's epoch (2001-01-01) and Unix epoch (1970-01-01)
const AppleEpochOffset = 978307200

type PodcastEpisode struct {
	ZTitle    string
	ShowName  string
	FilePath  string
	Published time.Time
	Selected  bool
	FileSize  int64
	OnDrive   bool
	Duration  time.Duration
	Progress  float64
}

func (p PodcastEpisode) Title() string {
	status := ""
	if p.OnDrive {
		status = "✓ "
	}
	return status + p.ZTitle
}

func (p PodcastEpisode) Description() string {
	parts := []string{p.ShowName}

	if !p.Published.IsZero() {
		parts = append(parts, p.Published.Format("2006-01-02"))
	}

	if p.Duration > 0 {
		parts = append(parts, formatDuration(p.Duration))
	}

	return strings.Join(parts, " • ")
}

func (p PodcastEpisode) FilterValue() string { return p.ZTitle }

// LoadMacPodcasts queries every podcast episodes from the local Apple Podcasts database
func LoadMacPodcasts() ([]PodcastEpisode, error) {
	dbPath := filepath.Join(
		os.Getenv("HOME"),
		"Library/Group Containers/243LU875E5.groups.com.apple.podcasts/Documents/MTLibrary.sqlite",
	)

	db, err := sql.Open("libsql", "file:"+dbPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(`
        SELECT 
            e.ZTITLE,
            p.ZTITLE,
            e.ZASSETURL,
            e.ZPUBDATE,
			e.ZDURATION
        FROM ZMTEPISODE e
        JOIN ZMTPODCAST p ON e.ZPODCASTUUID = p.ZUUID
        WHERE ZASSETURL IS NOT NULL
        ORDER BY e.ZPUBDATE DESC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodes []PodcastEpisode
	for rows.Next() {
		var e PodcastEpisode
		var pubDate int64
		var duration int64
		err := rows.Scan(&e.ZTitle, &e.ShowName, &e.FilePath, &pubDate, &duration)
		if err != nil {
			return nil, err
		}

		e.Published = time.Unix((pubDate + AppleEpochOffset), 0)
		e.Duration = time.Duration(duration) * time.Second
		episodes = append(episodes, e)
	}

	return episodes, nil
}

// LoadLocalPodcasts fills in the file size and checksum for each episode
func LoadLocalPodcasts(episodes []PodcastEpisode) ([]PodcastEpisode, error) {
	var errors []error
	for i := range episodes {
		filePath, err := convertFileURIToPath(episodes[i].FilePath)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to convert file URI for episode %s: %w", episodes[i].ZTitle, err))
			continue
		}

		fileInfo, err := os.Stat(filePath)
		if err == nil {
			episodes[i].FileSize = fileInfo.Size()
		} else if os.IsNotExist(err) {
			episodes[i].FileSize = 0
		} else {
			errors = append(errors, fmt.Errorf("failed to stat file for episode %s: %w", episodes[i].ZTitle, err))
		}
	}

	if len(errors) > 0 {
		return episodes, fmt.Errorf("errors loading local podcasts: %v", errors)
	}
	return episodes, nil
}
