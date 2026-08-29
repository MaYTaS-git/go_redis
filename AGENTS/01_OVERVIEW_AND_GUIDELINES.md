# AGENTS System Overview & Executive Directive

## 1. Project Mission & Identity
This project is an ultra-fast, lightweight, production-grade in-memory key-value database and cache built in Go, designed as a zero-dependency, single-binary alternative to Redis. It is engineered for both instant local development and production deployments.

### Primary Objectives:
- **Zero Configuration Run:** Starts instantly as a standalone binary; automatically detects a `config.txt` in the binary directory if present or falls back to sensible production defaults.
- **Protocol Compatibility:** Speaks the standard Redis Serialization Protocol (RESP2/RESP3), making it plug-and-play compatible with existing Redis clients (`ioredis`, `redis-py`, `go-redis`, `StackExchange.Redis`, etc.).
- **Light-Speed Latency & Throughput:** Sub-microsecond internal read/write operations and ultra-low TCP latency via zero-allocation parsing, fine-grained lock striping (sharding), xxHash, and cache-line aligned structures.
- **Production Durability & Security:** Append-Only File (AOF) with configurable `fsync` policies, background snapshotting (RDB-style), `AUTH` password protection, TLS encryption, memory limits (`max_memory_mb`) with LRU/LFU eviction, and Prometheus metrics.

---

## 2. AGENTS Documentation Map

The `AGENTS/` directory controls the autonomous implementation of this project:

1. `AGENTS/01_OVERVIEW_AND_GUIDELINES.md` (This file)
   - High-level project identity, core goals, critical anti-patterns ("Mistakes to Avoid"), and workflow navigation.
2. `AGENTS/02_ARCHITECTURE_AND_SPECS.md`
   - Comprehensive technical specifications, modular system breakdowns, clean code rules, file structure, refactoring guidelines, and zero-allocation engineering rules.
3. `AGENTS/03_TASK_PROGRESS.md`
   - Dynamic task checklist tracking all phases, sub-tasks, completion states (`COMPLETED`, `IN_PROGRESS`, `PENDING`, `BLOCKED`, `ON_HOLD`), and acceptance criteria.
4. `AGENTS/04_AGENT_CHANGELOG.md`
   - Detailed audit log of every file created, modified, or deleted, including timestamps, file paths, rationale, and exact resumption state.

---

## 3. Critical Mistakes to Avoid (Anti-Patterns)

1. **DO NOT use standard `net/http` for data operations:**
   - RESP is a TCP-based binary-safe protocol. All caching connections must use standard TCP socket handlers (`net.Listen("tcp", ...)`) or event multiplexers, NOT HTTP. (HTTP is strictly reserved for the optional Prometheus metrics endpoint).
2. **DO NOT introduce a single global Mutex across the cache:**
   - A single `sync.RWMutex` over a `map[string]Item` causes high lock contention under multi-client loads. Always use striped sharding (e.g., 32–128 shards with independent mutexes).
3. **DO NOT cause heap allocations in the hot read/write path:**
   - Avoid `fmt.Sprintf`, `fmt.Fprintf`, string conversions `string(byteSlice)` in hot loops. Use `unsafe.String` / `unsafe.SliceData`, pre-allocated byte slices, and reusable `sync.Pool` buffers.
4. **DO NOT use CGO dependencies:**
   - The final binary must compile with `CGO_ENABLED=0` to guarantee static linking across all target platforms (Linux, macOS, Windows) with zero OS-level dynamic library dependencies.
5. **DO NOT execute synchronous disk writes in the client request loop:**
   - Snapshots (`BGSAVE`) and AOF flushing (`everysec`) must run asynchronously via background workers/channels so client latency is never blocked by disk I/O.
6. **DO NOT create monolithic single-file packages:**
   - Every file must adhere strictly to single responsibility. If any file exceeds ~250–350 lines, it must be modularized into sub-packages or helper components.

---

## 4. Operational Protocol for AI Agents

Whenever starting or resuming a session:
1. **Read `AGENTS/04_AGENT_CHANGELOG.md`** to see the exact state and where previous work stopped.
2. **Consult `AGENTS/03_TASK_PROGRESS.md`** to find the next active `IN_PROGRESS` or `PENDING` task.
3. **Implement code strictly adhering to `AGENTS/02_ARCHITECTURE_AND_SPECS.md`**.
4. **Update `AGENTS/03_TASK_PROGRESS.md`** marking the task completed or blocked.
5. **Log all changes with timestamps in `AGENTS/04_AGENT_CHANGELOG.md`** before completing the turn.
