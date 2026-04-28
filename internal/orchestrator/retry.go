package orchestrator

import (
	"log/slog"
	"time"

	"github.com/anthropics/symphony/internal/domain"
)

const (
	continuationRetryDelayMs = 1000  // 1 second for continuation retries
	failureRetryBaseMs       = 10000 // 10 seconds base for failure retries
)

// scheduleRetry creates or replaces a retry entry for an issue. SPEC Section 8.4.
func (o *Orchestrator) scheduleRetry(issueID, identifier string, attempt int, errMsg string, continuation bool) {
	// Cancel any existing retry timer for this issue.
	if existing, ok := o.state.RetryAttempts[issueID]; ok {
		if existing.Timer != nil {
			existing.Timer.Stop()
		}
	}

	delay := retryDelay(attempt, continuation, o.currentConfig().Agent.MaxRetryBackoffMs)
	dueAt := time.Now().Add(delay)
	token := o.nextRetryToken.Add(1)

	timer := time.AfterFunc(delay, func() {
		o.RetryTimerCh <- domain.RetryTimerEvent{
			IssueID:    issueID,
			RetryToken: token,
		}
	})

	o.state.RetryAttempts[issueID] = &domain.RetryEntry{
		IssueID:    issueID,
		Identifier: identifier,
		Attempt:    attempt,
		DueAt:      dueAt,
		Timer:      timer,
		RetryToken: token,
		Error:      errMsg,
	}

	slog.Info("retry scheduled",
		"issue_id", issueID,
		"issue_identifier", identifier,
		"attempt", attempt,
		"delay_ms", delay.Milliseconds(),
		"continuation", continuation,
	)
}

// retryDelay computes the retry delay. SPEC Section 8.4.
func retryDelay(attempt int, continuation bool, maxBackoffMs int) time.Duration {
	if continuation && attempt == 1 {
		return time.Duration(continuationRetryDelayMs) * time.Millisecond
	}
	return failureRetryDelay(attempt, maxBackoffMs)
}

// failureRetryDelay computes exponential backoff: min(10000 * 2^(attempt-1), max).
func failureRetryDelay(attempt, maxBackoffMs int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}

	// Cap the exponent to prevent overflow.
	exp := attempt - 1
	if exp > 10 {
		exp = 10
	}

	delayMs := failureRetryBaseMs * (1 << exp)
	if delayMs > maxBackoffMs {
		delayMs = maxBackoffMs
	}

	return time.Duration(delayMs) * time.Millisecond
}
