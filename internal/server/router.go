package server

import (
	"fmt"
	"strings"

	"go_redis/pkg/utils"
)

// CommandHandler defines the standard signature for all Redis command handlers.
type CommandHandler func(c *Client, args [][]byte) error

// Router dispatches incoming commands to their registered handlers.
type Router struct {
	handlers map[string]CommandHandler
}

// NewRouter initializes a command router.
func NewRouter() *Router {
	return &Router{
		handlers: make(map[string]CommandHandler),
	}
}

// Register maps a command string (e.g. "GET") to a CommandHandler.
func (r *Router) Register(name string, handler CommandHandler) {
	r.handlers[strings.ToUpper(name)] = handler
}

// Dispatch executes the appropriate handler for the received command array.
func (r *Router) Dispatch(c *Client, cmdArgs [][]byte) error {
	if len(cmdArgs) == 0 {
		return nil
	}

	cmdName := strings.ToUpper(utils.BytesToString(cmdArgs[0]))
	args := cmdArgs[1:]

	// Authentication check
	if c.RequirePass != "" && !c.Authenticated && cmdName != "AUTH" && cmdName != "PING" {
		return c.Writer.WriteError("NOAUTH Authentication required.")
	}

	handler, exists := r.handlers[cmdName]
	if !exists {
		return c.Writer.WriteError(fmt.Sprintf("ERR unknown command '%s'", cmdName))
	}

	return handler(c, args)
}
