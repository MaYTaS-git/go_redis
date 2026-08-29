package persistence

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"go_redis/pkg/utils"
)

// AOF handles append-only file persistence.
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
func NewAOF(path string, policy string) (*AOF, error) {
	if err := utils.EnsureDir(path); err != nil {
		return nil, fmt.Errorf("failed to ensure directory for AOF: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open AOF file %s: %w", path, err)
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

	return aof, nil
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
