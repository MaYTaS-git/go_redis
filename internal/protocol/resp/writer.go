package resp

import (
	"bufio"
	"io"
	"strconv"
)

// Writer provides fast RESP response serialization.
type Writer struct {
	w *bufio.Writer
}

// NewWriter creates a new RESP Writer.
func NewWriter(w io.Writer) *Writer {
	return &Writer{
		w: bufio.NewWriterSize(w, 4096),
	}
}

// Flush flushes buffered data to the underlying stream.
func (w *Writer) Flush() error {
	return w.w.Flush()
}

// WriteOK writes +OK\r\n.
func (w *Writer) WriteOK() error {
	_, err := w.w.Write(OKResponse)
	return err
}

// WritePong writes +PONG\r\n.
func (w *Writer) WritePong() error {
	_, err := w.w.Write(PONGResponse)
	return err
}

// WriteSimpleString writes +<str>\r\n.
func (w *Writer) WriteSimpleString(s string) error {
	w.w.WriteByte(TypeSimpleString)
	w.w.WriteString(s)
	_, err := w.w.WriteString("\r\n")
	return err
}

// WriteError writes -<msg>\r\n.
func (w *Writer) WriteError(msg string) error {
	w.w.WriteByte(TypeError)
	w.w.WriteString(msg)
	_, err := w.w.WriteString("\r\n")
	return err
}

// WriteInt writes :<n>\r\n.
func (w *Writer) WriteInt(n int64) error {
	w.w.WriteByte(TypeInteger)
	var buf [32]byte
	b := strconv.AppendInt(buf[:0], n, 10)
	w.w.Write(b)
	_, err := w.w.WriteString("\r\n")
	return err
}

// WriteNil writes $-1\r\n.
func (w *Writer) WriteNil() error {
	_, err := w.w.Write(NullBulkResponse)
	return err
}

// WriteBulkBytes writes $<len>\r\n<b\r\n.
func (w *Writer) WriteBulkBytes(b []byte) error {
	if b == nil {
		return w.WriteNil()
	}
	w.w.WriteByte(TypeBulkString)
	var buf [32]byte
	lBuf := strconv.AppendInt(buf[:0], int64(len(b)), 10)
	w.w.Write(lBuf)
	w.w.WriteString("\r\n")
	w.w.Write(b)
	_, err := w.w.WriteString("\r\n")
	return err
}

// WriteBulkString writes $<len>\r\n<s>\r\n.
func (w *Writer) WriteBulkString(s string) error {
	w.w.WriteByte(TypeBulkString)
	var buf [32]byte
	lBuf := strconv.AppendInt(buf[:0], int64(len(s)), 10)
	w.w.Write(lBuf)
	w.w.WriteString("\r\n")
	w.w.WriteString(s)
	_, err := w.w.WriteString("\r\n")
	return err
}

// WriteArrayHeader writes *<len>\r\n.
func (w *Writer) WriteArrayHeader(length int) error {
	if length < 0 {
		_, err := w.w.Write(NullArrayResponse)
		return err
	}
	w.w.WriteByte(TypeArray)
	var buf [32]byte
	b := strconv.AppendInt(buf[:0], int64(length), 10)
	w.w.Write(b)
	_, err := w.w.WriteString("\r\n")
	return err
}

// WriteStringArray writes a array of bulk strings.
func (w *Writer) WriteStringArray(items []string) error {
	if items == nil {
		return w.WriteArrayHeader(-1)
	}
	if err := w.WriteArrayHeader(len(items)); err != nil {
		return err
	}
	for _, item := range items {
		if err := w.WriteBulkString(item); err != nil {
			return err
		}
	}
	return nil
}

// WriteBytesArray writes an array of byte slices.
func (w *Writer) WriteBytesArray(items [][]byte) error {
	if items == nil {
		return w.WriteArrayHeader(-1)
	}
	if err := w.WriteArrayHeader(len(items)); err != nil {
		return err
	}
	for _, item := range items {
		if err := w.WriteBulkBytes(item); err != nil {
			return err
		}
	}
	return nil
}
