package commands

import (
	"net"
	"testing"

	"go_redis/internal/server"
	"go_redis/internal/storage"
)

func TestCommandDispatch(t *testing.T) {
	eng := storage.NewEngine(10, "allkeys-lru")
	defer eng.Close()

	router := server.NewRouter()
	router.Register("PING", HandlePing)
	router.Register("SET", HandleSet)
	router.Register("GET", HandleGet)
	router.Register("DEL", HandleDel)
	router.Register("EXISTS", HandleExists)
	router.Register("INCR", HandleIncr)
	router.Register("DECR", HandleDecr)

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	c := server.NewClient(c1, eng, "")

	// Test SET & GET via Engine directly with router
	setArgs := [][]byte{[]byte("SET"), []byte("mykey"), []byte("myval")}
	getArgs := [][]byte{[]byte("GET"), []byte("mykey")}

	err := router.Dispatch(c, setArgs)
	if err != nil {
		t.Fatalf("SET dispatch failed: %v", err)
	}

	val, found := eng.Get("mykey")
	if !found || string(val) != "myval" {
		t.Errorf("expected myval, got %q", val)
	}

	err = router.Dispatch(c, getArgs)
	if err != nil {
		t.Fatalf("GET dispatch failed: %v", err)
	}
}

func TestAuth(t *testing.T) {
	eng := storage.NewEngine(10, "allkeys-lru")
	defer eng.Close()

	router := server.NewRouter()
	router.Register("PING", HandlePing)
	router.Register("AUTH", HandleAuth)
	router.Register("GET", HandleGet)

	c1, c2 := net.Pipe()
	defer c1.Close()
	defer c2.Close()

	c := server.NewClient(c1, eng, "secret123")

	// PING before auth should succeed
	if err := router.Dispatch(c, [][]byte{[]byte("PING")}); err != nil {
		t.Errorf("PING before AUTH failed: %v", err)
	}

	if c.Authenticated {
		t.Errorf("client should not be authenticated initially")
	}

	// AUTH with wrong password
	authWrong := [][]byte{[]byte("AUTH"), []byte("wrongpass")}
	_ = router.Dispatch(c, authWrong)
	if c.Authenticated {
		t.Errorf("client should not be authenticated with wrong pass")
	}

	// AUTH with correct password
	authRight := [][]byte{[]byte("AUTH"), []byte("secret123")}
	_ = router.Dispatch(c, authRight)
	if !c.Authenticated {
		t.Errorf("client should be authenticated after correct pass")
	}
}

func TestKeyCommands(t *testing.T) {
	eng := storage.NewEngine(10, "allkeys-lru")
	defer eng.Close()

	eng.Set("k1", []byte("v1"), 0)
	eng.Set("k2", []byte("v2"), 0)

	if eng.DBSize() != 2 {
		t.Errorf("expected DBSize 2, got %d", eng.DBSize())
	}

	keys := eng.Keys("*")
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}

	eng.FlushDB()
	if eng.DBSize() != 0 {
		t.Errorf("expected DBSize 0 after flush, got %d", eng.DBSize())
	}
}
