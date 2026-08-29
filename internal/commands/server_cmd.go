package commands

import (
	"fmt"
	"runtime"
	"strings"

	"go_redis/internal/server"
	"go_redis/pkg/utils"
)

// HandlePing processes PING [message]
func HandlePing(c *server.Client, args [][]byte) error {
	if len(args) == 0 {
		return c.Writer.WritePong()
	}
	return c.Writer.WriteBulkBytes(args[0])
}

// HandleAuth processes AUTH password
func HandleAuth(c *server.Client, args [][]byte) error {
	if len(args) != 1 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'auth' command")
	}

	if c.RequirePass == "" {
		return c.Writer.WriteError("ERR Client sent AUTH, but no password is set")
	}

	pass := utils.BytesToString(args[0])
	if pass != c.RequirePass {
		return c.Writer.WriteError("WRONGPASS Invalid password")
	}

	c.Authenticated = true
	return c.Writer.WriteOK()
}

// HandleInfo processes INFO [section]
func HandleInfo(c *server.Client, args [][]byte) error {
	hits, misses, usedMem, keysCount := c.Engine.Stats()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	infoStr := fmt.Sprintf(
		"# Server\r\n"+
			"redis_version:7.0.0-go-redis\r\n"+
			"os:%s\r\n"+
			"arch:%s\r\n"+
			"\r\n"+
			"# Memory\r\n"+
			"used_memory:%d\r\n"+
			"used_memory_human:%.2fMB\r\n"+
			"heap_sys:%d\r\n"+
			"\r\n"+
			"# Stats\r\n"+
			"keyspace_hits:%d\r\n"+
			"keyspace_misses:%d\r\n"+
			"total_keys:%d\r\n",
		runtime.GOOS,
		runtime.GOARCH,
		usedMem,
		float64(usedMem)/(1024*1024),
		m.HeapSys,
		hits,
		misses,
		keysCount,
	)

	return c.Writer.WriteBulkString(infoStr)
}

// HandleBGSave processes BGSAVE
func HandleBGSave(c *server.Client, args [][]byte) error {
	type BGSaver interface {
		TriggerBGSave() error
	}

	if saver, ok := c.Persistence.(BGSaver); ok {
		if err := saver.TriggerBGSave(); err != nil {
			return c.Writer.WriteError(fmt.Sprintf("ERR %v", err))
		}
		return c.Writer.WriteSimpleString("Background saving started")
	}

	return c.Writer.WriteSimpleString("Background saving started")
}

// HandleShutdown processes SHUTDOWN
func HandleShutdown(c *server.Client, args [][]byte) error {
	type Downer interface {
		Shutdown()
	}

	if d, ok := c.Persistence.(Downer); ok {
		d.Shutdown()
	}

	_ = c.Writer.WriteOK()
	_ = c.Writer.Flush()
	_ = c.Conn.Close()
	return nil
}

// IsAuthRequired returns true if client needs authentication for the command.
func IsAuthRequired(c *server.Client, cmdName string) bool {
	if c.RequirePass == "" || c.Authenticated {
		return false
	}

	cmdUpper := strings.ToUpper(cmdName)
	return cmdUpper != "AUTH" && cmdUpper != "PING"
}
