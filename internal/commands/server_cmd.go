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

// HandleSelect processes SELECT index
func HandleSelect(c *server.Client, args [][]byte) error {
	if len(args) != 1 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'select' command")
	}
	return c.Writer.WriteOK()
}

// HandleClient processes CLIENT [SETINFO|SETNAME|GETNAME|LIST|KILL...]
func HandleClient(c *server.Client, args [][]byte) error {
	if len(args) == 0 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'client' command")
	}
	subCmd := strings.ToUpper(utils.BytesToString(args[0]))
	switch subCmd {
	case "SETINFO", "SETNAME":
		return c.Writer.WriteOK()
	case "GETNAME":
		return c.Writer.WriteBulkString("")
	case "LIST":
		return c.Writer.WriteBulkString("id=1 addr=127.0.0.1 fd=1 name= age=1 idle=0 flags=N db=0 sub=0 psub=0 multi=-1 qbuf=0 qbuf-free=0 obl=0 oll=0 omem=0 events=r cmd=client\n")
	default:
		return c.Writer.WriteOK()
	}
}

// HandleCommand processes COMMAND [DOCS|COUNT|LIST|INFO]
func HandleCommand(c *server.Client, args [][]byte) error {
	return c.Writer.WriteArrayHeader(0)
}

// HandleConfig processes CONFIG [GET|SET|RESETSTAT]
func HandleConfig(c *server.Client, args [][]byte) error {
	if len(args) > 0 && strings.ToUpper(utils.BytesToString(args[0])) == "GET" {
		return c.Writer.WriteArrayHeader(0)
	}
	return c.Writer.WriteOK()
}

// HandleEcho processes ECHO message
func HandleEcho(c *server.Client, args [][]byte) error {
	if len(args) != 1 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'echo' command")
	}
	return c.Writer.WriteBulkBytes(args[0])
}

// HandleHello processes HELLO [protover [AUTH username password] [SETNAME name]]
func HandleHello(c *server.Client, args [][]byte) error {
	return c.Writer.WriteBulkString("proto:2 server:go-redis version:7.0.0 mode:standalone")
}

// IsAuthRequired returns true if client needs authentication for the command.
func IsAuthRequired(c *server.Client, cmdName string) bool {
	if c.RequirePass == "" || c.Authenticated {
		return false
	}

	cmdUpper := strings.ToUpper(cmdName)
	return cmdUpper != "AUTH" && cmdUpper != "PING" && cmdUpper != "HELLO"
}
