package server

import (
	"fmt"
	"net/http"
	"sync/atomic"

	"go_redis/internal/storage"
)

// MetricsTracker holds atomic telemetry counters.
type MetricsTracker struct {
	ConnectedClients int64
	TotalCommands    int64
	Engine           *storage.Engine
}

// StartMetricsServer starts a lightweight HTTP Prometheus metrics listener.
func StartMetricsServer(addr string, tracker *MetricsTracker) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		hits, misses, usedMem, keysCount := tracker.Engine.Stats()
		totalCmds := atomic.LoadInt64(&tracker.TotalCommands)
		connClients := atomic.LoadInt64(&tracker.ConnectedClients)

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		fmt.Fprintf(w, "# HELP go_redis_connected_clients Current number of connected clients\n")
		fmt.Fprintf(w, "# TYPE go_redis_connected_clients gauge\n")
		fmt.Fprintf(w, "go_redis_connected_clients %d\n\n", connClients)

		fmt.Fprintf(w, "# HELP go_redis_commands_total Total processed commands\n")
		fmt.Fprintf(w, "# TYPE go_redis_commands_total counter\n")
		fmt.Fprintf(w, "go_redis_commands_total %d\n\n", totalCmds)

		fmt.Fprintf(w, "# HELP go_redis_keyspace_hits_total Total keyspace hits\n")
		fmt.Fprintf(w, "# TYPE go_redis_keyspace_hits_total counter\n")
		fmt.Fprintf(w, "go_redis_keyspace_hits_total %d\n\n", hits)

		fmt.Fprintf(w, "# HELP go_redis_keyspace_misses_total Total keyspace misses\n")
		fmt.Fprintf(w, "# TYPE go_redis_keyspace_misses_total counter\n")
		fmt.Fprintf(w, "go_redis_keyspace_misses_total %d\n\n", misses)

		fmt.Fprintf(w, "# HELP go_redis_used_memory_bytes Memory currently used in bytes\n")
		fmt.Fprintf(w, "# TYPE go_redis_used_memory_bytes gauge\n")
		fmt.Fprintf(w, "go_redis_used_memory_bytes %d\n\n", usedMem)

		fmt.Fprintf(w, "# HELP go_redis_total_keys Total active unexpired keys\n")
		fmt.Fprintf(w, "# TYPE go_redis_total_keys gauge\n")
		fmt.Fprintf(w, "go_redis_total_keys %d\n", keysCount)
	})

	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		_ = srv.ListenAndServe()
	}()

	return srv
}
