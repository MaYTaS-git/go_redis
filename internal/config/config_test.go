package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Port != 6379 {
		t.Errorf("got port %d, want 6379", cfg.Port)
	}
	if cfg.MaxMemoryMB != 512 {
		t.Errorf("got max_memory_mb %d, want 512", cfg.MaxMemoryMB)
	}
	if !cfg.AOFEnabled {
		t.Errorf("expected AOF to be enabled by default")
	}
}

func TestLoadConfigFile(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "config.txt")

	content := []byte(`# Test Config File
port 7000
bind 127.0.0.1
requirepass mysecretpassword
max_memory_mb 1024
eviction_policy volatile-lru
log_enabled yes
log_level debug
log_requests yes
log_file ./test.log
aof_enabled yes
aof_fsync always
snapshot_enabled no
metrics_port 9091
`)

	err := os.WriteFile(configPath, content, 0644)
	if err != nil {
		t.Fatalf("failed to write test config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.Port != 7000 {
		t.Errorf("got port %d, want 7000", cfg.Port)
	}
	if cfg.Bind != "127.0.0.1" {
		t.Errorf("got bind %s, want 127.0.0.1", cfg.Bind)
	}
	if cfg.RequirePass != "mysecretpassword" {
		t.Errorf("got requirepass %s, want mysecretpassword", cfg.RequirePass)
	}
	if cfg.MaxMemoryMB != 1024 {
		t.Errorf("got max_memory_mb %d, want 1024", cfg.MaxMemoryMB)
	}
	if cfg.EvictionPolicy != "volatile-lru" {
		t.Errorf("got eviction_policy %s, want volatile-lru", cfg.EvictionPolicy)
	}
	if !cfg.LogEnabled {
		t.Errorf("expected LogEnabled true")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("got log_level %s, want debug", cfg.LogLevel)
	}
	if !cfg.LogRequests {
		t.Errorf("expected LogRequests true")
	}
	if cfg.LogFile != "./test.log" {
		t.Errorf("got log_file %s, want ./test.log", cfg.LogFile)
	}
	if !cfg.AOFEnabled {
		t.Errorf("expected AOF enabled")
	}
	if cfg.AOFFsync != "always" {
		t.Errorf("got aof_fsync %s, want always", cfg.AOFFsync)
	}
	if cfg.SnapshotEnabled {
		t.Errorf("expected snapshot disabled")
	}
	if cfg.MetricsPort != 9091 {
		t.Errorf("got metrics_port %d, want 9091", cfg.MetricsPort)
	}
}
