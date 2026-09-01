package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureDir ensures that the directory at targetPath (or the parent directory if targetPath has a file extension) exists, creating it if necessary.
func EnsureDir(targetPath string) error {
	if targetPath == "" || targetPath == "." {
		return nil
	}

	clean := filepath.Clean(targetPath)
	// If path has a file extension (e.g. .aof, .db, .log, .txt) and does not end with a path separator, treat as a file path and resolve its parent directory
	if filepath.Ext(clean) != "" && !strings.HasSuffix(targetPath, "/") && !strings.HasSuffix(targetPath, "\\") {
		clean = filepath.Dir(clean)
		if clean == "" || clean == "." {
			return nil
		}
	}

	if abs, err := filepath.Abs(clean); err == nil {
		clean = abs
	}

	fi, err := os.Stat(clean)
	if err == nil {
		if fi.IsDir() {
			return nil
		}
		return fmt.Errorf("path %s exists and is not a directory", clean)
	}

	return os.MkdirAll(clean, 0755)
}

// EnsureDirExists ensures that the specified directory exists.
func EnsureDirExists(dirPath string) error {
	return EnsureDir(dirPath)
}

// AtomicWriteFile writes data to targetPath atomically by writing to a temporary file first and then renaming it.
func AtomicWriteFile(targetPath string, data []byte, perm os.FileMode) error {
	if err := EnsureDir(targetPath); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", targetPath, err)
	}

	cleanPath := filepath.Clean(targetPath)
	if abs, err := filepath.Abs(cleanPath); err == nil {
		cleanPath = abs
	}

	dir := filepath.Dir(cleanPath)
	tmpFile, err := os.CreateTemp(dir, "go_redis-tmp-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to write to temp file %s: %w", tmpName, err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("failed to sync temp file %s: %w", tmpName, err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file %s: %w", tmpName, err)
	}

	if err := os.Rename(tmpName, cleanPath); err != nil {
		return fmt.Errorf("failed to rename temp file to %s: %w", cleanPath, err)
	}

	return nil
}
