package resp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
)

// Reader is a fast RESP protocol reader.
type Reader struct {
	rd *bufio.Reader
}

// NewReader returns a new RESP Reader wrapping an io.Reader.
func NewReader(rd io.Reader) *Reader {
	if b, ok := rd.(*bufio.Reader); ok {
		return &Reader{rd: b}
	}
	return &Reader{
		rd: bufio.NewReaderSize(rd, 4096),
	}
}

// Read implements io.Reader interface.
func (r *Reader) Read(p []byte) (n int, err error) {
	return r.rd.Read(p)
}

// ReadCommand reads a single command array from the client stream.
// It returns a slice of byte slices representing the command and its arguments.
// Supports both standard RESP arrays (*N\r\n...) and inline text format.
func (r *Reader) ReadCommand() ([][]byte, error) {
	b, err := r.rd.Peek(1)
	if err != nil {
		return nil, err
	}

	if b[0] == TypeArray {
		return r.readArrayCommand()
	}

	// Fallback: Inline commands (e.g. "PING\r\n" or "SET foo bar\r\n")
	return r.readInlineCommand()
}

func (r *Reader) readArrayCommand() ([][]byte, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	if len(line) < 3 || line[0] != TypeArray {
		return nil, ErrInvalidSyntax
	}

	numArgs, err := strconv.Atoi(string(line[1 : len(line)-2]))
	if err != nil || numArgs < 0 {
		return nil, ErrInvalidSyntax
	}

	if numArgs == 0 {
		return [][]byte{}, nil
	}

	args := make([][]byte, numArgs)
	for i := 0; i < numArgs; i++ {
		arg, err := r.readBulkString()
		if err != nil {
			return nil, err
		}
		args[i] = arg
	}

	return args, nil
}

func (r *Reader) readBulkString() ([]byte, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}
	if len(line) < 3 || line[0] != TypeBulkString {
		return nil, ErrInvalidSyntax
	}

	strLen, err := strconv.Atoi(string(line[1 : len(line)-2]))
	if err != nil {
		return nil, ErrInvalidSyntax
	}

	if strLen == -1 {
		return nil, nil // Nil bulk string
	}
	if strLen < -1 {
		return nil, ErrInvalidSyntax
	}

	buf := make([]byte, strLen+2)
	_, err = io.ReadFull(r.rd, buf)
	if err != nil {
		return nil, err
	}

	if buf[strLen] != '\r' || buf[strLen+1] != '\n' {
		return nil, ErrInvalidSyntax
	}

	return buf[:strLen], nil
}

func (r *Reader) readInlineCommand() ([][]byte, error) {
	line, err := r.readLine()
	if err != nil {
		return nil, err
	}

	trimmed := bytes.TrimSuffix(line, []byte("\r\n"))
	trimmed = bytes.TrimSuffix(trimmed, []byte("\n"))

	parts := bytes.Fields(trimmed)
	if len(parts) == 0 {
		return nil, ErrInvalidSyntax
	}

	return parts, nil
}

func (r *Reader) readLine() ([]byte, error) {
	line, err := r.rd.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	if !bytes.HasSuffix(line, []byte("\r\n")) {
		// allow plain \n
		if bytes.HasSuffix(line, []byte("\n")) {
			return line, nil
		}
		return nil, fmt.Errorf("line does not end with CR-LF")
	}
	return line, nil
}
