package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds all server configuration settings.
type Config struct {
	Port                int    `json:"port"`
	Bind                string `json:"bind"`
	RequirePass         string `json:"requirepass"`
	TLSEnabled          bool   `json:"tls_enabled"`
	TLSCert             string `json:"tls_cert"`
	TLSKey              string `json:"tls_key"`
	MaxMemoryMB         int64  `json:"max_memory_mb"`
	EvictionPolicy      string `json:"eviction_policy"`
	AOFEnabled          bool   `json:"aof_enabled"`
	AOFFsync            string `json:"aof_fsync"`
	AOFPath             string `json:"aof_path"`
	SnapshotEnabled     bool   `json:"snapshot_enabled"`
	SnapshotIntervalSec int    `json:"snapshot_interval_sec"`
	SnapshotPath        string `json:"snapshot_path"`
	MetricsPort         int    `json:"metrics_port"`
	LogEnabled          bool   `json:"log_enabled"`
	LogLevel            string `json:"log_level"`
	LogRequests         bool   `json:"log_requests"`
	LogFile             string `json:"log_file"`
}

// DefaultConfig returns production-ready default settings.
func DefaultConfig() *Config {
	return &Config{
		Port:                6379,
		Bind:                "0.0.0.0",
		RequirePass:         "",
		TLSEnabled:          false,
		TLSCert:             "",
		TLSKey:              "",
		MaxMemoryMB:         512,
		EvictionPolicy:      "allkeys-lru",
		AOFEnabled:          true,
		AOFFsync:            "everysec",
		AOFPath:             "./data/appendonly.aof",
		SnapshotEnabled:     true,
		SnapshotIntervalSec: 300,
		SnapshotPath:        "./data/dump.db",
		MetricsPort:         9090,
		LogEnabled:          true,
		LogLevel:            "info",
		LogRequests:         true,
		LogFile:             "",
	}
}

// LoadConfig attempts to locate config.txt near executable, then current working directory,
// falling back to defaults if not found.
func LoadConfig(specifiedPath string) (*Config, error) {
	cfg := DefaultConfig()

	path := findConfigFile(specifiedPath)
	if path == "" {
		return cfg, nil
	}

	err := parseConfigFile(path, cfg)
	if err != nil {
		return nil, fmt.Errorf("error parsing config file %s: %w", path, err)
	}

	return cfg, nil
}

func findConfigFile(specifiedPath string) string {
	if specifiedPath != "" {
		if _, err := os.Stat(specifiedPath); err == nil {
			return specifiedPath
		}
	}

	// Check executable directory
	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Dir(execPath)
		candidate := filepath.Join(execDir, "config.txt")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Check current working directory
	if _, err := os.Stat("config.txt"); err == nil {
		return "config.txt"
	}

	return ""
}

func parseConfigFile(path string, cfg *Config) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if idx := strings.Index(line, "#"); idx != -1 {
			line = strings.TrimSpace(line[:idx])
		}
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		key := strings.ToLower(parts[0])
		val := strings.TrimSpace(strings.Join(parts[1:], " "))

		switch key {
		case "port":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.Port = v
			}
		case "bind":
			cfg.Bind = val
		case "requirepass":
			cfg.RequirePass = val
		case "tls_enabled":
			cfg.TLSEnabled = parseBool(val)
		case "tls_cert":
			cfg.TLSCert = val
		case "tls_key":
			cfg.TLSKey = val
		case "max_memory_mb":
			if v, err := strconv.ParseInt(val, 10, 64); err == nil {
				cfg.MaxMemoryMB = v
			}
		case "eviction_policy":
			cfg.EvictionPolicy = strings.ToLower(val)
		case "aof_enabled":
			cfg.AOFEnabled = parseBool(val)
		case "aof_fsync":
			cfg.AOFFsync = strings.ToLower(val)
		case "aof_path":
			cfg.AOFPath = val
		case "snapshot_enabled":
			cfg.SnapshotEnabled = parseBool(val)
		case "snapshot_interval_sec":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.SnapshotIntervalSec = v
			}
		case "snapshot_path":
			cfg.SnapshotPath = val
		case "metrics_port":
			if v, err := strconv.Atoi(val); err == nil {
				cfg.MetricsPort = v
			}
		case "log_enabled":
			cfg.LogEnabled = parseBool(val)
		case "log_level":
			cfg.LogLevel = strings.ToLower(val)
		case "log_requests":
			cfg.LogRequests = parseBool(val)
		case "log_file":
			cfg.LogFile = val
		}
	}

	return scanner.Err()
}

func parseBool(v string) bool {
	v = strings.ToLower(strings.TrimSpace(v))
	return v == "true" || v == "yes" || v == "1"
}
