// Package tracker defines the interface for issue-tracker integrations.
// SPEC Section 5.3.1.
package tracker

import (
	"context"

	"github.com/siddhant2408/nimbus/internal/domain"
)

// Tracker abstracts an issue-tracker backend (e.g. Linear) so the orchestrator
// can poll for candidate issues and refresh issue state without coupling to any
// particular API.
type Tracker interface {
	// FetchCandidateIssues returns issues in the project's active states that
	// are eligible for dispatch. The tracker implementation applies assignee
	// filtering when configured.
	FetchCandidateIssues(ctx context.Context) ([]domain.Issue, error)

	// FetchIssuesByStates returns all issues whose state name matches one of
	// the given values. This is used for broader queries (e.g. fetching both
	// active and terminal states).
	FetchIssuesByStates(ctx context.Context, states []string) ([]domain.Issue, error)

	// FetchIssueStatesByIDs fetches the current state of the given issue IDs.
	// Results are returned in the same order as the input IDs (missing IDs are
	// omitted).
	FetchIssueStatesByIDs(ctx context.Context, ids []string) ([]domain.Issue, error)
}
