package server

import (
	"net"

	"go_redis/internal/protocol/resp"
	"go_redis/internal/storage"
)

// Client represents an active client TCP session.
type Client struct {
	Conn          net.Conn
	Reader        *resp.Reader
	Writer        *resp.Writer
	Authenticated bool
	RequirePass   string
	Engine        *storage.Engine
	Persistence   interface{} // AOF / Snapshotter interface
}

// NewClient creates a new Client instance wrapping a net.Conn.
func NewClient(conn net.Conn, engine *storage.Engine, requirePass string) *Client {
	return &Client{
		Conn:          conn,
		Reader:        resp.NewReader(conn),
		Writer:        resp.NewWriter(conn),
		Authenticated: requirePass == "",
		RequirePass:   requirePass,
		Engine:        engine,
	}
}

// Flush flushes buffered responses to the network connection.
func (c *Client) Flush() error {
	return c.Writer.Flush()
}
