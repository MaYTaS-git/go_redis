package utils

import (
	"testing"
)

func TestBytesToString(t *testing.T) {
	tests := []struct {
		input []byte
		want  string
	}{
		{[]byte("hello"), "hello"},
		{[]byte(""), ""},
		{nil, ""},
		{[]byte("redis_cache_test"), "redis_cache_test"},
	}

	for _, tt := range tests {
		got := BytesToString(tt.input)
		if got != tt.want {
			t.Errorf("BytesToString(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func TestStringToBytes(t *testing.T) {
	tests := []struct {
		input string
		want  []byte
	}{
		{"hello", []byte("hello")},
		{"", nil},
		{"redis_cache_test", []byte("redis_cache_test")},
	}

	for _, tt := range tests {
		got := StringToBytes(tt.input)
		if string(got) != string(tt.want) {
			t.Errorf("StringToBytes(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}
