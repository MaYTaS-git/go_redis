package server

import (
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go_redis/internal/config"
	"go_redis/internal/storage"
	"go_redis/pkg/logger"
	"go_redis/pkg/utils"
)

// AOFLogger is an interface for writing mutating commands to AOF.
type AOFLogger interface {
	WriteCommand(cmdName string, args ...[]byte)
}

// Server represents the core TCP server instance.
type Server struct {
	cfg      *config.Config
	listener net.Listener
	engine   *storage.Engine
	router   *Router
	aof      AOFLogger
	logger   *logger.Logger
	tracker  *MetricsTracker
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewServer initializes a Server instance.
func NewServer(cfg *config.Config, engine *storage.Engine, router *Router, aof AOFLogger, log *logger.Logger) *Server {
	return &Server{
		cfg:     cfg,
		engine:  engine,
		router:  router,
		aof:     aof,
		logger:  log,
		tracker: &MetricsTracker{Engine: engine},
		stopCh:  make(chan struct{}),
	}
}

// Start begins listening for TCP connections on the configured address.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Bind, s.cfg.Port)

	var listener net.Listener
	var err error

	if s.cfg.TLSEnabled {
		listener, err = ListenTLS(addr, s.cfg.TLSCert, s.cfg.TLSKey)
	} else {
		listener, err = net.Listen("tcp", addr)
	}

	if err != nil {
		if s.logger != nil {
			s.logger.Error("Failed to start TCP listener on %s: %v", addr, err)
		}
		return fmt.Errorf("failed to start TCP listener on %s: %w", addr, err)
	}

	s.listener = listener
	if s.logger != nil {
		s.logger.Info("Listening on TCP %s (TLS: %v)", addr, s.cfg.TLSEnabled)
	}

	if s.cfg.MetricsPort > 0 {
		metricsAddr := fmt.Sprintf(":%d", s.cfg.MetricsPort)
		StartMetricsServer(metricsAddr, s.tracker)
		if s.logger != nil {
			s.logger.Info("Prometheus endpoint active on %s/metrics", metricsAddr)
		}
	}

	s.wg.Add(1)
	go s.acceptLoop()

	return nil
}

func (s *Server) acceptLoop() {
	defer s.wg.Done()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
				if s.logger != nil {
					s.logger.Error("Accept error: %v", err)
				}
				continue
			}
		}

		s.wg.Add(1)
		go func(c net.Conn) {
			defer s.wg.Done()
			s.handleConnection(c)
		}(conn)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	atomic.AddInt64(&s.tracker.ConnectedClients, 1)
	defer atomic.AddInt64(&s.tracker.ConnectedClients, -1)

	remoteAddr := conn.RemoteAddr().String()
	if s.logger != nil {
		s.logger.Debug("New connection established from %s", remoteAddr)
	}

	client := NewClient(conn, s.engine, s.cfg.RequirePass)

	for {
		cmdArgs, err := client.Reader.ReadCommand()
		if err == io.EOF {
			break
		}
		if err != nil {
			if s.logger != nil {
				s.logger.Debug("Client %s disconnected or read error: %v", remoteAddr, err)
			}
			break
		}

		if len(cmdArgs) == 0 {
			continue
		}

		atomic.AddInt64(&s.tracker.TotalCommands, 1)

		cmdName := strings.ToUpper(utils.BytesToString(cmdArgs[0]))
		start := time.Now()

		dispatchErr := s.router.Dispatch(client, cmdArgs)
		duration := time.Since(start)

		if s.logger != nil {
			s.logger.Request(remoteAddr, cmdName, cmdArgs[1:], duration, dispatchErr)
		}

		_ = client.Flush()

		// AOF logging for write operations
		if s.aof != nil && isWriteCommand(cmdName) {
			s.aof.WriteCommand(cmdName, cmdArgs[1:]...)
		}
	}
}

func isWriteCommand(cmd string) bool {
	switch cmd {
	case "SET", "DEL", "EXPIRE", "PERSIST", "FLUSHDB", "INCR", "DECR", "MSET":
		return true
	default:
		return false
	}
}

// Stop gracefully shuts down the TCP listener and active connection handlers.
func (s *Server) Stop() {
	close(s.stopCh)
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	if s.logger != nil {
		s.logger.Info("Server stopped gracefully.")
	}
}
