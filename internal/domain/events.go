package domain

import "time"

// WorkerExitEvent is sent when a worker goroutine finishes.
type WorkerExitEvent struct {
	IssueID string
	Err     error // nil = normal exit
}

// CodexUpdateEvent carries a Codex app-server event to the orchestrator.
type CodexUpdateEvent struct {
	IssueID           string
	Event             string
	Timestamp         time.Time
	SessionID         string
	CodexAppServerPID string
	Usage             *TokenUsage
	RateLimits        map[string]any
	Message           map[string]any
	TurnCount         int
}

// WorkerRuntimeInfoEvent carries workspace/host info from a started worker.
type WorkerRuntimeInfoEvent struct {
	IssueID       string
	WorkerHost    string
	WorkspacePath string
	PersonaName   string
}

// RetryTimerEvent fires when a retry timer expires.
type RetryTimerEvent struct {
	IssueID    string
	RetryToken uint64
}

// SnapshotRequest asks the orchestrator for a point-in-time state snapshot.
type SnapshotRequest struct {
	Reply chan<- *Snapshot
}

// RefreshRequest triggers an immediate poll cycle.
type RefreshRequest struct {
	Reply chan<- RefreshResponse
}

// RefreshResponse is the result of a manual refresh trigger.
type RefreshResponse struct {
	Queued      bool      `json:"queued"`
	Coalesced   bool      `json:"coalesced"`
	RequestedAt time.Time `json:"requested_at"`
}

// Snapshot is a point-in-time view of orchestrator state for the API/dashboard.
type Snapshot struct {
	GeneratedAt time.Time             `json:"generated_at"`
	Counts      SnapshotCounts        `json:"counts"`
	Running     []SnapshotRunning     `json:"running"`
	Retrying    []SnapshotRetrying    `json:"retrying"`
	CodexTotals SnapshotCodexTotals   `json:"codex_totals"`
	RateLimits  map[string]any        `json:"rate_limits"`
}

// SnapshotCounts holds summary counts.
type SnapshotCounts struct {
	Running  int `json:"running"`
	Retrying int `json:"retrying"`
}

// SnapshotRunning represents one running session in the snapshot.
type SnapshotRunning struct {
	IssueID       string         `json:"issue_id"`
	Identifier    string         `json:"issue_identifier"`
	State         string         `json:"state"`
	SessionID     string         `json:"session_id"`
	TurnCount     int            `json:"turn_count"`
	Persona       string         `json:"persona"`
	LastEvent     string         `json:"last_event"`
	LastMessage   string         `json:"last_message"`
	StartedAt     time.Time      `json:"started_at"`
	LastEventAt   *time.Time     `json:"last_event_at"`
	Tokens        SnapshotTokens `json:"tokens"`
	WorkerHost    string         `json:"worker_host,omitempty"`
	WorkspacePath string         `json:"workspace_path,omitempty"`
}

// SnapshotRetrying represents one retry queue entry in the snapshot.
type SnapshotRetrying struct {
	IssueID    string `json:"issue_id"`
	Identifier string `json:"issue_identifier"`
	Attempt    int    `json:"attempt"`
	DueAt      string `json:"due_at"`
	Error      string `json:"error"`
	WorkerHost string `json:"worker_host,omitempty"`
}

// SnapshotTokens holds per-session token counts.
type SnapshotTokens struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	TotalTokens  int64 `json:"total_tokens"`
}

// SnapshotCodexTotals holds aggregate token and runtime totals.
type SnapshotCodexTotals struct {
	InputTokens    int64   `json:"input_tokens"`
	OutputTokens   int64   `json:"output_tokens"`
	TotalTokens    int64   `json:"total_tokens"`
	SecondsRunning float64 `json:"seconds_running"`
}
