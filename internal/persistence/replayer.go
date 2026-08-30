package persistence

import (
	"fmt"

	"go_redis/internal/server"
	"go_redis/internal/storage"
	"go_redis/pkg/utils"
)

// ColdRecovery loads the binary snapshot (if present).
func ColdRecovery(snapshotPath string, aofPath string, engine *storage.Engine, router *server.Router) error {
	if snapshotPath != "" {
		_ = utils.EnsureDir(snapshotPath)
		if err := LoadSnapshot(snapshotPath, engine); err != nil {
			return fmt.Errorf("failed during snapshot recovery: %w", err)
		}
	}
	return nil
}
