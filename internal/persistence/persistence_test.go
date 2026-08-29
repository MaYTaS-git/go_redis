package persistence

import (
	"path/filepath"
	"testing"
	"time"

	"go_redis/internal/commands"
	"go_redis/internal/server"
	"go_redis/internal/storage"
)

func TestSnapshotSaveAndLoad(t *testing.T) {
	tempDir := t.TempDir()
	snapshotPath := filepath.Join(tempDir, "dump.db")

	eng := storage.NewEngine(10, "allkeys-lru")
	defer eng.Close()

	_ = eng.Set("k1", []byte("v1"), 0)
	_ = eng.Set("k2", []byte("v2"), 10000)

	snp := NewSnapshotter(snapshotPath, eng)
	err := snp.Save()
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Create new engine and restore
	eng2 := storage.NewEngine(10, "allkeys-lru")
	defer eng2.Close()

	err = LoadSnapshot(snapshotPath, eng2)
	if err != nil {
		t.Fatalf("LoadSnapshot failed: %v", err)
	}

	val1, found := eng2.Get("k1")
	if !found || string(val1) != "v1" {
		t.Errorf("k1 expected v1, got %q", val1)
	}

	val2, found := eng2.Get("k2")
	if !found || string(val2) != "v2" {
		t.Errorf("k2 expected v2, got %q", val2)
	}
}

func TestAOFWriteAndReplay(t *testing.T) {
	tempDir := t.TempDir()
	aofPath := filepath.Join(tempDir, "appendonly.aof")

	aof, err := NewAOF(aofPath, "always")
	if err != nil {
		t.Fatalf("NewAOF failed: %v", err)
	}

	aof.WriteCommand("SET", []byte("user"), []byte("alice"))
	aof.WriteCommand("SET", []byte("role"), []byte("admin"))
	aof.WriteCommand("INCR", []byte("counter"))

	time.Sleep(50 * time.Millisecond)
	_ = aof.Close()

	// Replay AOF
	eng := storage.NewEngine(10, "allkeys-lru")
	defer eng.Close()

	router := server.NewRouter()
	router.Register("SET", commands.HandleSet)
	router.Register("INCR", commands.HandleIncr)

	err = ColdRecovery("", aofPath, eng, router)
	if err != nil {
		t.Fatalf("ColdRecovery failed: %v", err)
	}

	valUser, found := eng.Get("user")
	if !found || string(valUser) != "alice" {
		t.Errorf("expected alice, got %q", valUser)
	}

	valCnt, found := eng.Get("counter")
	if !found || string(valCnt) != "1" {
		t.Errorf("expected 1, got %q", valCnt)
	}
}
