package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"io/fs"

	"github.com/anthropics/symphony/internal/agent"
	"github.com/anthropics/symphony/internal/codex"
	"github.com/anthropics/symphony/internal/config"
	"github.com/anthropics/symphony/internal/domain"
	"github.com/anthropics/symphony/internal/orchestrator"
	"github.com/anthropics/symphony/internal/persona"
	"github.com/anthropics/symphony/internal/server"
	"github.com/anthropics/symphony/internal/tracker/linear"
	"github.com/anthropics/symphony/internal/workspace"
	"github.com/anthropics/symphony/web"
)

func main() {
	port := flag.Int("port", -1, "HTTP server port (overrides server.port in WORKFLOW.md)")
	logsRoot := flag.String("logs-root", "", "Log file directory")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: symphony [OPTIONS] [path-to-WORKFLOW.md]\n\n")
		fmt.Fprintf(os.Stderr, "Symphony orchestrates coding agents for project work.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// Resolve workflow file path.
	workflowPath := "WORKFLOW.md"
	if flag.NArg() > 0 {
		workflowPath = flag.Arg(0)
	}
	absPath, err := filepath.Abs(workflowPath)
	if err != nil {
		fatal("resolve workflow path: %v", err)
	}

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		fatal("workflow file not found: %s", absPath)
	}

	// Configure logging.
	configureLogging(*logsRoot)

	slog.Info("starting symphony",
		"workflow", absPath,
		"pid", os.Getpid(),
	)

	// Load and watch workflow config.
	cfgWatch, err := config.NewWatcher(absPath)
	if err != nil {
		fatal("load workflow: %v", err)
	}

	// Validate config at startup.
	cfg := cfgWatch.Current().Config
	if err := config.ValidateForDispatch(cfg); err != nil {
		fatal("config validation: %v", err)
	}

	// Port override: CLI flag > config > skip server.
	serverPort := cfg.Server.Port
	if *port >= 0 {
		serverPort = *port
	}

	// Create dependencies.
	configFn := func() *config.Config { return cfgWatch.Current().Config }
	promptFn := func() string { return cfgWatch.Current().PromptTemplate }

	trk := linear.NewClient(configFn)
	ws := workspace.NewManager(configFn)

	// Persona extension.
	personaReg := persona.NewRegistry()
	if cfg.Personas.Directory != "" {
		if err := personaReg.Load(cfg.Personas.Directory); err != nil {
			slog.Warn("persona loading failed", "error", err)
		}
	}
	personaStore := persona.NewAssignmentStore(cfg.Workspace.Root)

	// Reload personas when workflow config changes.
	cfgWatch.OnChange(func(def *config.WorkflowDefinition) {
		if def.Config.Personas.Directory != "" {
			if err := personaReg.Load(def.Config.Personas.Directory); err != nil {
				slog.Warn("persona reload failed", "error", err)
			}
		}
	})

	// Start config watcher.
	cfgWatch.Start()
	defer cfgWatch.Stop()

	// Wrap the Linear client as a codex.GraphQLExecutor.
	graphqlExec := codex.GraphQLExecutor(func(query string, variables map[string]any) (json.RawMessage, error) {
		result, err := trk.GraphQL(context.Background(), query, variables)
		if err != nil {
			return nil, err
		}
		return json.Marshal(result)
	})

	// Create orchestrator. Use a pointer so the closure can reference it.
	var orch *orchestrator.Orchestrator
	orch = orchestrator.New(cfgWatch, trk, ws, func(ctx context.Context, issue domain.Issue, attempt *int) error {
		return agent.Run(ctx, issue, attempt, agent.RunnerDeps{
			Config:         configFn,
			PromptTemplate: promptFn,
			Workspace:      ws,
			PersonaReg:     personaReg,
			PersonaStore:   personaStore,
			OnCodexEvent: func(evt domain.CodexUpdateEvent) {
				orch.CodexUpdateCh <- evt
			},
			OnRuntimeInfo: func(evt domain.WorkerRuntimeInfoEvent) {
				orch.RuntimeInfoCh <- evt
			},
			GraphQLExec: graphqlExec,
		})
	})

	// Context with signal handling.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Set embedded frontend assets.
	if distFS, fsErr := fs.Sub(web.DistFS, "dist"); fsErr == nil {
		server.SetFrontendFS(distFS)
	}

	// Start HTTP server if port configured.
	if serverPort > 0 {
		snapshotFn := func() *domain.Snapshot {
			replyCh := make(chan *domain.Snapshot, 1)
			orch.SnapshotCh <- domain.SnapshotRequest{Reply: replyCh}
			return <-replyCh
		}
		refreshFn := func() domain.RefreshResponse {
			replyCh := make(chan domain.RefreshResponse, 1)
			orch.RefreshCh <- domain.RefreshRequest{Reply: replyCh}
			return <-replyCh
		}

		srv := server.New(configFn, snapshotFn, refreshFn, personaReg, personaStore)
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, serverPort)

		go func() {
			if err := srv.Start(ctx, addr); err != nil {
				slog.Error("HTTP server error", "error", err)
			}
		}()
	}

	// Run orchestrator (blocks until ctx cancelled).
	slog.Info("orchestrator starting",
		"poll_interval_ms", cfg.Polling.IntervalMs,
		"max_concurrent_agents", cfg.Agent.MaxConcurrentAgents,
		"tracker", cfg.Tracker.Kind,
		"project", cfg.Tracker.ProjectSlug,
	)
	orch.Run(ctx)

	slog.Info("symphony stopped")
}

func configureLogging(logsRoot string) {
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	var handler slog.Handler
	if logsRoot != "" {
		// JSON logging to file.
		if err := os.MkdirAll(logsRoot, 0755); err != nil {
			fatal("create logs directory: %v", err)
		}
		logFile := filepath.Join(logsRoot, "symphony.log")
		f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fatal("open log file: %v", err)
		}
		handler = slog.NewJSONHandler(f, opts)
	} else {
		handler = slog.NewTextHandler(os.Stderr, opts)
	}

	slog.SetDefault(slog.New(handler))
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "symphony: "+format+"\n", args...)
	os.Exit(1)
}
