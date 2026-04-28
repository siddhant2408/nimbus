package orchestrator

import (
	"context"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/siddhant2408/nimbus/internal/config"
	"github.com/siddhant2408/nimbus/internal/domain"
)

// shouldDispatch checks all dispatch eligibility rules. SPEC Section 8.2.
func (o *Orchestrator) shouldDispatch(issue *domain.Issue, cfg *config.Config) bool {
	// Required fields.
	if issue.ID == "" || issue.Identifier == "" || issue.Title == "" || issue.State == "" {
		return false
	}

	// Must be in active state.
	if !isActiveState(issue.State, cfg.Tracker.ActiveStates) {
		return false
	}

	// Must not be in terminal state.
	if isTerminalState(issue.State, cfg.Tracker.TerminalStates) {
		return false
	}

	// Must not be already claimed or running.
	if _, ok := o.state.Claimed[issue.ID]; ok {
		return false
	}
	if _, ok := o.state.Running[issue.ID]; ok {
		return false
	}

	// Todo blocker rule: don't dispatch if any blocker is non-terminal.
	if isTodoState(issue.State) && hasNonTerminalBlockers(issue, cfg.Tracker.TerminalStates) {
		return false
	}

	// Per-state concurrency limit.
	if !o.stateSlotsAvailable(issue.State, cfg) {
		return false
	}

	return true
}

// dispatchIssue spawns a worker goroutine for the issue. SPEC Section 16.4.
func (o *Orchestrator) dispatchIssue(ctx context.Context, issue *domain.Issue, attempt *int, cfg *config.Config) {
	workerCtx, cancel := context.WithCancel(ctx)

	entry := &domain.RunningEntry{
		IssueID:    issue.ID,
		Issue:      *issue,
		Identifier: issue.Identifier,
		StartedAt:  time.Now().UTC(),
		Cancel:     cancel,
	}
	if attempt != nil {
		entry.RetryAttempt = *attempt
	}

	o.state.Running[issue.ID] = entry
	o.state.Claimed[issue.ID] = struct{}{}
	delete(o.state.RetryAttempts, issue.ID)

	slog.Info("dispatching issue",
		"issue_id", issue.ID,
		"issue_identifier", issue.Identifier,
		"state", issue.State,
		"attempt", attempt,
	)

	// Spawn worker goroutine. On exit, send event to orchestrator.
	issueCopy := *issue
	var attemptCopy *int
	if attempt != nil {
		v := *attempt
		attemptCopy = &v
	}

	go func() {
		err := o.runner(workerCtx, issueCopy, attemptCopy)
		o.WorkerExitCh <- domain.WorkerExitEvent{
			IssueID: issueCopy.ID,
			Err:     err,
		}
	}()
}

// availableSlots returns how many global dispatch slots remain.
func (o *Orchestrator) availableSlots(cfg *config.Config) int {
	slots := cfg.Agent.MaxConcurrentAgents - len(o.state.Running)
	if slots < 0 {
		return 0
	}
	return slots
}

// stateSlotsAvailable checks per-state concurrency limits. SPEC Section 8.3.
func (o *Orchestrator) stateSlotsAvailable(state string, cfg *config.Config) bool {
	normalized := strings.ToLower(state)
	limit, ok := cfg.Agent.MaxConcurrentAgentsByState[normalized]
	if !ok {
		return true // no per-state limit
	}

	count := 0
	for _, entry := range o.state.Running {
		if strings.ToLower(entry.Issue.State) == normalized {
			count++
		}
	}
	return count < limit
}

// sortIssuesForDispatch sorts issues by priority (ascending), then created_at (oldest first),
// then identifier (lexicographic). SPEC Section 8.2.
func sortIssuesForDispatch(issues []domain.Issue) {
	sort.SliceStable(issues, func(i, j int) bool {
		pi := priorityRank(issues[i].Priority)
		pj := priorityRank(issues[j].Priority)
		if pi != pj {
			return pi < pj
		}

		// Created at: oldest first.
		ti := timeRank(issues[i].CreatedAt)
		tj := timeRank(issues[j].CreatedAt)
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}

		// Identifier: lexicographic.
		return issues[i].Identifier < issues[j].Identifier
	})
}

// priorityRank returns a sort key for priority (lower is better, nil sorts last).
func priorityRank(p *int) int {
	if p == nil {
		return 999
	}
	return *p
}

// timeRank returns a sort key for time (nil sorts to max time / last).
func timeRank(t *time.Time) time.Time {
	if t == nil {
		return time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	return *t
}

func isActiveState(state string, active []string) bool {
	n := strings.ToLower(state)
	for _, s := range active {
		if strings.ToLower(s) == n {
			return true
		}
	}
	return false
}

func isTerminalState(state string, terminal []string) bool {
	n := strings.ToLower(state)
	for _, s := range terminal {
		if strings.ToLower(s) == n {
			return true
		}
	}
	return false
}

func isTodoState(state string) bool {
	return strings.ToLower(state) == "todo"
}

// hasNonTerminalBlockers returns true if any blocker has a non-terminal state.
func hasNonTerminalBlockers(issue *domain.Issue, terminalStates []string) bool {
	for _, blocker := range issue.BlockedBy {
		if blocker.State == nil {
			// Unknown state treated as non-terminal.
			return true
		}
		if !isTerminalState(*blocker.State, terminalStates) {
			return true
		}
	}
	return false
}
