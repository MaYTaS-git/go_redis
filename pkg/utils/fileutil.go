package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// EnsureDir ensures that the parent directory for targetPath exists.
func EnsureDir(targetPath string) error {
	dir := filepath.Dir(targetPath)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0755)
}

// AtomicWriteFile writes data to targetPath atomically by writing to a temporary file first and then renaming it.
func AtomicWriteFile(targetPath string, data []byte, perm os.FileMode) error {
	if err := EnsureDir(targetPath); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
	}

	tmpPath := targetPath + ".tmp"
	tmpFile, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("failed to create temp file %s: %w", tmpPath, err)
	}

	_, writeErr := tmpFile.Write(data)
	syncErr := tmpFile.Sync()
	closeErr := tmpFile.Close()

	if writeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to write to temp file %s: %w", tmpPath, writeErr)
	}
	if syncErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to sync temp file %s: %w", tmpPath, syncErr)
	}
	if closeErr != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp file %s: %w", tmpPath, closeErr)
	}

	if err := os.Rename(tmpPath, targetPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp file to %s: %w", targetPath, err)
	}

	return nil
}
