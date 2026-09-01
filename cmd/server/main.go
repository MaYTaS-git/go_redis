package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"go_redis/internal/commands"
	"go_redis/internal/config"
	"go_redis/internal/persistence"
	"go_redis/internal/server"
	"go_redis/internal/storage"
	syslog "go_redis/pkg/logger"
	"go_redis/pkg/utils"
)

func main() {
	configPathFlag := flag.String("config", "", "Path to configuration file (default: config.txt)")
	flag.Parse()

	// 1. Load Configuration
	cfg, err := config.LoadConfig(*configPathFlag)
	if err != nil {
		log.Fatalf("[BOOT] Failed to load configuration: %v", err)
	}

	// 2. Ensure Data and Storage Directories Exist
	if err := ensureDataDirectories(cfg); err != nil {
		log.Printf("[BOOT] Warning: Failed to ensure data directory: %v", err)
	}

	// 3. Initialize Structured Logger
	appLog, err := syslog.NewLogger(cfg.LogEnabled, cfg.LogLevel, cfg.LogRequests, cfg.LogFile)
	if err != nil {
		log.Fatalf("[BOOT] Failed to initialize logger: %v", err)
	}
	defer appLog.Close()

	appLog.Info("Starting Go-Redis Server...")
	logAdaptationStatus(cfg.Port, cfg.Bind, appLog, engineStats(nil, cfg.MaxMemoryMB), cfg.AOFEnabled, cfg.SnapshotEnabled)

	// 4. Initialize Storage Engine
	engine := storage.NewEngine(cfg.MaxMemoryMB, cfg.EvictionPolicy)
	defer engine.Close()

	// 5. Register Command Handlers in Router
	router := server.NewRouter()
	router.Register("PING", commands.HandlePing)
	router.Register("AUTH", commands.HandleAuth)
	router.Register("INFO", commands.HandleInfo)
	router.Register("BGSAVE", commands.HandleBGSave)
	router.Register("SHUTDOWN", commands.HandleShutdown)
	router.Register("SELECT", commands.HandleSelect)
	router.Register("CLIENT", commands.HandleClient)
	router.Register("COMMAND", commands.HandleCommand)
	router.Register("CONFIG", commands.HandleConfig)
	router.Register("ECHO", commands.HandleEcho)
	router.Register("HELLO", commands.HandleHello)

	router.Register("GET", commands.HandleGet)
	router.Register("SET", commands.HandleSet)
	router.Register("MGET", commands.HandleMGet)
	router.Register("MSET", commands.HandleMSet)
	router.Register("INCR", commands.HandleIncr)
	router.Register("DECR", commands.HandleDecr)
	router.Register("DEL", commands.HandleDel)
	router.Register("EXISTS", commands.HandleExists)

	router.Register("EXPIRE", commands.HandleExpire)
	router.Register("TTL", commands.HandleTTL)
	router.Register("PERSIST", commands.HandlePersist)
	router.Register("KEYS", commands.HandleKeys)
	router.Register("FLUSHDB", commands.HandleFlushDB)
	router.Register("DBSIZE", commands.HandleDBSize)

	// 6. Cold Recovery (Load Snapshot)
	snapshotPath := ""
	if cfg.SnapshotEnabled {
		snapshotPath = cfg.SnapshotPath
		if err := persistence.ColdRecovery(snapshotPath, "", engine, router); err != nil {
			appLog.Warn("Snapshot cold recovery notice: %v", err)
		} else {
			appLog.Info("Snapshot recovery completed. Keys restored: %d", engine.DBSize())
		}
	}

	// 7. Initialize AOF Logger & Atomic Recovery (Single Open Descriptor)
	var aof *persistence.AOF
	if cfg.AOFEnabled {
		var replayed int
		aof, replayed, err = persistence.NewAOF(cfg.AOFPath, cfg.AOFFsync, engine, router)
		if err != nil {
			appLog.Error("Failed to initialize AOF logger: %v", err)
			os.Exit(1)
		}
		defer aof.Close()
		appLog.Info("AOF persistence active. Total keys loaded: %d (Replayed AOF commands: %d)", engine.DBSize(), replayed)
	}

	// 8. Start TCP Server
	srv := server.NewServer(cfg, engine, router, aof, appLog)
	if err := srv.Start(); err != nil {
		appLog.Error("Fatal server error: %v", err)
		os.Exit(1)
	}

	// 9. Setup Signal Handling & Interactive Keyboard Listener
	shutdownCh := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	// Background interactive terminal keyboard shortcut listener
	go func() {
		reader := bufio.NewReader(os.Stdin)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.TrimSpace(line)
			if cmd == "" {
				continue
			}

			switch strings.ToLower(cmd) {
			case "l":
				newState := appLog.ToggleLogRequests()
				appLog.Info("[ADAPT] Request tracing updated on the fly to: %v", newState)
				logAdaptationStatus(cfg.Port, cfg.Bind, appLog, engineStats(engine, cfg.MaxMemoryMB), cfg.AOFEnabled, cfg.SnapshotEnabled)

			case "v":
				newLevel := appLog.CycleLogLevel()
				appLog.Info("[ADAPT] Log verbosity level updated on the fly to: %s", newLevel)
				logAdaptationStatus(cfg.Port, cfg.Bind, appLog, engineStats(engine, cfg.MaxMemoryMB), cfg.AOFEnabled, cfg.SnapshotEnabled)

			case "f":
				engine.FlushDB()
				appLog.Info("[FLUSH] Cache cleared on the fly! All keys removed from memory.")

			case "s":
				if cfg.SnapshotEnabled {
					snp := persistence.NewSnapshotter(cfg.SnapshotPath, engine)
					if err := snp.TriggerBGSave(); err != nil {
						appLog.Warn("[SNAPSHOT] Manual BGSave failed: %v", err)
					} else {
						appLog.Info("[SNAPSHOT] Manual background snapshot (BGSAVE) triggered.")
					}
				} else {
					appLog.Warn("[SNAPSHOT] Snapshots disabled in configuration.")
				}

			case "i":
				hits, misses, usedMem, keysCount := engine.Stats()
				appLog.Info("[STATS] Memory: %.2fMB / %dMB | Active Keys: %d | Hits: %d | Misses: %d",
					float64(usedMem)/(1024*1024), cfg.MaxMemoryMB, keysCount, hits, misses)

			case "h", "?":
				printHelpMenu()

			case "q", "exit", "quit":
				appLog.Info("[SHUTDOWN] Interactive exit command 'q' received.")
				close(shutdownCh)
				return
			}
		}
	}()

	// 10. Wait for Shutdown Trigger (Ctrl+C, Signal, or 'q' key)
	select {
	case sig := <-sigCh:
		appLog.Info("[SHUTDOWN] Signal %v received. Initiating graceful shutdown...", sig)
	case <-shutdownCh:
		appLog.Info("[SHUTDOWN] Exit key pressed. Initiating graceful shutdown...")
	}

	// 11. Graceful Cleanup
	srv.Stop()

	if cfg.SnapshotEnabled {
		appLog.Info("[SHUTDOWN] Saving final snapshot to %s...", cfg.SnapshotPath)
		snp := persistence.NewSnapshotter(cfg.SnapshotPath, engine)
		if err := snp.Save(); err != nil {
			appLog.Error("[SHUTDOWN] Error saving final snapshot: %v", err)
		} else {
			appLog.Info("[SHUTDOWN] Final snapshot saved successfully.")
		}
	}

	appLog.Info("Go-Redis Server exited cleanly.")
}

func engineStats(eng *storage.Engine, maxMemMB int64) string {
	if eng == nil {
		return fmt.Sprintf("0.00 MB / %d MB (0 keys)", maxMemMB)
	}
	_, _, usedMem, keysCount := eng.Stats()
	return fmt.Sprintf("%.2f MB / %d MB (%d keys)", float64(usedMem)/(1024*1024), maxMemMB, keysCount)
}

func logAdaptationStatus(port int, bind string, appLog *syslog.Logger, memStats string, aofEnabled bool, snapshotEnabled bool) {
	reqStatus := "DISABLED"
	if appLog.GetLogRequests() {
		reqStatus = "ENABLED"
	}

	aofStr := "DISABLED"
	if aofEnabled {
		aofStr = "ENABLED"
	}
	snpStr := "DISABLED"
	if snapshotEnabled {
		snpStr = "ENABLED"
	}

	banner := fmt.Sprintf("\n"+
		"==============================================================================\n"+
		"⚡ GO-REDIS ADAPTIVE SERVER STATUS & SHORTCUTS MENU\n"+
		"------------------------------------------------------------------------------\n"+
		" • Server Address  : %s:%d\n"+
		" • Request Tracing : [l] %s  (Press 'l' + Enter to toggle on/off)\n"+
		" • Log Level       : [v] %s       (Press 'v' + Enter to cycle level)\n"+
		" • Memory Usage    : %s        (Press 'f' + Enter to flush cache)\n"+
		" • Durability      : Snapshot: %s | AOF: %s\n"+
		" • Interactive Keys: [i] Stats | [s] Save Snapshot | [h] Help | [q] Exit\n"+
		"==============================================================================",
		bind, port, reqStatus, appLog.GetLogLevelStr(), memStats, snpStr, aofStr)

	fmt.Println(banner)
}

func printHelpMenu() {
	menu := `
==============================================================================
⌨️ INTERACTIVE KEYBOARD SHORTCUTS MENU
------------------------------------------------------------------------------
  l + Enter  : Toggle Request Tracing (on/off without restarting)
  v + Enter  : Cycle Log Level (DEBUG -> INFO -> WARN -> ERROR)
  f + Enter  : Flush All Keys (FLUSHDB in-memory cache)
  s + Enter  : Trigger Manual Snapshot (BGSAVE to dump.db)
  i + Enter  : Show Live Memory & Hit/Miss Telemetry Stats
  h + Enter  : Show this Shortcuts Help Menu
  q + Enter  : Graceful Shutdown & Exit (or Ctrl+C)
==============================================================================`
	fmt.Println(menu)
}

func ensureDataDirectories(cfg *config.Config) error {
	// 1. Ensure root data directory
	if err := utils.EnsureDirExists("./data"); err != nil {
		return err
	}

	// 2. Ensure parent directories for configured persistence files
	if cfg.AOFPath != "" {
		_ = utils.EnsureDir(cfg.AOFPath)
	}
	if cfg.SnapshotPath != "" {
		_ = utils.EnsureDir(cfg.SnapshotPath)
	}
	if cfg.LogFile != "" {
		_ = utils.EnsureDir(cfg.LogFile)
	}

	return nil
}
