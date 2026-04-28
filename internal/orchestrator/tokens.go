package orchestrator

import "github.com/siddhant2408/nimbus/internal/domain"

// applyTokenDelta computes and applies token deltas from absolute usage totals.
// The Codex app-server reports absolute thread totals; we compute deltas relative
// to the last reported values to avoid double-counting. SPEC Section 13.5.
func applyTokenDelta(entry *domain.RunningEntry, usage *domain.TokenUsage) {
	// Compute deltas relative to last reported absolute totals.
	inputDelta := usage.InputTokens - entry.Session.LastReportedInputTokens
	outputDelta := usage.OutputTokens - entry.Session.LastReportedOutputTokens
	totalDelta := usage.TotalTokens - entry.Session.LastReportedTotalTokens

	// Only apply positive deltas (monotonically increasing).
	if inputDelta > 0 {
		entry.Session.InputTokens += inputDelta
	}
	if outputDelta > 0 {
		entry.Session.OutputTokens += outputDelta
	}
	if totalDelta > 0 {
		entry.Session.TotalTokens += totalDelta
	}

	// Update last reported values.
	entry.Session.LastReportedInputTokens = usage.InputTokens
	entry.Session.LastReportedOutputTokens = usage.OutputTokens
	entry.Session.LastReportedTotalTokens = usage.TotalTokens
}

// accumulateGlobalTokens adds a running entry's tokens to the global totals.
func (o *Orchestrator) accumulateGlobalTokens(entry *domain.RunningEntry) {
	o.state.CodexTotals.InputTokens += entry.Session.InputTokens
	o.state.CodexTotals.OutputTokens += entry.Session.OutputTokens
	o.state.CodexTotals.TotalTokens += entry.Session.TotalTokens
}
