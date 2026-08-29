package commands

import (
	"strconv"

	"go_redis/internal/server"
	"go_redis/pkg/utils"
)

// HandleExpire processes EXPIRE key seconds
func HandleExpire(c *server.Client, args [][]byte) error {
	if len(args) != 2 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'expire' command")
	}

	key := utils.BytesToString(args[0])
	sec, err := strconv.ParseInt(utils.BytesToString(args[1]), 10, 64)
	if err != nil {
		return c.Writer.WriteError("ERR value is not an integer or out of range")
	}

	if c.Engine.Expire(key, sec*1000) {
		return c.Writer.WriteInt(1)
	}
	return c.Writer.WriteInt(0)
}

// HandleTTL processes TTL key
func HandleTTL(c *server.Client, args [][]byte) error {
	if len(args) != 1 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'ttl' command")
	}

	key := utils.BytesToString(args[0])
	ttlMs, exists := c.Engine.TTL(key)
	if !exists {
		return c.Writer.WriteInt(-2)
	}
	if ttlMs < 0 {
		return c.Writer.WriteInt(-1)
	}

	// Convert ms to seconds
	ttlSec := ttlMs / 1000
	if ttlSec == 0 && ttlMs > 0 {
		ttlSec = 1
	}

	return c.Writer.WriteInt(ttlSec)
}

// HandlePersist processes PERSIST key
func HandlePersist(c *server.Client, args [][]byte) error {
	if len(args) != 1 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'persist' command")
	}

	key := utils.BytesToString(args[0])
	if c.Engine.Persist(key) {
		return c.Writer.WriteInt(1)
	}
	return c.Writer.WriteInt(0)
}

// HandleKeys processes KEYS pattern
func HandleKeys(c *server.Client, args [][]byte) error {
	if len(args) != 1 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'keys' command")
	}

	pattern := utils.BytesToString(args[0])
	matchedKeys := c.Engine.Keys(pattern)

	return c.Writer.WriteStringArray(matchedKeys)
}

// HandleFlushDB processes FLUSHDB
func HandleFlushDB(c *server.Client, args [][]byte) error {
	c.Engine.FlushDB()
	return c.Writer.WriteOK()
}

// HandleDBSize processes DBSIZE
func HandleDBSize(c *server.Client, args [][]byte) error {
	size := c.Engine.DBSize()
	return c.Writer.WriteInt(size)
}
