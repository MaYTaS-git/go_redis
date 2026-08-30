package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFile(t *testing.T) {
	tempDir := t.TempDir()
	targetPath := filepath.Join(tempDir, "sub", "test.txt")
	content := []byte("hello atomic world")

	err := AtomicWriteFile(targetPath, content, 0644)
	if err != nil {
		t.Fatalf("AtomicWriteFile failed: %v", err)
	}

	readData, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}

	if string(readData) != string(content) {
		t.Errorf("got %q, want %q", readData, content)
	}
}

func TestEnsureDirAndEnsureDirExists(t *testing.T) {
	tempDir := t.TempDir()

	// 1. EnsureDirExists with a nested directory
	nestedDir := filepath.Join(tempDir, "a", "b", "c")
	if err := EnsureDirExists(nestedDir); err != nil {
		t.Fatalf("EnsureDirExists failed: %v", err)
	}
	info, err := os.Stat(nestedDir)
	if err != nil || !info.IsDir() {
		t.Fatalf("expected %s to exist as directory", nestedDir)
	}

	// 2. EnsureDir with a file path (should create parent directory)
	filePath := filepath.Join(tempDir, "parent_dir", "file.db")
	if err := EnsureDir(filePath); err != nil {
		t.Fatalf("EnsureDir with file path failed: %v", err)
	}
	info, err = os.Stat(filepath.Join(tempDir, "parent_dir"))
	if err != nil || !info.IsDir() {
		t.Fatalf("expected parent_dir to exist as directory")
	}

	// 3. EnsureDir with a directory path without extension (e.g., "data" or "./data")
	directDirPath := filepath.Join(tempDir, "data_folder")
	if err := EnsureDir(directDirPath); err != nil {
		t.Fatalf("EnsureDir with dir path failed: %v", err)
	}
	info, err = os.Stat(directDirPath)
	if err != nil || !info.IsDir() {
		t.Fatalf("expected data_folder to exist as directory")
	}

	// 4. EnsureDir with trailing slash
	trailingSlashPath := filepath.Join(tempDir, "trailing_slash") + string(filepath.Separator)
	if err := EnsureDir(trailingSlashPath); err != nil {
		t.Fatalf("EnsureDir with trailing slash failed: %v", err)
	}
	info, err = os.Stat(filepath.Join(tempDir, "trailing_slash"))
	if err != nil || !info.IsDir() {
		t.Fatalf("expected trailing_slash to exist as directory")
	}

	// 5. EnsureDir with empty or dot
	if err := EnsureDir("."); err != nil {
		t.Errorf("EnsureDir('.') should not error, got: %v", err)
	}
	if err := EnsureDir(""); err != nil {
		t.Errorf("EnsureDir('') should not error, got: %v", err)
	}

	// 6. EnsureDirExists with spaces in path name
	spacePath := filepath.Join(tempDir, "New folder with space", "sub folder", "data")
	if err := EnsureDirExists(spacePath); err != nil {
		t.Fatalf("EnsureDirExists with spaces in path failed: %v", err)
	}
	info, err = os.Stat(spacePath)
	if err != nil || !info.IsDir() {
		t.Fatalf("expected path with spaces to exist as directory")
	}

	// 7. Rapid delete and immediate recreate stress test
	for i := 0; i < 10; i++ {
		testDir := filepath.Join(tempDir, fmt.Sprintf("rapid_dir_%d", i))
		_ = os.MkdirAll(testDir, 0755)
		_ = os.Remove(testDir)
		if err := EnsureDir(testDir); err != nil {
			t.Fatalf("iteration %d: EnsureDir failed after deletion: %v", i, err)
		}
		info, err := os.Stat(testDir)
		if err != nil || !info.IsDir() {
			t.Fatalf("iteration %d: expected directory to exist after EnsureDir", i)
		}
	}
}
