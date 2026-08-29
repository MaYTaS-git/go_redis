package persistence

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"

	"go_redis/internal/storage"
	"go_redis/pkg/utils"
)

var MagicHeader = []byte("REDIS_GO_DB01")

// Snapshotter manages background snapshot save operations.
type Snapshotter struct {
	path       string
	engine     *storage.Engine
	inProgress int32
	mu         sync.Mutex
}

// NewSnapshotter creates a snapshot manager.
func NewSnapshotter(path string, engine *storage.Engine) *Snapshotter {
	return &Snapshotter{
		path:   path,
		engine: engine,
	}
}

// Save executes a synchronous snapshot to disk atomically.
func (s *Snapshotter) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := utils.EnsureDir(s.path); err != nil {
		return fmt.Errorf("failed to ensure snapshot dir: %w", err)
	}

	tmpPath := s.path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("failed to open snapshot tmp file: %w", err)
	}

	// Write magic header
	if _, err := f.Write(MagicHeader); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}

	// Iterate over all entries in storage engine
	var writeErr error
	s.engine.Iterate(func(key string, val []byte, expiresAt int64) bool {
		keyBytes := utils.StringToBytes(key)

		// Key Len (4 bytes)
		var lenBuf [4]byte
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(keyBytes)))
		if _, err := f.Write(lenBuf[:]); err != nil {
			writeErr = err
			return false
		}
		if _, err := f.Write(keyBytes); err != nil {
			writeErr = err
			return false
		}

		// Val Len (4 bytes)
		binary.BigEndian.PutUint32(lenBuf[:], uint32(len(val)))
		if _, err := f.Write(lenBuf[:]); err != nil {
			writeErr = err
			return false
		}
		if _, err := f.Write(val); err != nil {
			writeErr = err
			return false
		}

		// ExpiresAt (8 bytes)
		var expBuf [8]byte
		binary.BigEndian.PutUint64(expBuf[:], uint64(expiresAt))
		if _, err := f.Write(expBuf[:]); err != nil {
			writeErr = err
			return false
		}

		return true
	})

	if writeErr != nil {
		f.Close()
		os.Remove(tmpPath)
		return writeErr
	}

	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	f.Close()

	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath)
		return err
	}

	return nil
}

// TriggerBGSave triggers an asynchronous background snapshot.
func (s *Snapshotter) TriggerBGSave() error {
	if !atomic.CompareAndSwapInt32(&s.inProgress, 0, 1) {
		return fmt.Errorf("background save already in progress")
	}

	go func() {
		defer atomic.StoreInt32(&s.inProgress, 0)
		_ = s.Save()
	}()

	return nil
}

// LoadSnapshot restores database state from snapshot file if it exists.
func LoadSnapshot(path string, engine *storage.Engine) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No snapshot file, fresh boot
		}
		return err
	}
	defer f.Close()

	header := make([]byte, len(MagicHeader))
	if _, err := io.ReadFull(f, header); err != nil {
		return fmt.Errorf("invalid snapshot header: %w", err)
	}
	if string(header) != string(MagicHeader) {
		return fmt.Errorf("corrupt snapshot header magic bytes")
	}

	var lenBuf [4]byte
	var expBuf [8]byte

	for {
		_, err := io.ReadFull(f, lenBuf[:])
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			return err
		}

		keyLen := binary.BigEndian.Uint32(lenBuf[:])
		keyBytes := make([]byte, keyLen)
		if _, err := io.ReadFull(f, keyBytes); err != nil {
			return err
		}

		if _, err := io.ReadFull(f, lenBuf[:]); err != nil {
			return err
		}
		valLen := binary.BigEndian.Uint32(lenBuf[:])
		valBytes := make([]byte, valLen)
		if _, err := io.ReadFull(f, valBytes); err != nil {
			return err
		}

		if _, err := io.ReadFull(f, expBuf[:]); err != nil {
			return err
		}
		expiresAt := int64(binary.BigEndian.Uint64(expBuf[:]))

		ttlMs := int64(0)
		if expiresAt > 0 {
			now := utils.FastNowUnixMilli()
			if expiresAt <= now {
				continue // Skip already expired keys
			}
			ttlMs = expiresAt - now
		}

		keyStr := utils.BytesToString(keyBytes)
		_ = engine.Set(keyStr, valBytes, ttlMs)
	}

	return nil
}
