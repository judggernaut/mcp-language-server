package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/isaacphi/mcp-language-server/internal/logging"
	"github.com/isaacphi/mcp-language-server/internal/lsp"
	"github.com/isaacphi/mcp-language-server/internal/telemetry"
	"github.com/isaacphi/mcp-language-server/internal/utilities"
	"github.com/isaacphi/mcp-language-server/internal/watcher"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Create a logger for the core component
var coreLogger = logging.NewLogger(logging.Core)

type config struct {
	workspaceDir string
	lspCommand   string
	lspArgs      []string
	idleTimeout  time.Duration
}

type mcpServer struct {
	config           config
	lspClient        *lsp.Client
	mcpServer        *server.MCPServer
	telemetry        *telemetry.Recorder
	ctx              context.Context
	cancelFunc       context.CancelFunc
	workspaceWatcher *watcher.WorkspaceWatcher
	cleanupOnce      sync.Once
	done             chan struct{}
	lastActivityNs   atomic.Int64
}

func parseConfig() (*config, error) {
	cfg := &config{}
	idleTimeoutDefault := envDuration("MCP_IDLE_TIMEOUT", 0)
	flag.StringVar(&cfg.workspaceDir, "workspace", "", "Path to workspace directory")
	flag.StringVar(&cfg.lspCommand, "lsp", "", "LSP command to run (args should be passed after --)")
	flag.DurationVar(&cfg.idleTimeout, "idle-timeout", idleTimeoutDefault,
		"Shut down automatically after this long with no MCP requests (0 = disabled, also settable via MCP_IDLE_TIMEOUT)")
	flag.Parse()

	// Get remaining args after -- as LSP arguments
	cfg.lspArgs = flag.Args()

	// Validate workspace directory
	if cfg.workspaceDir == "" {
		return nil, fmt.Errorf("workspace directory is required")
	}

	workspaceDir, err := filepath.Abs(cfg.workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for workspace: %v", err)
	}
	cfg.workspaceDir = workspaceDir

	if _, err := os.Stat(cfg.workspaceDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("workspace directory does not exist: %s", cfg.workspaceDir)
	}

	// Validate LSP command
	if cfg.lspCommand == "" {
		return nil, fmt.Errorf("LSP command is required")
	}

	if cfg.idleTimeout < 0 {
		return nil, fmt.Errorf("idle-timeout must be >= 0")
	}

	if _, err := exec.LookPath(cfg.lspCommand); err != nil {
		return nil, fmt.Errorf("LSP command not found: %s", cfg.lspCommand)
	}

	return cfg, nil
}

// envDuration reads a duration from the given environment variable, falling
// back to def if unset or unparseable. Used as the flag default so an env
// var can configure the server without a wrapper script, while the flag
// still takes precedence if both are set.
func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		coreLogger.Warn("Invalid duration %q for %s, using default %s", v, key, def)
		return def
	}
	return d
}

func newServer(config *config) (*mcpServer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &mcpServer{
		config:     *config,
		ctx:        ctx,
		cancelFunc: cancel,
		done:       make(chan struct{}),
	}
	s.touchActivity()
	return s, nil
}

// touchActivity records the current time as the last observed MCP activity.
// Called on every request the hooks see, and once at startup so a server
// with idle-timeout enabled doesn't measure idleness from the Unix epoch.
func (s *mcpServer) touchActivity() {
	s.lastActivityNs.Store(time.Now().UnixNano())
}

// monitorIdleTimeout shuts the server down after config.idleTimeout has
// elapsed with no MCP activity. Only started when idleTimeout > 0. Checks
// at a quarter of the timeout (min 5s) rather than continuously, since idle
// shutdown doesn't need to be precise to the second.
func (s *mcpServer) monitorIdleTimeout() {
	checkInterval := s.config.idleTimeout / 4
	if checkInterval < 5*time.Second {
		checkInterval = 5 * time.Second
	}

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			idleFor := time.Since(time.Unix(0, s.lastActivityNs.Load()))
			if idleFor >= s.config.idleTimeout {
				coreLogger.Warn("Idle timeout reached (%s >= %s), shutting down", idleFor.Round(time.Second), s.config.idleTimeout)
				cleanup(s)
				// Closing os.Stdin doesn't reliably unblock mcp-go's stdio
				// read loop on every platform (a Read() already blocked on a
				// file descriptor doesn't always observe that fd being
				// closed from another goroutine), so main()'s call to
				// server.start() may never return on its own. Exit directly
				// once our own cleanup has finished instead of waiting for it.
				os.Exit(0)
			}
		case <-s.done:
			return
		case <-s.ctx.Done():
			return
		}
	}
}

func (s *mcpServer) initializeLSP() error {
	if err := os.Chdir(s.config.workspaceDir); err != nil {
		return fmt.Errorf("failed to change to workspace directory: %v", err)
	}

	// Confine all file mutations (edits, renames, creates, deletes) to the
	// workspace so a tool call can't read or write outside the project root.
	if err := utilities.SetWorkspaceRoot(s.config.workspaceDir); err != nil {
		return fmt.Errorf("failed to set workspace root: %v", err)
	}

	client, err := lsp.NewClient(s.config.lspCommand, s.config.lspArgs...)
	if err != nil {
		return fmt.Errorf("failed to create LSP client: %v", err)
	}
	s.lspClient = client
	watcherConfig := watcher.DefaultWatcherConfig()
	watcherConfig.PreopenAllFiles = watcher.ResolvePreopenMode(s.config.lspCommand)
	s.workspaceWatcher = watcher.NewWorkspaceWatcherWithConfig(client, watcherConfig)

	initResult, err := client.InitializeLSPClient(s.ctx, s.config.workspaceDir)
	if err != nil {
		return fmt.Errorf("initialize failed: %v", err)
	}

	coreLogger.Debug("Server capabilities: %+v", initResult.Capabilities)

	go s.workspaceWatcher.WatchWorkspace(s.ctx, s.config.workspaceDir)
	return client.WaitForServerReady(s.ctx)
}

func (s *mcpServer) start() error {
	if err := s.initializeLSP(); err != nil {
		return err
	}

	const serverVersion = "v0.0.2"
	s.telemetry = telemetry.NewRecorder(serverVersion)
	if s.telemetry.FileEnabled() {
		coreLogger.Info("MCP tool telemetry: writing ATIF trajectory to %s", os.Getenv(telemetry.EnvTrajectoryFile))
	}

	hooks := newTelemetryHooks(s.telemetry)
	// Any MCP request counts as activity, not just tool calls, so a client
	// that's merely listing tools/pinging doesn't get shut down mid-session.
	hooks.AddBeforeAny(func(ctx context.Context, id any, method mcp.MCPMethod, message any) {
		s.touchActivity()
	})

	s.mcpServer = server.NewMCPServer(
		"MCP Language Server",
		serverVersion,
		server.WithLogging(),
		server.WithRecovery(),
		server.WithHooks(hooks),
	)

	err := s.registerTools()
	if err != nil {
		return fmt.Errorf("tool registration failed: %v", err)
	}

	if s.config.idleTimeout > 0 {
		coreLogger.Info("Idle timeout enabled: %s", s.config.idleTimeout)
		go s.monitorIdleTimeout()
	}

	return server.ServeStdio(s.mcpServer)
}

func main() {
	coreLogger.Info("MCP Language Server starting")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	config, err := parseConfig()
	if err != nil {
		coreLogger.Fatal("%v", err)
	}

	server, err := newServer(config)
	if err != nil {
		coreLogger.Fatal("%v", err)
	}

	// Parent process monitoring channel
	parentDeath := make(chan struct{})

	// Monitor parent process termination
	// Claude desktop does not properly kill child processes for MCP servers
	go func() {
		ppid := os.Getppid()
		coreLogger.Debug("Monitoring parent process: %d", ppid)

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				currentPpid := os.Getppid()
				if currentPpid != ppid && (currentPpid == 1 || ppid == 1) {
					coreLogger.Info("Parent process %d terminated (current ppid: %d), initiating shutdown", ppid, currentPpid)
					close(parentDeath)
					return
				}
			case <-server.done:
				return
			}
		}
	}()

	// Handle shutdown triggers
	go func() {
		select {
		case sig := <-sigChan:
			coreLogger.Info("Received signal %v in PID: %d", sig, os.Getpid())
			cleanup(server)
		case <-parentDeath:
			coreLogger.Info("Parent death detected, initiating shutdown")
			cleanup(server)
			// Unlike the signal case above (where mcp-go's own ServeStdio
			// independently catches the same signal and cancels its internal
			// context, unblocking its stdio read loop on its own), nothing
			// else observes parent death, so main()'s call to server.start()
			// would otherwise never return. Exit directly once cleanup is done.
			os.Exit(0)
		}
	}()

	if err := server.start(); err != nil {
		coreLogger.Error("Server error: %v", err)
		cleanup(server)
		<-server.done
		os.Exit(1)
	}

	// start() returned without error, which means the MCP client closed the
	// connection (stdin EOF). Shut down cleanly so the LSP child and telemetry
	// don't leak.
	coreLogger.Info("MCP connection closed, shutting down")
	cleanup(server)

	<-server.done
	coreLogger.Info("Server shutdown complete for PID: %d", os.Getpid())
	os.Exit(0)
}

func cleanup(s *mcpServer) {
	s.cleanupOnce.Do(func() {
		runCleanup(s)
	})
}

func runCleanup(s *mcpServer) {
	coreLogger.Info("Cleanup initiated for PID: %d", os.Getpid())

	// Close stdin so mcp-go's stdio read loop unblocks and ServeStdio
	// returns. mcp-go's own ServeStdio independently catches SIGINT/SIGTERM
	// and cancels its internal context to unblock itself, which is why the
	// signal-triggered shutdown path already worked without this. But the
	// parent-death and idle-timeout paths only reach this cleanup function,
	// with nothing else to unblock the still-open stdin read: without this,
	// main()'s call to server.start() would never return and the process
	// would hang instead of exiting.
	if err := os.Stdin.Close(); err != nil {
		coreLogger.Debug("Failed to close stdin during cleanup: %v", err)
	}

	// Flush any pending telemetry before shutting down.
	if s.telemetry != nil {
		s.telemetry.Close()
	}

	// Create a context with timeout for shutdown operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if s.lspClient != nil {
		coreLogger.Info("Closing open files")
		s.lspClient.CloseAllFiles(ctx)

		// Create a shorter timeout context for the shutdown request
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 500*time.Millisecond)
		defer shutdownCancel()

		// Run shutdown in a goroutine with timeout to avoid blocking if LSP doesn't respond
		shutdownDone := make(chan struct{})
		go func() {
			coreLogger.Info("Sending shutdown request")
			if err := s.lspClient.Shutdown(shutdownCtx); err != nil {
				coreLogger.Error("Shutdown request failed: %v", err)
			}
			close(shutdownDone)
		}()

		// Wait for shutdown with timeout
		select {
		case <-shutdownDone:
			coreLogger.Info("Shutdown request completed")
		case <-time.After(1 * time.Second):
			coreLogger.Warn("Shutdown request timed out, proceeding with exit")
		}

		coreLogger.Info("Sending exit notification")
		if err := s.lspClient.Exit(ctx); err != nil {
			coreLogger.Error("Exit notification failed: %v", err)
		}

		coreLogger.Info("Closing LSP client")
		if err := s.lspClient.Close(); err != nil {
			coreLogger.Error("Failed to close LSP client: %v", err)
		}
	}

	// Send signal to the done channel
	select {
	case <-s.done: // Channel already closed
	default:
		close(s.done)
	}

	coreLogger.Info("Cleanup completed for PID: %d", os.Getpid())
}
