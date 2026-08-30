package main

import (
	"os"
	"path/filepath"
	"testing"

	"go_redis/internal/config"
)

func TestEnsureDataDirectories(t *testing.T) {
	tempDir := t.TempDir()

	cfg := &config.Config{
		AOFPath:      filepath.Join(tempDir, "aof_dir", "appendonly.aof"),
		SnapshotPath: filepath.Join(tempDir, "snap_dir", "dump.db"),
		LogFile:      filepath.Join(tempDir, "logs_dir", "server.log"),
	}

	err := ensureDataDirectories(cfg)
	if err != nil {
		t.Fatalf("ensureDataDirectories failed: %v", err)
	}

	// Verify persistence directories were created
	dirs := []string{
		filepath.Join(tempDir, "aof_dir"),
		filepath.Join(tempDir, "snap_dir"),
		filepath.Join(tempDir, "logs_dir"),
	}

	for _, d := range dirs {
		info, err := os.Stat(d)
		if err != nil || !info.IsDir() {
			t.Errorf("expected directory %s to exist", d)
		}
	}
}
