package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

type PodcastEpisode struct {
	ZTitle         string
	PTitle         string
	ShowName       string
	FilePath       string
	Published      time.Time
	Selected       bool
	FileSize       int64
	OnDrive        bool
	DriveState     DriveState
	Duration       time.Duration
	DateDownloaded time.Time
	Progress       float64
	MD5Hash        string
}

func (p PodcastEpisode) Title() string {
	status := ""
	switch p.DriveState {
	case OnDriveNewer:
		status = "📱 "
	case OnDriveOlder:
		status = "💻 "
	case OnDriveSame:
		status = "✓ "
	}
	return status + p.ZTitle
}
func (p PodcastEpisode) Description() string {
	return fmt.Sprintf("%s • %s • %s",
		p.ShowName,
		p.Published.Format("2006-01-02"),
		formatDuration(p.Duration))
}
func (p PodcastEpisode) FilterValue() string { return p.ZTitle }

type macPodcastsMsg []PodcastEpisode

// type drivePodcastsMsg []PodcastEpisode

type DriveState int

const (
	NotOnDrive DriveState = iota
	OnDriveNewer
	OnDriveOlder
	OnDriveSame
)

type BatchOperation struct {
	Type             BatchOpType
	Episodes         []PodcastEpisode
	Progress         float64
	TransferProgress TransferProgress
	Current          int
	Total            int
}

type BatchOpType int

const (
	BatchSync BatchOpType = iota
	BatchDelete
	BatchValidate
)

type TransferProgress struct {
	CurrentFile      string
	CurrentProgress  float64
	BytesTransferred int64
	TotalBytes       int64
	Speed            float64 // bytes per second
	TimeRemaining    time.Duration
	StartTime        time.Time
	FilesDone        int
	TotalFiles       int
}

// Messages for detailed progress
type progressUpdateMsg struct {
	Progress TransferProgress
}

type batchProgressMsg struct {
	Batch BatchOperation
}

type fileOpMsg struct {
	Operation string // "copy" or "delete"
	Progress  float64
	Filename  string
}

type fileOpCompleteMsg struct {
	Operation string
	Error     error
}

type syncProgressMsg float64
type syncCompleteMsg struct{}

// Query podcast episodes from the Apple Podcasts database
func loadLocalPodcasts() tea.Msg {
	dbPath := filepath.Join(
		os.Getenv("HOME"),
		"Library/Group Containers/243LU875E5.groups.com.apple.podcasts/Documents/MTLibrary.sqlite",
	)

	db, err := sql.Open("libsql", "file:"+dbPath)
	if err != nil {
		return errMsg{err}
	}
	defer db.Close()

	rows, err := db.Query(`
        SELECT 
            e.ZTITLE,
			p.ZTITLE,
            p.ZAUTHOR,
            e.ZASSETURL,
            e.ZPUBDATE,
			e.ZDURATION
        FROM ZMTEPISODE e
        JOIN ZMTPODCAST p ON e.ZPODCASTUUID = p.ZUUID
        WHERE ZASSETURL IS NOT NULL
        ORDER BY e.ZPUBDATE DESC
    `)
	if err != nil {
		return errMsg{err}
	}
	defer rows.Close()

	var episodes []PodcastEpisode
	for rows.Next() {
		var e PodcastEpisode
		var pubDate int64
		var duration int64
		err := rows.Scan(&e.ZTitle, &e.PTitle, &e.ShowName, &e.FilePath, &pubDate, &duration)
		if err != nil {
			return errMsg{err}
		}

		e.Published = time.Unix((pubDate + 978307200), 0)
		e.Duration = time.Duration(duration) * time.Second
		episodes = append(episodes, e)
	}

	return macPodcastsMsg(episodes)
}

func calculateMD5(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func startBatchOperation(op BatchOpType, episodes []PodcastEpisode, drive USBDrive) tea.Cmd {
	return func() tea.Msg {
		batch := BatchOperation{
			Type:     op,
			Episodes: episodes,
			Total:    len(episodes),
		}

		//TODO: handle batch operations
		switch op {
		case BatchSync:
			return handleBatchSync(batch, drive)
			// case BatchDelete:
			// 	return handleBatchDelete(batch, drive)
			// case BatchValidate:
			// 	return handleBatchValidate(batch, drive)
		}

		return batch
	}
}
