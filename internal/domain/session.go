package domain

import (
	"context"
	"time"
)

// LiveSession tracks state while a coding-agent subprocess is running.
// Maps to SPEC Section 4.1.6.
type LiveSession struct {
	SessionID         string // "<thread_id>-<turn_id>"
	ThreadID          string
	TurnID            string
	CodexAppServerPID string
	LastCodexEvent    string
	LastCodexTimestamp time.Time
	LastCodexMessage  map[string]any
	InputTokens       int64
	OutputTokens      int64
	TotalTokens       int64
	// Track last reported values for delta computation.
	LastReportedInputTokens  int64
	LastReportedOutputTokens int64
	LastReportedTotalTokens  int64
	TurnCount                int
}

// RunningEntry represents an active agent session in the orchestrator state.
type RunningEntry struct {
	IssueID       string
	Issue         Issue
	Identifier    string
	WorkerHost    string
	WorkspacePath string
	PersonaName   string // empty if no persona assigned
	Session       LiveSession
	RetryAttempt  int
	StartedAt     time.Time
	Cancel        context.CancelFunc // cancels the worker goroutine
}

// RetryEntry is a scheduled retry for an issue. Maps to SPEC Section 4.1.7.
type RetryEntry struct {
	IssueID       string
	Identifier    string
	Attempt       int
	DueAt         time.Time
	Timer         StoppableTimer
	RetryToken    uint64
	Error         string
	WorkerHost    string
	WorkspacePath string
}

// StoppableTimer abstracts time.Timer for testability.
type StoppableTimer interface {
	Stop() bool
}

// CodexTotals holds aggregate token and runtime counters.
type CodexTotals struct {
	InputTokens    int64
	OutputTokens   int64
	TotalTokens    int64
	SecondsRunning float64
}

// TokenUsage represents token counts extracted from a Codex event.
type TokenUsage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
}
