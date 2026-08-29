package resp

import (
	"bytes"
	"reflect"
	"testing"
)

func TestRESPReader_ReadCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    [][]byte
		wantErr bool
	}{
		{
			name:    "Standard RESP Array PING",
			input:   "*1\r\n$4\r\nPING\r\n",
			want:    [][]byte{[]byte("PING")},
			wantErr: false,
		},
		{
			name:    "Standard RESP Array SET foo bar",
			input:   "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n",
			want:    [][]byte{[]byte("SET"), []byte("foo"), []byte("bar")},
			wantErr: false,
		},
		{
			name:    "Inline command PING",
			input:   "PING\r\n",
			want:    [][]byte{[]byte("PING")},
			wantErr: false,
		},
		{
			name:    "Inline command SET k v",
			input:   "SET k v\r\n",
			want:    [][]byte{[]byte("SET"), []byte("k"), []byte("v")},
			wantErr: false,
		},
		{
			name:    "Empty array",
			input:   "*0\r\n",
			want:    [][]byte{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewReader(bytes.NewBufferString(tt.input))
			got, err := r.ReadCommand()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ReadCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ReadCommand() got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRESPWriter(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	if err := w.WriteOK(); err != nil {
		t.Fatalf("WriteOK failed: %v", err)
	}
	if err := w.WritePong(); err != nil {
		t.Fatalf("WritePong failed: %v", err)
	}
	if err := w.WriteInt(100); err != nil {
		t.Fatalf("WriteInt failed: %v", err)
	}
	if err := w.WriteBulkString("hello"); err != nil {
		t.Fatalf("WriteBulkString failed: %v", err)
	}
	if err := w.WriteNil(); err != nil {
		t.Fatalf("WriteNil failed: %v", err)
	}
	if err := w.WriteStringArray([]string{"a", "b"}); err != nil {
		t.Fatalf("WriteStringArray failed: %v", err)
	}

	if err := w.Flush(); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	expected := "+OK\r\n+PONG\r\n:100\r\n$5\r\nhello\r\n$-1\r\n*2\r\n$1\r\na\r\n$1\r\nb\r\n"
	if buf.String() != expected {
		t.Errorf("Writer got %q, want %q", buf.String(), expected)
	}
}
