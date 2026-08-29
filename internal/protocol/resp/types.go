package resp

import (
	"errors"
)

// RESP Protocol frame type prefixes
const (
	TypeSimpleString = '+'
	TypeError        = '-'
	TypeInteger      = ':'
	TypeBulkString   = '$'
	TypeArray        = '*'
)

// Pre-defined errors
var (
	ErrInvalidSyntax   = errors.New("ERR invalid RESP syntax")
	ErrBulkStringLimit = errors.New("ERR bulk string exceeds size limit")
	ErrArrayLimit      = errors.New("ERR array length exceeds limit")
	ErrNilValue        = errors.New("ERR nil value encountered")
)

// Common RESP static byte responses
var (
	OKResponse        = []byte("+OK\r\n")
	PONGResponse      = []byte("+PONG\r\n")
	NullBulkResponse  = []byte("$-1\r\n")
	NullArrayResponse = []byte("*-1\r\n")
	EmptyArrayResponse = []byte("*0\r\n")
	ZeroIntResponse   = []byte(":0\r\n")
	OneIntResponse    = []byte(":1\r\n")
)

// Command represents a parsed RESP Command (Array of Bulk Strings)
type Command struct {
	Name string
	Args [][]byte
}
