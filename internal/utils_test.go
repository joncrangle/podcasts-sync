package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSystemHiddenFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"DS_Store", ".DS_Store", true},
		{"Spotlight", ".Spotlight-V100", true},
		{"Trashes", ".Trashes", true},
		{"fseventsd", ".fseventsd", true},
		{"TemporaryItems", ".TemporaryItems", true},
		{"VolumeIcon", ".VolumeIcon.icns", true},
		{"AppleDouble", "._somefile", true},
		{"regular file", "test.mp3", false},
		{"hidden regular file", ".hidden.txt", false},
		{"underscore file", "_test.mp3", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSystemHiddenFile(tt.filename); got != tt.want {
				t.Errorf("isSystemHiddenFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestIsDirEmpty(t *testing.T) {
	t.Run("truly empty directory", func(t *testing.T) {
		tempDir := t.TempDir()
		empty, err := isDirEmpty(tempDir)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !empty {
			t.Error("Expected directory to be empty")
		}
	})

	t.Run("directory with visible files", func(t *testing.T) {
		tempDir := t.TempDir()
		testFile := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(testFile, []byte("test"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		empty, err := isDirEmpty(tempDir)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if empty {
			t.Error("Expected directory to not be empty")
		}
	})

	t.Run("directory with only hidden system files", func(t *testing.T) {
		tempDir := t.TempDir()
		dsStore := filepath.Join(tempDir, ".DS_Store")
		err := os.WriteFile(dsStore, []byte("test"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create .DS_Store: %v", err)
		}

		hiddenFile := filepath.Join(tempDir, "._test")
		err = os.WriteFile(hiddenFile, []byte("test"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create hidden file: %v", err)
		}

		empty, err := isDirEmpty(tempDir)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !empty {
			t.Error("Expected directory to be considered empty (only system hidden files)")
		}
	})

	t.Run("directory with mix of visible and hidden files", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create visible file
		testFile := filepath.Join(tempDir, "test.txt")
		err := os.WriteFile(testFile, []byte("test"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Create hidden system file
		dsStore := filepath.Join(tempDir, ".DS_Store")
		err = os.WriteFile(dsStore, []byte("test"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create .DS_Store: %v", err)
		}

		empty, err := isDirEmpty(tempDir)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if empty {
			t.Error("Expected directory to not be empty (has visible file)")
		}
	})
}

func TestCleanupSystemHiddenFiles(t *testing.T) {
	t.Run("removes DS_Store and other hidden files", func(t *testing.T) {
		tempDir := t.TempDir()

		// Create various files
		testFile := filepath.Join(tempDir, "test.txt")
		dsStore := filepath.Join(tempDir, ".DS_Store")
		hiddenFile := filepath.Join(tempDir, "._hidden")

		err := os.WriteFile(testFile, []byte("keep"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
		err = os.WriteFile(dsStore, []byte("remove"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create .DS_Store: %v", err)
		}
		err = os.WriteFile(hiddenFile, []byte("remove"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create hidden file: %v", err)
		}

		// Clean up hidden files
		cleanupSystemHiddenFiles(tempDir)

		// Check that regular file still exists
		if _, err := os.Stat(testFile); os.IsNotExist(err) {
			t.Error("Expected regular file to still exist")
		}

		// Check that hidden files were removed
		if _, err := os.Stat(dsStore); !os.IsNotExist(err) {
			t.Error("Expected .DS_Store to be removed")
		}
		if _, err := os.Stat(hiddenFile); !os.IsNotExist(err) {
			t.Error("Expected ._hidden file to be removed")
		}
	})

	t.Run("handles non-existent directory gracefully", func(_ *testing.T) {
		// Should not panic or error
		cleanupSystemHiddenFiles("/non/existent/path")
	})
}
