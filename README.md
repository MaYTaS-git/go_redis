# Go-Redis: Ultra-Fast Standalone In-Memory Database & Cache

`go_redis` is a production-grade, zero-dependency, sharded in-memory key-value database built from scratch in Go. It is plug-and-play compatible with standard Redis clients (`redis-cli`, `ioredis`, `redis-py`, `go-redis`, `StackExchange.Redis`).

---

## ⚡ Features

- **RESP Protocol Compatibility:** Native support for standard Redis commands (`GET`, `SET`, `MGET`, `MSET`, `INCR`, `DECR`, `DEL`, `EXISTS`, `EXPIRE`, `TTL`, `PERSIST`, `KEYS`, `FLUSHDB`, `DBSIZE`, `PING`, `AUTH`, `INFO`, `BGSAVE`, `SHUTDOWN`).
- **64-Shard Lock Striping:** Memory is split across 64 independent partitions with cache-line padding to eliminate lock contention on multi-core systems.
- **Zero-Allocation Hot Path:** Fast zero-copy parsing using Go `unsafe` strings, reusable `bufio` readers, and pre-allocated byte slices.
- **Durability & Persistence:** Append-Only File (`AOF`) with configurable `fsync` policies (`always`, `everysec`, `no`) + background atomic snapshotting (`dump.db`).
- **Memory Limits & Eviction:** Configurable `max_memory_mb` limit supporting `allkeys-lru`, `volatile-lru`, `allkeys-lfu`, `volatile-lfu`, and `noeviction`.
- **Configurable Logging & Request Tracing:** Structured logging with configurable verbosity, log file output, and per-request origin IP / payload / duration tracing.
- **Interactive On-The-Fly Server Control:** Real-time terminal shortcuts to toggle request tracing, cycle log levels, clear cache, save snapshots, and exit cleanly (`Ctrl+C` or `q`).
- **Observability:** Prometheus HTTP `/metrics` endpoint on port 9090.
- **Security:** Password protection (`AUTH`) and native TLS encryption support.

---

## ⌨️ Interactive Terminal Shortcuts (On-The-Fly Controls)

While `go_redis.exe` is running, you can press any of the following shortcut keys in the terminal window followed by `Enter` to adapt server behavior dynamically **without modifying `config.txt` or restarting the server**:

| Shortcut Key | Action | Description |
| :--- | :--- | :--- |
| `l` + Enter | **Toggle Request Tracing** | Turns real-time request origin IP & payload logs ON or OFF instantly. |
| `v` + Enter | **Cycle Log Level** | Cycles log verbosity: `DEBUG` ➔ `INFO` ➔ `WARN` ➔ `ERROR`. |
| `f` + Enter | **Flush In-Memory Cache** | Clears all keys & values from memory (`FLUSHDB`). |
| `s` + Enter | **Save Snapshot** | Triggers an immediate background binary dump (`BGSAVE`). |
| `i` + Enter | **Show Telemetry Stats** | Displays live memory footprint, active key count, and hit/miss stats. |
| `h` + Enter | **Show Help Menu** | Displays the shortcut controls menu. |
| `q` + Enter | **Graceful Exit** | Flushes remaining buffers, saves snapshot, and exits cleanly (`Ctrl+C`). |

---

## 💾 Data Persistence & Server Restarts

• **Will data persist on restart?**
  - **If `aof_enabled true` or `snapshot_enabled true`:** Yes. Upon restart, cold boot recovery loads existing entries from `dump.db` and replays transactions from `appendonly.aof`.
  - **If `aof_enabled false` and `snapshot_enabled false`:** No. The server starts with a 100% fresh in-memory database on every boot (pure cache mode).

---

## ⚙️ Configuration Reference Guide (`config.txt`)

Go-Redis loads settings from `config.txt` in the executable or working directory. If missing, sensible production defaults are used automatically.

### 1. Networking Settings

| Option | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `port` | Integer | `6379` | TCP port for client connections. |
| `bind` | String | `"0.0.0.0"` | Network interface IP to listen on (`"0.0.0.0"` for all interfaces, `"127.0.0.1"` for local only). |

### 2. Security & Authentication

| Option | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `requirepass` | String | `""` | Require client authentication via `AUTH <password>`. Leave empty to disable authentication. |
| `tls_enabled` | Boolean | `false` | Enable TLS encryption on the main TCP port (`true` / `false`). |
| `tls_cert` | String | `""` | Path to TLS certificate file (e.g. `./cert.pem`). Required if `tls_enabled` is `true`. |
| `tls_key` | String | `""` | Path to TLS private key file (e.g. `./key.pem`). Required if `tls_enabled` is `true`. |

### 3. Memory & Eviction Management

| Option | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `max_memory_mb` | Integer | `512` | Maximum RAM allocated for database keys & values in Megabytes. |
| `eviction_policy` | String | `"allkeys-lru"` | Policy triggered when memory limit is exceeded. <br>• `allkeys-lru`: Evict least recently used key among all keys.<br>• `volatile-lru`: Evict least recently used key with expiration TTL.<br>• `allkeys-lfu`: Evict least frequently used key among all keys.<br>• `volatile-lfu`: Evict least frequently used key with TTL.<br>• `noeviction`: Return OOM error on new writes. |

### 4. Logging & Debugging

| Option | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `log_enabled` | Boolean | `true` | Enable or disable server logging completely (`true` / `false`). |
| `log_level` | String | `"info"` | Log verbosity filter: `debug`, `info`, `warn`, `error`. |
| `log_requests` | Boolean | `true` | Trace every client request with origin IP, command name, payload summary, and execution duration. <br>• **Dev / Debug:** Set `true` to inspect requests.<br>• **Max Throughput Benchmarks:** Set `false` to disable console I/O overhead. |
| `log_file` | String | `""` | File path to write logs (e.g. `./logs/server.log`). Leave empty to output directly to console `stdout`. |

### 5. Persistence & Durability

| Option | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `aof_enabled` | Boolean | `true` | Enable Append-Only File transaction logging (`true` / `false`). |
| `aof_fsync` | String | `"everysec"` | File sync policy:<br>• `everysec`: Background thread syncs file every second (balanced).<br>• `always`: Sync to disk after every write (maximum durability, lower write speed).<br>• `no`: OS manages disk sync (fastest write speed). |
| `aof_path` | String | `"./data/appendonly.aof"` | File path where AOF transactions are stored. |
| `snapshot_enabled` | Boolean | `true` | Enable atomic binary background database snapshots (`true` / `false`). |
| `snapshot_interval_sec` | Integer | `300` | Automated snapshot saving interval in seconds. |
| `snapshot_path` | String | `"./data/dump.db"` | File path for binary snapshot dump file. |

### 6. Telemetry & Observability

| Option | Type | Default | Description |
| :--- | :--- | :--- | :--- |
| `metrics_port` | Integer | `9090` | HTTP port serving Prometheus metrics at `http://localhost:9090/metrics`. Set `0` to disable. |

---

## 🎯 Configuration Presets

### Preset A: High-Performance Production (Maximum Throughput)
```text
port 6379
bind 0.0.0.0
max_memory_mb 2048
eviction_policy allkeys-lru
log_enabled true
log_level info
log_requests false
aof_enabled true
aof_fsync everysec
snapshot_enabled true
metrics_port 9090
```

### Preset B: Development & Feature Debugging (Full Request Tracing)
```text
port 6379
bind 127.0.0.1
max_memory_mb 512
eviction_policy allkeys-lru
log_enabled true
log_level debug
log_requests true
log_file ./logs/server.log
aof_enabled true
aof_fsync no
snapshot_enabled false
metrics_port 9090
```

---

## 🔍 Log Analysis & Internal Latency

When request logging is enabled (`log_requests true`), the server outputs real-time operational traces:

```text
2026/08/29 16:12:02.998 [REQ] origin=127.0.0.1:56579 cmd=SET payload=["bench_key_580", "value_580"] duration=0.000ms status=OK
2026/08/29 16:12:02.999 [REQ] origin=127.0.0.1:56579 cmd=GET payload=["bench_key_580"] duration=0.000ms status=OK
```

Notice that `duration=0.000ms` across requests confirms that **internal database memory operations run in under 1 microsecond**. Any perceived delay in high-volume sequential testing is due to network round-trips or console stdout printing.

---

## 🚀 Quick Start

### 1. Build Server
```bash
go build -ldflags="-s -w" -o go_redis.exe ./cmd/server
```

### 2. Run Server
```bash
./go_redis.exe
```
*Note: The server automatically creates the `./data` directory on initial startup.*

### 3. Run Benchmark (1,000 Operations)
```bash
go run ./cmd/benchmark
# Or via Node.js:
node benchmark.js
```
The benchmark will generate a full latency & ops/sec report saved to `performance.txt`.
