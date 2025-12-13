package internal

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// createTestMP3 creates a minimal valid MP3 file for testing.
// This creates a very basic MP3 with just an ID3v2 header and minimal audio data.
func createTestMP3(t *testing.T, path string) {
	t.Helper()

	// Minimal MP3: ID3v2 header + basic MP3 frame
	// ID3v2.3 header: "ID3" + version (03 00) + flags (00) + size (00 00 00 00)
	id3Header := []byte{
		0x49, 0x44, 0x33, // "ID3"
		0x03, 0x00, // version 2.3.0
		0x00,                   // flags
		0x00, 0x00, 0x00, 0x00, // size (synchsafe int)
	}

	// Basic MP3 frame header (MPEG-1 Layer 3)
	mp3Frame := []byte{
		0xFF, 0xFB, 0x90, 0x00, // Frame sync + MPEG-1 Layer 3
	}

	// Add some audio data padding
	audioData := make([]byte, 100)
	for i := range audioData {
		audioData[i] = 0x00
	}

	data := append(id3Header, mp3Frame...)
	data = append(data, audioData...)

	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("Failed to create test MP3: %v", err)
	}
}

func TestCleanupID3TempFiles(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name          string
		setupFiles    []string
		targetFile    string
		expectRemoved []string
	}{
		{
			name:          "removes -id3v2 temp file",
			setupFiles:    []string{"test.mp3", "test.mp3-id3v2"},
			targetFile:    "test.mp3",
			expectRemoved: []string{"test.mp3-id3v2"},
		},
		{
			name:          "removes .id3 file",
			setupFiles:    []string{"test.mp3", "test.mp3.id3"},
			targetFile:    "test.mp3",
			expectRemoved: []string{"test.mp3.id3"},
		},
		{
			name:          "removes both temp files",
			setupFiles:    []string{"test.mp3", "test.mp3-id3v2", "test.mp3.id3"},
			targetFile:    "test.mp3",
			expectRemoved: []string{"test.mp3-id3v2", "test.mp3.id3"},
		},
		{
			name:          "no error when no temp files exist",
			setupFiles:    []string{"test.mp3"},
			targetFile:    "test.mp3",
			expectRemoved: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test files
			for _, file := range tt.setupFiles {
				path := filepath.Join(tempDir, file)
				if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			}

			// Run cleanup
			targetPath := filepath.Join(tempDir, tt.targetFile)
			err := CleanupID3TempFiles(targetPath)
			if err != nil {
				t.Errorf("cleanupID3TempFiles() error = %v", err)
			}

			// Verify expected files were removed
			for _, file := range tt.expectRemoved {
				path := filepath.Join(tempDir, file)
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Errorf("Expected file %s to be removed, but it still exists", file)
				}
			}

			// Verify original file still exists
			if _, err := os.Stat(targetPath); err != nil {
				t.Errorf("Original file should still exist: %v", err)
			}

			// Cleanup for next test
			for _, file := range tt.setupFiles {
				_ = os.Remove(filepath.Join(tempDir, file))
			}
		})
	}
}

func TestVerifyNoTempFiles(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name       string
		setupFiles []string
		targetFile string
		wantErr    bool
	}{
		{
			name:       "error when -id3v2 temp file exists",
			setupFiles: []string{"test.mp3", "test.mp3-id3v2"},
			targetFile: "test.mp3",
			wantErr:    true,
		},
		{
			name:       "error when .id3 file exists",
			setupFiles: []string{"test.mp3", "test.mp3.id3"},
			targetFile: "test.mp3",
			wantErr:    true,
		},
		{
			name:       "no error when no temp files exist",
			setupFiles: []string{"test.mp3"},
			targetFile: "test.mp3",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test files
			for _, file := range tt.setupFiles {
				path := filepath.Join(tempDir, file)
				if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			}

			// Run verification
			targetPath := filepath.Join(tempDir, tt.targetFile)
			err := VerifyNoTempFiles(targetPath)

			if (err != nil) != tt.wantErr {
				t.Errorf("verifyNoTempFiles() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Cleanup for next test
			for _, file := range tt.setupFiles {
				_ = os.Remove(filepath.Join(tempDir, file))
			}
		})
	}
}

func TestAddID3Tags(t *testing.T) {
	tempDir := t.TempDir()

	tests := []struct {
		name         string
		fileExt      string
		episode      PodcastEpisode
		wantErr      bool
		expectTagged bool
	}{
		{
			name:    "successfully tags MP3 file",
			fileExt: ".mp3",
			episode: PodcastEpisode{
				ZTitle:    "Test Episode",
				ShowName:  "Test Show",
				Published: time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
			},
			wantErr:      false,
			expectTagged: true,
		},
		{
			name:    "skips non-MP3 file",
			fileExt: ".m4a",
			episode: PodcastEpisode{
				ZTitle:   "Test Episode",
				ShowName: "Test Show",
			},
			wantErr:      false,
			expectTagged: false,
		},
		{
			name:    "skips WAV file",
			fileExt: ".wav",
			episode: PodcastEpisode{
				ZTitle:   "Test Episode",
				ShowName: "Test Show",
			},
			wantErr:      false,
			expectTagged: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tempDir, "test"+tt.fileExt)

			if tt.fileExt == ".mp3" {
				// Create a valid MP3 file
				createTestMP3(t, testFile)
			} else {
				// Create a simple file for non-MP3 tests
				if err := os.WriteFile(testFile, []byte("test audio data"), 0o644); err != nil {
					t.Fatalf("Failed to create test file: %v", err)
				}
			}

			// Run AddID3Tags
			err := AddID3Tags(testFile, tt.episode)

			if (err != nil) != tt.wantErr {
				t.Errorf("AddID3Tags() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Verify no temp files remain
			tempFile := testFile + "-id3v2"
			if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
				t.Errorf("Temp file %s should not exist after AddID3Tags", tempFile)
			}

			id3File := testFile + ".id3"
			if _, err := os.Stat(id3File); !os.IsNotExist(err) {
				t.Errorf("ID3 file %s should not exist after AddID3Tags", id3File)
			}

			// Verify original file still exists
			if _, err := os.Stat(testFile); err != nil {
				t.Errorf("Original file should still exist: %v", err)
			}

			// Cleanup
			_ = os.Remove(testFile)
		})
	}
}

func TestAddID3Tags_CleanupOnFailure(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.mp3")

	// Create a valid MP3 file
	createTestMP3(t, testFile)

	// Create a pre-existing temp file to simulate a previous failed attempt
	tempFile := testFile + "-id3v2"
	if err := os.WriteFile(tempFile, []byte("old temp"), 0o644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	episode := PodcastEpisode{
		ZTitle:   "Test Episode",
		ShowName: "Test Show",
	}

	// Run AddID3Tags - it should clean up the pre-existing temp file
	_ = AddID3Tags(testFile, episode)

	// Verify the old temp file was cleaned up
	// Note: After successful tagging, no temp files should remain
	if _, err := os.Stat(tempFile); !os.IsNotExist(err) {
		t.Errorf("Pre-existing temp file should have been cleaned up")
	}
}
