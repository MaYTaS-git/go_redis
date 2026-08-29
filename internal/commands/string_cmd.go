package commands

import (
	"strconv"
	"strings"

	"go_redis/internal/server"
	"go_redis/pkg/utils"
)

// HandleGet processes GET key
func HandleGet(c *server.Client, args [][]byte) error {
	if len(args) != 1 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'get' command")
	}

	key := utils.BytesToString(args[0])
	val, found := c.Engine.Get(key)
	if !found {
		return c.Writer.WriteNil()
	}

	return c.Writer.WriteBulkBytes(val)
}

// HandleSet processes SET key value [EX seconds|PX milliseconds]
func HandleSet(c *server.Client, args [][]byte) error {
	if len(args) < 2 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'set' command")
	}

	key := utils.BytesToString(args[0])
	val := args[1]
	var ttlMs int64 = 0

	// Parse EX / PX options
	for i := 2; i < len(args); i++ {
		opt := strings.ToUpper(utils.BytesToString(args[i]))
		if opt == "EX" && i+1 < len(args) {
			sec, err := strconv.ParseInt(utils.BytesToString(args[i+1]), 10, 64)
			if err != nil || sec <= 0 {
				return c.Writer.WriteError("ERR invalid expire time in 'set' command")
			}
			ttlMs = sec * 1000
			i++
		} else if opt == "PX" && i+1 < len(args) {
			ms, err := strconv.ParseInt(utils.BytesToString(args[i+1]), 10, 64)
			if err != nil || ms <= 0 {
				return c.Writer.WriteError("ERR invalid expire time in 'set' command")
			}
			ttlMs = ms
			i++
		}
	}

	if err := c.Engine.Set(key, val, ttlMs); err != nil {
		return c.Writer.WriteError(err.Error())
	}

	return c.Writer.WriteOK()
}

// HandleMGet processes MGET key1 [key2 ...]
func HandleMGet(c *server.Client, args [][]byte) error {
	if len(args) < 1 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'mget' command")
	}

	if err := c.Writer.WriteArrayHeader(len(args)); err != nil {
		return err
	}

	for _, arg := range args {
		key := utils.BytesToString(arg)
		val, found := c.Engine.Get(key)
		if !found {
			if err := c.Writer.WriteNil(); err != nil {
				return err
			}
		} else {
			if err := c.Writer.WriteBulkBytes(val); err != nil {
				return err
			}
		}
	}
	return nil
}

// HandleMSet processes MSET key1 value1 [key2 value2 ...]
func HandleMSet(c *server.Client, args [][]byte) error {
	if len(args) < 2 || len(args)%2 != 0 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'mset' command")
	}

	for i := 0; i < len(args); i += 2 {
		key := utils.BytesToString(args[i])
		val := args[i+1]
		if err := c.Engine.Set(key, val, 0); err != nil {
			return c.Writer.WriteError(err.Error())
		}
	}

	return c.Writer.WriteOK()
}

// HandleIncr processes INCR key
func HandleIncr(c *server.Client, args [][]byte) error {
	if len(args) != 1 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'incr' command")
	}

	key := utils.BytesToString(args[0])
	val, err := c.Engine.IncrBy(key, 1)
	if err != nil {
		return c.Writer.WriteError(err.Error())
	}

	return c.Writer.WriteInt(val)
}

// HandleDecr processes DECR key
func HandleDecr(c *server.Client, args [][]byte) error {
	if len(args) != 1 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'decr' command")
	}

	key := utils.BytesToString(args[0])
	val, err := c.Engine.IncrBy(key, -1)
	if err != nil {
		return c.Writer.WriteError(err.Error())
	}

	return c.Writer.WriteInt(val)
}

// HandleDel processes DEL key1 [key2 ...]
func HandleDel(c *server.Client, args [][]byte) error {
	if len(args) < 1 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'del' command")
	}

	keys := make([]string, len(args))
	for i, arg := range args {
		keys[i] = utils.BytesToString(arg)
	}

	count := c.Engine.Del(keys...)
	return c.Writer.WriteInt(int64(count))
}

// HandleExists processes EXISTS key1 [key2 ...]
func HandleExists(c *server.Client, args [][]byte) error {
	if len(args) < 1 {
		return c.Writer.WriteError("ERR wrong number of arguments for 'exists' command")
	}

	keys := make([]string, len(args))
	for i, arg := range args {
		keys[i] = utils.BytesToString(arg)
	}

	count := c.Engine.Exists(keys...)
	return c.Writer.WriteInt(int64(count))
}
