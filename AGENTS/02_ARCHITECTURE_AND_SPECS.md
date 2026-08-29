# Detailed Technical Architecture, Implementation Specifications & Coding Rules

---

## 1. Project Directory Structure

The project must strictly adhere to the standard Go layout:

```text
.
├── AGENTS/
│   ├── 01_OVERVIEW_AND_GUIDELINES.md
│   ├── 02_ARCHITECTURE_AND_SPECS.md
│   ├── 03_TASK_PROGRESS.md
│   └── 04_AGENT_CHANGELOG.md
├── cmd/
│   └── server/
│       └── main.go                 # Entrypoint, signal handling, graceful shutdown
├── config.txt                      # Default/sample configuration file
├── internal/
│   ├── config/
│   │   ├── config.go               # Config struct, defaults, and config.txt parser
│   │   └── config_test.go
│   ├── protocol/
│   │   ├── resp/
│   │   │   ├── reader.go           # Fast zero-alloc RESP parser
│   │   │   ├── writer.go           # Pre-formatted byte response writers
│   │   │   ├── types.go            # RESP frame types (+, -, :, $, *)
│   │   │   └── resp_test.go
│   ├── storage/
│   │   ├── shard.go                # Shard struct, RWMutex, cache-line padding
│   │   ├── engine.go               # Top-level storage engine, hash routing
│   │   ├── item.go                 # Item payload, TTL metadata
│   │   ├── hasher.go               # Fast xxHash/FNV implementation
│   │   ├── eviction/
│   │   │   ├── lru.go              # Least Recently Used eviction policy
│   │   │   ├── lfu.go              # Least Frequently Used eviction policy
│   │   │   └── noeviction.go       # Return error on out-of-memory
│   │   └── storage_test.go
│   ├── persistence/
│   │   ├── aof.go                  # Append-Only File writer and fsync loop
│   │   ├── snapshot.go             # Binary snapshot generator and loader
│   │   ├── replayer.go             # Cold recovery parser on startup
│   │   └── persistence_test.go
│   ├── server/
│   │   ├── tcp.go                  # TCP listener, connection dispatcher
│   │   ├── client.go               # Per-connection session state & auth flags
│   │   ├── router.go               # Command router (PING, GET, SET, DEL, etc.)
│   │   ├── metrics.go              # Prometheus /metrics endpoint (HTTP)
│   │   └── tls.go                  # TLS certificate loader and listener setup
│   └── commands/
│       ├── string_cmd.go           # GET, SET, MGET, MSET, INCR, DECR, DEL, EXISTS
│       ├── key_cmd.go              # EXPIRE, TTL, PERSIST, KEYS, FLUSHDB
│       ├── server_cmd.go           # INFO, PING, AUTH, BGSAVE, SHUTDOWN
│       └── commands_test.go
├── pkg/
│   └── utils/
│       ├── byteconv.go             # Zero-copy unsafe string <-> byte slice utilities
│       ├── time.go                 # Monotonic fast time helpers
│       └── fileutil.go             # Atomic file writers (rename after temp write)
├── go.mod
├── go.sum
├── Makefile                        # Multi-platform build targets
└── README.md                       # User documentation
```

---

## 2. Detailed Technical Breakdown (6 Modules)

### Module A: Configuration Engine (`internal/config`)
* **Behavior:** Look for `config.txt` in the executable directory (`os.Executable()`). If missing, search `./config.txt` or use hardcoded production-ready defaults.
* **Fields:**
  - `port` (int, default: 6379)
  - `bind` (string, default: "0.0.0.0")
  - `requirepass` (string, default: "")
  - `tls_enabled` (bool, default: false)
  - `tls_cert` (string), `tls_key` (string)
  - `max_memory_mb` (int64, default: 512)
  - `eviction_policy` (string, default: "allkeys-lru")
  - `aof_enabled` (bool, default: true)
  - `aof_fsync` (string: "always", "everysec", "no", default: "everysec")
  - `aof_path` (string, default: "./data/appendonly.aof")
  - `snapshot_enabled` (bool, default: true)
  - `snapshot_interval_sec` (int, default: 300)
  - `snapshot_path` (string, default: "./data/dump.db")
  - `metrics_port` (int, default: 9090)

### Module B: Zero-Allocation RESP Protocol (`internal/protocol/resp`)
* **Parser:** Read RESP byte-by-byte using a reusable buffer (`bufio.Reader`).
* **Frame Support:** Simple Strings (`+`), Errors (`-`), Integers (`:`), Bulk Strings (`$`), Arrays (`*`).
* **Writer:** Provide pre-baked static response byte slices:
  - `+OK
`, `+PONG
`, `$-1
` (nil), `:0
`, `:1
`.
  - Number formatters using `strconv.AppendInt` on stack slices to avoid heap escapes.

### Module C: Sharded In-Memory Storage Engine (`internal/storage`)
* **Sharding:** 64 shards partitioned using 64-bit `xxHash(key) & 63`.
* **Lock Striping:** Each shard has its own `sync.RWMutex`.
* **Memory Tracking:** Atomic integer counting exact bytes used (keys + values + metadata struct overhead).
* **Eviction Worker:** When memory reaches `max_memory_mb`, invoke the active eviction policy (LRU / LFU / volatile-lru).
* **Lazy & Active Expiration:**
  - *Lazy:* On key access, check `ExpiresAt`. If expired, delete and return nil.
  - *Active:* Background ticker every 100ms tests 20 random keys per shard.

### Module D: Command Router & Handlers (`internal/commands`)
* Standardize all command handlers to the signature:
  `type Handler func(c *server.Client, args [][]byte) error`
* Commands to implement:
  - **Connection/Admin:** `PING`, `AUTH`, `INFO`, `SHUTDOWN`, `BGSAVE`
  - **Key Operations:** `GET`, `SET [EX seconds|PX ms]`, `MGET`, `MSET`, `DEL`, `EXISTS`, `INCR`, `DECR`, `TTL`, `EXPIRE`, `PERSIST`, `FLUSHDB`, `DBSIZE`

### Module E: Durability & Persistence (`internal/persistence`)
* **AOF:** Append mutation commands to a buffered file channel. If `aof_fsync=everysec`, run a dedicated background ticker to call `file.Sync()`.
* **Snapshots:** Execute `BGSAVE` asynchronously. Iterate all shards, serialize keys and TTLs to a temporary file (`dump.db.tmp`), sync to disk, and atomic rename to `dump.db`.
* **Replayer:** On binary boot, first load `dump.db` (if exists), then replay `appendonly.aof` to bring memory to latest state.

### Module F: Server, Security & Telemetry (`internal/server`)
* **Networking:** TCP listener dispatching `go handleConnection(conn)`.
* **Auth Check:** If `requirepass` is set, block all commands except `AUTH` and `PING` until authenticated.
* **Observability:** Prometheus HTTP listener on `metrics_port` exposing total commands, memory usage, connected clients, hit/miss counter.
* **Signal Handling:** Catch `SIGINT` / `SIGTERM`, stop TCP listener, sync AOF file, flush snapshots, and terminate cleanly.

---

## 3. Strict Coding Rules & Best Practices

1. **DRY (Don't Repeat Yourself):**
   - Centralize byte conversion, file I/O, error formatting, and number parsing in reusable utility functions.
2. **File Size & Modularity Constraint:**
   - **No file should exceed ~300 lines.** If a file grows larger, break it down:
     - Example: If `router.go` handles too many commands, split command handlers into `string_cmd.go`, `key_cmd.go`, and `server_cmd.go`.
3. **Reusable Functions over Inline Duplication:**
   - Never write repetitive byte formatting or lock acquisitions. Abstract them into clear helper methods with descriptive names.
4. **Zero-Allocation Guidelines:**
   - Use `unsafe.String` / `unsafe.SliceData` for string conversions where read-only semantics apply.
   - Use stack-allocated byte arrays or `sync.Pool` for buffers.
   - Avoid `interface{}` in hot paths; use concrete types and byte slices `[]byte`.
5. **Thread Safety & Race Detection:**
   - Always run tests with `go test -race ./...`.
   - Never access shard maps outside their respective shard mutex lock.
