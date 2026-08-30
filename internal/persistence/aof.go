package persistence

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"go_redis/internal/protocol/resp"
	"go_redis/internal/server"
	"go_redis/internal/storage"
	"go_redis/pkg/utils"
)

// AOF handles append-only file persistence and atomic recovery.
type AOF struct {
	file      *os.File
	writer    *bufio.Writer
	mu        sync.Mutex
	policy    string // "always", "everysec", "no"
	stopCh    chan struct{}
	wg        sync.WaitGroup
	writeChan chan []byte
}

// NewAOF initializes the Append-Only File logger.
// It opens the file once in RDWR mode, replays any existing commands to restore state,
// seeks to the end, and keeps the open descriptor for active append logging without re-opening.
func NewAOF(path string, policy string, engine *storage.Engine, router *server.Router) (*AOF, int, error) {
	if err := utils.EnsureDir(path); err != nil {
		return nil, 0, fmt.Errorf("failed to ensure directory for AOF: %w", err)
	}

	var file *os.File
	var err error
	for attempt := 0; attempt < 10; attempt++ {
		file, err = os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0666)
		if err == nil {
			break
		}
		if attempt < 9 {
			_ = utils.EnsureDir(path)
			time.Sleep(20 * time.Millisecond)
		}
	}
	if err != nil {
		return nil, 0, fmt.Errorf("failed to open AOF file %s: %w (ensure no other Go-Redis instance is running)", path, err)
	}

	replayedCount := 0

	// If engine and router are provided, replay existing AOF data directly from the open file
	if engine != nil && router != nil {
		_, _ = file.Seek(0, io.SeekStart)
		r := resp.NewReader(file)
		dummyConn := &dummyNetConn{}
		client := server.NewClient(dummyConn, engine, "")

		for {
			cmdArgs, err := r.ReadCommand()
			if err == io.EOF {
				break
			}
			if err != nil {
				break // Replayed up to end or clean EOF
			}
			if len(cmdArgs) == 0 {
				continue
			}

			_ = router.Dispatch(client, cmdArgs)
			replayedCount++
		}
	}

	// Seek to end of file for subsequent appends
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		_ = file.Close()
		return nil, 0, fmt.Errorf("failed to seek to end of AOF file: %w", err)
	}

	aof := &AOF{
		file:      file,
		writer:    bufio.NewWriterSize(file, 64*1024),
		policy:    policy,
		stopCh:    make(chan struct{}),
		writeChan: make(chan []byte, 10000),
	}

	aof.wg.Add(1)
	go aof.writeLoop()

	if policy == "everysec" {
		aof.wg.Add(1)
		go aof.fsyncLoop()
	}

	return aof, replayedCount, nil
}

// WriteCommand serializes a command to RESP format and queues it for AOF write.
func (a *AOF) WriteCommand(cmdName string, args ...[]byte) {
	var buf []byte
	// Format as RESP Array: *<len>\r\n$<cmdLen>\r\n<cmdName>\r\n...
	totalArgs := 1 + len(args)
	buf = append(buf, '*')
	buf = strconv.AppendInt(buf, int64(totalArgs), 10)
	buf = append(buf, "\r\n"...)

	buf = append(buf, '$')
	buf = strconv.AppendInt(buf, int64(len(cmdName)), 10)
	buf = append(buf, "\r\n"...)
	buf = append(buf, cmdName...)
	buf = append(buf, "\r\n"...)

	for _, arg := range args {
		buf = append(buf, '$')
		buf = strconv.AppendInt(buf, int64(len(arg)), 10)
		buf = append(buf, "\r\n"...)
		buf = append(buf, arg...)
		buf = append(buf, "\r\n"...)
	}

	a.writeChan <- buf
}

func (a *AOF) writeLoop() {
	defer a.wg.Done()
	for {
		select {
		case data := <-a.writeChan:
			a.mu.Lock()
			a.writer.Write(data)
			// Drain buffered writes
			for len(a.writeChan) > 0 {
				moreData := <-a.writeChan
				a.writer.Write(moreData)
			}
			a.writer.Flush()

			if a.policy == "always" {
				a.file.Sync()
			}
			a.mu.Unlock()

		case <-a.stopCh:
			a.mu.Lock()
			for len(a.writeChan) > 0 {
				data := <-a.writeChan
				a.writer.Write(data)
			}
			a.writer.Flush()
			a.file.Sync()
			a.mu.Unlock()
			return
		}
	}
}

func (a *AOF) fsyncLoop() {
	defer a.wg.Done()
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.stopCh:
			return
		case <-ticker.C:
			a.mu.Lock()
			a.file.Sync()
			a.mu.Unlock()
		}
	}
}

// Close flushes and closes the AOF file.
func (a *AOF) Close() error {
	close(a.stopCh)
	a.wg.Wait()
	return a.file.Close()
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
