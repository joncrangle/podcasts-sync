package internal

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUSBDrive_Methods(t *testing.T) {
	drive := USBDrive{
		Name:      "Test Drive",
		MountPath: "/Volumes/TestDrive",
		Folder:    "podcasts",
	}

	if drive.Title() != "Test Drive" {
		t.Errorf("Expected Title() to return 'Test Drive', got %s", drive.Title())
	}

	if drive.Description() != "/Volumes/TestDrive" {
		t.Errorf("Expected Description() to return '/Volumes/TestDrive', got %s", drive.Description())
	}

	if drive.FilterValue() != "Test Drive" {
		t.Errorf("Expected FilterValue() to return 'Test Drive', got %s", drive.FilterValue())
	}
}

func TestNewDriveManager(t *testing.T) {
	tests := []struct {
		name          string
		volumesPath   string
		template      DirectoryTemplate
		expectDefault bool
	}{
		{
			name:          "with custom template",
			volumesPath:   "/Volumes",
			template:      DirectoryTemplate{ShowNameFormat: "{show}", SanitizeNames: false},
			expectDefault: false,
		},
		{
			name:          "with empty template should use default",
			volumesPath:   "/Volumes",
			template:      DirectoryTemplate{},
			expectDefault: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dm := NewDriveManager(tt.volumesPath, tt.template)

			if dm.volumesPath != tt.volumesPath {
				t.Errorf("Expected volumesPath to be %s, got %s", tt.volumesPath, dm.volumesPath)
			}

			if tt.expectDefault {
				if dm.template.ShowNameFormat != defaultDirTemplate.ShowNameFormat {
					t.Error("Expected default template to be used")
				}
			}
		})
	}
}

func TestDriveManager_DetectDrives(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir := t.TempDir()

	// Create some test "volumes"
	testDrives := []string{"TestDrive1", "TestDrive2", "Macintosh HD"}
	for _, drive := range testDrives {
		drivePath := filepath.Join(tempDir, drive)
		err := os.Mkdir(drivePath, 0o755)
		if err != nil {
			t.Fatalf("Failed to create test drive directory: %v", err)
		}
	}

	dm := NewDriveManager(tempDir, DirectoryTemplate{})
	drives, err := dm.DetectDrives()
	if err != nil {
		t.Fatalf("DetectDrives() failed: %v", err)
	}

	// Should detect 2 drives (excluding "Macintosh HD")
	if len(drives) != 2 {
		t.Errorf("Expected 2 drives, got %d", len(drives))
	}

	// Check that Macintosh HD was excluded
	for _, drive := range drives {
		if drive.Name == "Macintosh HD" {
			t.Error("Macintosh HD should be excluded from detected drives")
		}
	}

	// Check that detected drives have correct properties
	for _, drive := range drives {
		if drive.Folder != "podcasts" {
			t.Errorf("Expected folder to be 'podcasts', got %s", drive.Folder)
		}

		expectedPath := filepath.Join(tempDir, drive.Name)
		if drive.MountPath != expectedPath {
			t.Errorf("Expected mount path to be %s, got %s", expectedPath, drive.MountPath)
		}
	}
}

func TestNewPodcastScanner(t *testing.T) {
	tests := []struct {
		name          string
		template      DirectoryTemplate
		expectDefault bool
	}{
		{
			name:          "with custom template",
			template:      DirectoryTemplate{ShowNameFormat: "{show}", SanitizeNames: false},
			expectDefault: false,
		},
		{
			name:          "with empty template should use default",
			template:      DirectoryTemplate{},
			expectDefault: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ps := NewPodcastScanner(tt.template)

			if tt.expectDefault {
				if ps.template.ShowNameFormat != defaultDirTemplate.ShowNameFormat {
					t.Error("Expected default template to be used")
				}
			} else {
				if ps.template.SanitizeNames == defaultDirTemplate.SanitizeNames {
					t.Error("Expected custom template to be used")
				}
			}
		})
	}
}

func TestNewPodcastSync(t *testing.T) {
	ps := NewPodcastSync()
	if ps == nil {
		t.Error("NewPodcastSync() returned nil")
	}
}

func TestIsReadableDrive(t *testing.T) {
	// Test with a readable directory (temp dir)
	tempDir := t.TempDir()
	if !isReadableDrive(tempDir) {
		t.Error("Expected temp directory to be readable")
	}

	// Test with a non-existent path
	nonExistentPath := "/this/path/does/not/exist"
	if isReadableDrive(nonExistentPath) {
		t.Error("Expected non-existent path to not be readable")
	}
}

func TestInitializeProgress(t *testing.T) {
	totalBytes := int64(1024)
	totalFiles := 5

	progress := initializeProgress(totalBytes, totalFiles)

	if progress.TotalBytes != totalBytes {
		t.Errorf("Expected TotalBytes to be %d, got %d", totalBytes, progress.TotalBytes)
	}

	if progress.TotalFiles != totalFiles {
		t.Errorf("Expected TotalFiles to be %d, got %d", totalFiles, progress.TotalFiles)
	}

	if progress.StartTime.IsZero() {
		t.Error("Expected StartTime to be set")
	}
}

func TestNewFileOp(t *testing.T) {
	progress := TransferProgress{TotalBytes: 1024}
	complete := true
	testErr := &testError{msg: "test error"}

	fileOp := newFileOp(progress, complete, testErr)

	if fileOp.Progress.TotalBytes != 1024 {
		t.Errorf("Expected TotalBytes to be 1024, got %d", fileOp.Progress.TotalBytes)
	}

	if !fileOp.Complete {
		t.Error("Expected Complete to be true")
	}

	if fileOp.Error == nil {
		t.Error("Expected Error to be set")
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func TestUSBDrivesEqual(t *testing.T) {
	drive1 := USBDrive{Name: "Drive1", MountPath: "/path1", Folder: "podcasts"}
	drive2 := USBDrive{Name: "Drive2", MountPath: "/path2", Folder: "podcasts"}

	drives1 := []USBDrive{drive1, drive2}
	drives2 := []USBDrive{drive1, drive2}
	drives3 := []USBDrive{drive1}

	if !USBDrivesEqual(drives1, drives2) {
		t.Error("Expected identical drive lists to be equal")
	}

	if USBDrivesEqual(drives1, drives3) {
		t.Error("Expected different length drive lists to not be equal")
	}
}

func TestPodcastSync_DeleteSelected(t *testing.T) {
	t.Run("delete single file and empty directory", func(t *testing.T) {
		// Create a temporary directory structure
		tempDir := t.TempDir()
		showDir := filepath.Join(tempDir, "TestShow")
		err := os.MkdirAll(showDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create show directory: %v", err)
		}

		testFile := filepath.Join(showDir, "test.mp3")

		// Create a test file
		err = os.WriteFile(testFile, []byte("test content"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		episodes := []PodcastEpisode{
			{
				ZTitle:   "Test Episode",
				FilePath: testFile,
				Selected: true,
			},
		}

		ps := NewPodcastSync()
		result := ps.DeleteSelected(episodes)

		if result.Error != nil {
			t.Errorf("Expected no error, got %v", result.Error)
		}

		if !result.Complete {
			t.Error("Expected operation to be complete")
		}

		// Check that file was deleted
		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("Expected file to be deleted")
		}

		// Check that empty directory was also deleted
		if _, err := os.Stat(showDir); !os.IsNotExist(err) {
			t.Error("Expected empty directory to be deleted")
		}
	})

	t.Run("delete one file but keep non-empty directory", func(t *testing.T) {
		// Create a temporary directory structure
		tempDir := t.TempDir()
		showDir := filepath.Join(tempDir, "TestShow")
		err := os.MkdirAll(showDir, 0o755)
		if err != nil {
			t.Fatalf("Failed to create show directory: %v", err)
		}

		testFile1 := filepath.Join(showDir, "test1.mp3")
		testFile2 := filepath.Join(showDir, "test2.mp3")

		// Create two test files
		err = os.WriteFile(testFile1, []byte("test content 1"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file 1: %v", err)
		}
		err = os.WriteFile(testFile2, []byte("test content 2"), 0o644)
		if err != nil {
			t.Fatalf("Failed to create test file 2: %v", err)
		}

		episodes := []PodcastEpisode{
			{
				ZTitle:   "Test Episode 1",
				FilePath: testFile1,
				Selected: true,
			},
			{
				ZTitle:   "Test Episode 2",
				FilePath: testFile2,
				Selected: false, // Not selected for deletion
			},
		}

		ps := NewPodcastSync()
		result := ps.DeleteSelected(episodes)

		if result.Error != nil {
			t.Errorf("Expected no error, got %v", result.Error)
		}

		if !result.Complete {
			t.Error("Expected operation to be complete")
		}

		// Check that first file was deleted
		if _, err := os.Stat(testFile1); !os.IsNotExist(err) {
			t.Error("Expected first file to be deleted")
		}

		// Check that second file still exists
		if _, err := os.Stat(testFile2); err != nil {
			t.Error("Expected second file to still exist")
		}

		// Check that directory still exists (not empty)
		if _, err := os.Stat(showDir); err != nil {
			t.Error("Expected directory to still exist since it's not empty")
		}
	})
}
