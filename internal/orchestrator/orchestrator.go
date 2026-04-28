package orchestrator

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/anthropics/symphony/internal/config"
	"github.com/anthropics/symphony/internal/domain"
	"github.com/anthropics/symphony/internal/tracker"
	"github.com/anthropics/symphony/internal/workspace"
)

// RunnerFunc is the signature for spawning agent worker goroutines.
type RunnerFunc func(ctx context.Context, issue domain.Issue, attempt *int) error

// Orchestrator owns the poll tick, in-memory runtime state, and all dispatch/retry/reconciliation
// decisions. It runs as a single goroutine consuming typed channels. SPEC Section 7.
type Orchestrator struct {
	// Dependencies (injected).
	cfgWatch  *config.Watcher
	tracker   tracker.Tracker
	workspace *workspace.Manager
	runner    RunnerFunc

	// Channels — the orchestrator's "mailbox".
	WorkerExitCh  chan domain.WorkerExitEvent
	CodexUpdateCh chan domain.CodexUpdateEvent
	RuntimeInfoCh chan domain.WorkerRuntimeInfoEvent
	RetryTimerCh  chan domain.RetryTimerEvent
	SnapshotCh    chan domain.SnapshotRequest
	RefreshCh     chan domain.RefreshRequest

	// Internal state — only accessed by the Run goroutine.
	state State

	nextRetryToken atomic.Uint64
}

// State holds the single-authority in-memory orchestrator state. SPEC Section 4.1.8.
type State struct {
	Running       map[string]*domain.RunningEntry // issue_id -> entry
	Claimed       map[string]struct{}             // issue IDs reserved
	RetryAttempts map[string]*domain.RetryEntry   // issue_id -> retry
	Completed     map[string]struct{}             // bookkeeping only
	CodexTotals   domain.CodexTotals
	RateLimits    map[string]any
	// Cumulative runtime from ended sessions (active session time added at snapshot).
	EndedSecondsRunning float64
}

// New creates an Orchestrator with injected dependencies.
func New(
	cfgWatch *config.Watcher,
	trk tracker.Tracker,
	ws *workspace.Manager,
	runner RunnerFunc,
) *Orchestrator {
	return &Orchestrator{
		cfgWatch:      cfgWatch,
		tracker:       trk,
		workspace:     ws,
		runner:        runner,
		WorkerExitCh:  make(chan domain.WorkerExitEvent, 64),
		CodexUpdateCh: make(chan domain.CodexUpdateEvent, 256),
		RuntimeInfoCh: make(chan domain.WorkerRuntimeInfoEvent, 64),
		RetryTimerCh:  make(chan domain.RetryTimerEvent, 64),
		SnapshotCh:    make(chan domain.SnapshotRequest, 8),
		RefreshCh:     make(chan domain.RefreshRequest, 4),
		state: State{
			Running:       make(map[string]*domain.RunningEntry),
			Claimed:       make(map[string]struct{}),
			RetryAttempts: make(map[string]*domain.RetryEntry),
			Completed:     make(map[string]struct{}),
		},
	}
}

// Run starts the orchestrator event loop. Blocks until ctx is cancelled. SPEC Section 16.1.
func (o *Orchestrator) Run(ctx context.Context) {
	cfg := o.currentConfig()

	// Startup terminal workspace cleanup. SPEC Section 8.6.
	o.startupTerminalCleanup(ctx)

	// Immediate first tick, then repeat on interval.
	ticker := time.NewTicker(1 * time.Millisecond) // immediate
	defer ticker.Stop()
	firstTick := true

	for {
		select {
		case <-ctx.Done():
			o.shutdown()
			return

		case <-ticker.C:
			if firstTick {
				ticker.Reset(time.Duration(cfg.Polling.IntervalMs) * time.Millisecond)
				firstTick = false
			}
			o.onTick(ctx)
			// Re-read config in case poll interval changed.
			cfg = o.currentConfig()
			ticker.Reset(time.Duration(cfg.Polling.IntervalMs) * time.Millisecond)

		case evt := <-o.WorkerExitCh:
			o.onWorkerExit(evt)

		case evt := <-o.CodexUpdateCh:
			o.onCodexUpdate(evt)

		case evt := <-o.RuntimeInfoCh:
			o.onRuntimeInfo(evt)

		case evt := <-o.RetryTimerCh:
			o.onRetryTimer(ctx, evt)

		case req := <-o.SnapshotCh:
			req.Reply <- o.buildSnapshot()

		case req := <-o.RefreshCh:
			req.Reply <- domain.RefreshResponse{
				Queued:      true,
				Coalesced:   false,
				RequestedAt: time.Now().UTC(),
			}
			// Trigger immediate tick.
			o.onTick(ctx)
		}
	}
}

// currentConfig returns the latest valid config.
func (o *Orchestrator) currentConfig() *config.Config {
	return o.cfgWatch.Current().Config
}

// onTick runs one poll-and-dispatch cycle. SPEC Section 16.2.
func (o *Orchestrator) onTick(ctx context.Context) {
	cfg := o.currentConfig()

	// 1. Reconcile running issues.
	o.reconcileRunningIssues(ctx, cfg)

	// 2. Validate config.
	if err := config.ValidateForDispatch(cfg); err != nil {
		slog.Error("dispatch preflight validation failed", "error", err)
		return
	}

	// 3. Fetch candidate issues.
	issues, err := o.tracker.FetchCandidateIssues(ctx)
	if err != nil {
		slog.Error("candidate fetch failed", "error", err)
		return
	}

	// 4. Sort for dispatch.
	sortIssuesForDispatch(issues)

	// 5. Dispatch eligible issues.
	for i := range issues {
		if o.availableSlots(cfg) <= 0 {
			break
		}
		if o.shouldDispatch(&issues[i], cfg) {
			o.dispatchIssue(ctx, &issues[i], nil, cfg)
		}
	}
}

// onWorkerExit handles a worker goroutine finishing. SPEC Section 16.6.
func (o *Orchestrator) onWorkerExit(evt domain.WorkerExitEvent) {
	entry, ok := o.state.Running[evt.IssueID]
	if !ok {
		return
	}

	// Remove from running, accumulate runtime.
	delete(o.state.Running, evt.IssueID)
	elapsed := time.Since(entry.StartedAt).Seconds()
	o.state.EndedSecondsRunning += elapsed

	if evt.Err == nil {
		// Normal exit: schedule continuation retry. SPEC Section 7.3.
		o.state.Completed[evt.IssueID] = struct{}{}
		slog.Info("worker exited normally, scheduling continuation",
			"issue_id", evt.IssueID,
			"issue_identifier", entry.Identifier,
		)
		o.scheduleRetry(evt.IssueID, entry.Identifier, 1, "", true)
	} else {
		// Abnormal exit: exponential backoff retry.
		nextAttempt := entry.RetryAttempt + 1
		slog.Warn("worker exited with error, scheduling retry",
			"issue_id", evt.IssueID,
			"issue_identifier", entry.Identifier,
			"error", evt.Err,
			"next_attempt", nextAttempt,
		)
		o.scheduleRetry(evt.IssueID, entry.Identifier, nextAttempt, evt.Err.Error(), false)
	}
}

// onCodexUpdate integrates a Codex event into the running entry. SPEC Section 7.3.
func (o *Orchestrator) onCodexUpdate(evt domain.CodexUpdateEvent) {
	entry, ok := o.state.Running[evt.IssueID]
	if !ok {
		return
	}

	if evt.SessionID != "" {
		entry.Session.SessionID = evt.SessionID
	}
	if evt.CodexAppServerPID != "" {
		entry.Session.CodexAppServerPID = evt.CodexAppServerPID
	}
	entry.Session.LastCodexEvent = evt.Event
	entry.Session.LastCodexTimestamp = evt.Timestamp
	entry.Session.LastCodexMessage = evt.Message
	if evt.TurnCount > 0 {
		entry.Session.TurnCount = evt.TurnCount
	}

	// Apply token deltas.
	if evt.Usage != nil {
		applyTokenDelta(entry, evt.Usage)
	}

	// Track rate limits.
	if evt.RateLimits != nil {
		o.state.RateLimits = evt.RateLimits
	}
}

// onRuntimeInfo updates workspace/host info for a running entry.
func (o *Orchestrator) onRuntimeInfo(evt domain.WorkerRuntimeInfoEvent) {
	entry, ok := o.state.Running[evt.IssueID]
	if !ok {
		return
	}
	entry.WorkspacePath = evt.WorkspacePath
	entry.WorkerHost = evt.WorkerHost
	if evt.PersonaName != "" {
		entry.PersonaName = evt.PersonaName
	}
}

// onRetryTimer handles a retry timer firing. SPEC Section 16.6.
func (o *Orchestrator) onRetryTimer(ctx context.Context, evt domain.RetryTimerEvent) {
	retry, ok := o.state.RetryAttempts[evt.IssueID]
	if !ok {
		return
	}

	// Validate retry token to prevent stale fires.
	if retry.RetryToken != evt.RetryToken {
		return
	}

	delete(o.state.RetryAttempts, evt.IssueID)
	cfg := o.currentConfig()

	// Fetch active candidates and look for this issue.
	issues, err := o.tracker.FetchCandidateIssues(ctx)
	if err != nil {
		slog.Error("retry candidate fetch failed", "issue_id", evt.IssueID, "error", err)
		o.scheduleRetry(evt.IssueID, retry.Identifier, retry.Attempt+1, "retry poll failed", false)
		return
	}

	var found *domain.Issue
	for i := range issues {
		if issues[i].ID == evt.IssueID {
			found = &issues[i]
			break
		}
	}

	if found == nil {
		// Issue no longer active; release claim.
		delete(o.state.Claimed, evt.IssueID)
		slog.Info("retry: issue no longer active, releasing claim",
			"issue_id", evt.IssueID,
			"issue_identifier", retry.Identifier,
		)
		return
	}

	if o.availableSlots(cfg) <= 0 {
		o.scheduleRetry(evt.IssueID, retry.Identifier, retry.Attempt+1,
			"no available orchestrator slots", false)
		return
	}

	attempt := retry.Attempt
	o.dispatchIssue(ctx, found, &attempt, cfg)
}

// shutdown cleans up when the orchestrator stops.
func (o *Orchestrator) shutdown() {
	slog.Info("orchestrator shutting down",
		"running", len(o.state.Running),
		"retrying", len(o.state.RetryAttempts),
	)

	// Cancel all running workers.
	for _, entry := range o.state.Running {
		if entry.Cancel != nil {
			entry.Cancel()
		}
	}

	// Stop all retry timers.
	for _, retry := range o.state.RetryAttempts {
		if retry.Timer != nil {
			retry.Timer.Stop()
		}
	}
}

// startupTerminalCleanup removes workspaces for issues already in terminal states.
// SPEC Section 8.6.
func (o *Orchestrator) startupTerminalCleanup(ctx context.Context) {
	cfg := o.currentConfig()

	issues, err := o.tracker.FetchIssuesByStates(ctx, cfg.Tracker.TerminalStates)
	if err != nil {
		slog.Warn("startup terminal cleanup: fetch failed", "error", err)
		return
	}

	for _, issue := range issues {
		if err := o.workspace.Remove(ctx, issue.Identifier); err != nil {
			slog.Debug("startup cleanup: workspace removal skipped",
				"issue_identifier", issue.Identifier,
				"error", err,
			)
		}
	}

	if len(issues) > 0 {
		slog.Info("startup terminal cleanup complete", "removed", len(issues))
	}
}

// buildSnapshot creates a point-in-time view of orchestrator state for the API.
func (o *Orchestrator) buildSnapshot() *domain.Snapshot {
	now := time.Now().UTC()

	running := make([]domain.SnapshotRunning, 0, len(o.state.Running))
	var activeSeconds float64
	for _, entry := range o.state.Running {
		activeSeconds += time.Since(entry.StartedAt).Seconds()

		var lastEventAt *time.Time
		if !entry.Session.LastCodexTimestamp.IsZero() {
			t := entry.Session.LastCodexTimestamp
			lastEventAt = &t
		}

		lastMsg := ""
		if entry.Session.LastCodexMessage != nil {
			if msg, ok := entry.Session.LastCodexMessage["message"].(string); ok {
				lastMsg = msg
			}
		}

		running = append(running, domain.SnapshotRunning{
			IssueID:       entry.IssueID,
			Identifier:    entry.Identifier,
			State:         entry.Issue.State,
			SessionID:     entry.Session.SessionID,
			TurnCount:     entry.Session.TurnCount,
			Persona:       entry.PersonaName,
			LastEvent:     entry.Session.LastCodexEvent,
			LastMessage:   lastMsg,
			StartedAt:     entry.StartedAt,
			LastEventAt:   lastEventAt,
			WorkerHost:    entry.WorkerHost,
			WorkspacePath: entry.WorkspacePath,
			Tokens: domain.SnapshotTokens{
				InputTokens:  entry.Session.InputTokens,
				OutputTokens: entry.Session.OutputTokens,
				TotalTokens:  entry.Session.TotalTokens,
			},
		})
	}

	retrying := make([]domain.SnapshotRetrying, 0, len(o.state.RetryAttempts))
	for _, retry := range o.state.RetryAttempts {
		retrying = append(retrying, domain.SnapshotRetrying{
			IssueID:    retry.IssueID,
			Identifier: retry.Identifier,
			Attempt:    retry.Attempt,
			DueAt:      retry.DueAt.Format(time.RFC3339),
			Error:      retry.Error,
			WorkerHost: retry.WorkerHost,
		})
	}

	totalSeconds := o.state.EndedSecondsRunning + activeSeconds

	return &domain.Snapshot{
		GeneratedAt: now,
		Counts: domain.SnapshotCounts{
			Running:  len(o.state.Running),
			Retrying: len(o.state.RetryAttempts),
		},
		Running:  running,
		Retrying: retrying,
		CodexTotals: domain.SnapshotCodexTotals{
			InputTokens:    o.state.CodexTotals.InputTokens,
			OutputTokens:   o.state.CodexTotals.OutputTokens,
			TotalTokens:    o.state.CodexTotals.TotalTokens,
			SecondsRunning: totalSeconds,
		},
		RateLimits: o.state.RateLimits,
	}
}
