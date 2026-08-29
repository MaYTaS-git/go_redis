package utils

import (
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
