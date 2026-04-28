package orchestrator

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/siddhant2408/nimbus/internal/config"
)

// reconcileRunningIssues performs stall detection and tracker state refresh.
// SPEC Section 8.5.
func (o *Orchestrator) reconcileRunningIssues(ctx context.Context, cfg *config.Config) {
	// Part A: Stall detection.
	o.reconcileStalledRuns(cfg)

	// Part B: Tracker state refresh.
	if len(o.state.Running) == 0 {
		return
	}

	ids := make([]string, 0, len(o.state.Running))
	for id := range o.state.Running {
		ids = append(ids, id)
	}

	refreshed, err := o.tracker.FetchIssueStatesByIDs(ctx, ids)
	if err != nil {
		slog.Debug("reconciliation state refresh failed, keeping workers running", "error", err)
		return
	}

	// Build lookup from refreshed issues.
	refreshedMap := make(map[string]string, len(refreshed)) // id -> state
	for _, issue := range refreshed {
		refreshedMap[issue.ID] = issue.State
	}

	for issueID, entry := range o.state.Running {
		state, found := refreshedMap[issueID]
		if !found {
			// Issue not returned by tracker; terminate without cleanup.
			slog.Warn("reconciliation: issue missing from tracker, terminating",
				"issue_id", issueID,
				"issue_identifier", entry.Identifier,
			)
			o.terminateRunning(issueID, false)
			continue
		}

		if isTerminalState(state, cfg.Tracker.TerminalStates) {
			slog.Info("reconciliation: issue reached terminal state, terminating + cleanup",
				"issue_id", issueID,
				"issue_identifier", entry.Identifier,
				"state", state,
			)
			o.terminateRunning(issueID, true)

			// Clean workspace for terminal issues.
			if entry.Identifier != "" {
				if err := o.workspace.Remove(ctx, entry.Identifier); err != nil {
					slog.Warn("workspace cleanup failed",
						"issue_identifier", entry.Identifier,
						"error", err,
					)
				}
			}
		} else if isActiveState(state, cfg.Tracker.ActiveStates) {
			// Update in-memory snapshot.
			entry.Issue.State = state
		} else {
			// Non-active, non-terminal: stop agent without cleanup.
			slog.Info("reconciliation: issue no longer active, terminating without cleanup",
				"issue_id", issueID,
				"issue_identifier", entry.Identifier,
				"state", state,
			)
			o.terminateRunning(issueID, false)
		}
	}
}

// reconcileStalledRuns detects and terminates stalled sessions. SPEC Section 8.5 Part A.
func (o *Orchestrator) reconcileStalledRuns(cfg *config.Config) {
	stallTimeoutMs := cfg.Codex.StallTimeoutMs
	if stallTimeoutMs <= 0 {
		return // stall detection disabled
	}

	stallTimeout := time.Duration(stallTimeoutMs) * time.Millisecond
	now := time.Now()

	for issueID, entry := range o.state.Running {
		var lastActivity time.Time
		if !entry.Session.LastCodexTimestamp.IsZero() {
			lastActivity = entry.Session.LastCodexTimestamp
		} else {
			lastActivity = entry.StartedAt
		}

		elapsed := now.Sub(lastActivity)
		if elapsed > stallTimeout {
			slog.Warn("reconciliation: stalled session detected, terminating",
				"issue_id", issueID,
				"issue_identifier", entry.Identifier,
				"elapsed_ms", elapsed.Milliseconds(),
				"stall_timeout_ms", stallTimeoutMs,
			)
			nextAttempt := entry.RetryAttempt + 1
			o.terminateRunning(issueID, false)
			o.scheduleRetry(issueID, entry.Identifier, nextAttempt,
				"stalled for "+elapsed.String(), false)
		}
	}
}

// terminateRunning stops a running worker and removes it from the running map.
func (o *Orchestrator) terminateRunning(issueID string, releaseClaim bool) {
	entry, ok := o.state.Running[issueID]
	if !ok {
		return
	}

	// Cancel the worker goroutine's context.
	if entry.Cancel != nil {
		entry.Cancel()
	}

	// Accumulate runtime.
	elapsed := time.Since(entry.StartedAt).Seconds()
	o.state.EndedSecondsRunning += elapsed

	delete(o.state.Running, issueID)

	if releaseClaim {
		delete(o.state.Claimed, issueID)
	}
}

// isActiveStateLower checks active state with pre-lowered input.
func isActiveStateLower(state string, active []string) bool {
	for _, s := range active {
		if strings.ToLower(s) == state {
			return true
		}
	}
	return false
}
