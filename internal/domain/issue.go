package domain

import "time"

// Issue is the normalized issue record used by orchestration, prompt rendering,
// and observability output. Maps to SPEC Section 4.1.1.
type Issue struct {
	ID          string
	Identifier  string
	Title       string
	Description *string
	Priority    *int // 1-4; lower is higher priority. nil = no priority.
	State       string
	BranchName  *string
	URL         *string
	AssigneeID  *string
	Labels      []string // normalized to lowercase
	BlockedBy   []BlockerRef
	CreatedAt   *time.Time
	UpdatedAt   *time.Time
}

// BlockerRef is a reference to an issue that blocks another issue.
type BlockerRef struct {
	ID         *string
	Identifier *string
	State      *string
}
