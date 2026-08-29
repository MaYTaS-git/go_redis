package persistence

import (
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"go_redis/internal/protocol/resp"
	"go_redis/internal/server"
	"go_redis/internal/storage"
)

// ColdRecovery loads the binary snapshot (if present) and replays the AOF (if present).
func ColdRecovery(snapshotPath string, aofPath string, engine *storage.Engine, router *server.Router) error {
	// 1. Load Snapshot if present
	if err := LoadSnapshot(snapshotPath, engine); err != nil {
		return fmt.Errorf("failed during snapshot recovery: %w", err)
	}

	// 2. Replay AOF if present
	if aofPath == "" {
		return nil
	}

	f, err := os.Open(aofPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Fresh start
		}
		return fmt.Errorf("failed to open AOF file %s: %w", aofPath, err)
	}
	defer f.Close()

	r := resp.NewReader(f)
	dummyConn := &dummyNetConn{}
	client := server.NewClient(dummyConn, engine, "")

	for {
		cmdArgs, err := r.ReadCommand()
		if err == io.EOF {
			break
		}
		if err != nil {
			break // Replayed up to end or corruption
		}

		if len(cmdArgs) == 0 {
			continue
		}

		if router != nil {
			_ = router.Dispatch(client, cmdArgs)
		}
	}

	return nil
}

type dummyNetConn struct{}

func (d *dummyNetConn) Read(b []byte) (n int, err error)   { return 0, io.EOF }
func (d *dummyNetConn) Write(b []byte) (n int, err error)  { return len(b), nil }
func (d *dummyNetConn) Close() error                       { return nil }
func (d *dummyNetConn) LocalAddr() net.Addr                { return nil }
func (d *dummyNetConn) RemoteAddr() net.Addr               { return nil }
func (d *dummyNetConn) SetDeadline(t time.Time) error      { return nil }
func (d *dummyNetConn) SetReadDeadline(t time.Time) error  { return nil }
func (d *dummyNetConn) SetWriteDeadline(t time.Time) error { return nil }
