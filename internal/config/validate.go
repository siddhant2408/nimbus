package config

import (
	"fmt"
	"strings"
)

// ValidationError holds one or more config validation failures.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return "config validation failed: " + strings.Join(e.Errors, "; ")
}

// ValidateForDispatch runs the dispatch preflight checks from SPEC Section 6.3.
func ValidateForDispatch(cfg *Config) error {
	var errs []string

	if cfg.Tracker.Kind == "" {
		errs = append(errs, "tracker.kind is required")
	} else if cfg.Tracker.Kind != "linear" {
		errs = append(errs, fmt.Sprintf("tracker.kind %q is not supported (supported: linear)", cfg.Tracker.Kind))
	}

	if cfg.Tracker.APIKey == "" {
		errs = append(errs, "tracker.api_key is required (set $LINEAR_API_KEY or provide in workflow)")
	}

	if cfg.Tracker.Kind == "linear" && cfg.Tracker.ProjectSlug == "" {
		errs = append(errs, "tracker.project_slug is required when tracker.kind=linear")
	}

	if cfg.Codex.Command == "" {
		errs = append(errs, "codex.command is required and must be non-empty")
	}

	if cfg.Polling.IntervalMs <= 0 {
		errs = append(errs, "polling.interval_ms must be > 0")
	}

	if cfg.Agent.MaxConcurrentAgents <= 0 {
		errs = append(errs, "agent.max_concurrent_agents must be > 0")
	}

	if cfg.Agent.MaxTurns <= 0 {
		errs = append(errs, "agent.max_turns must be > 0")
	}

	if cfg.Agent.MaxRetryBackoffMs <= 0 {
		errs = append(errs, "agent.max_retry_backoff_ms must be > 0")
	}

	if cfg.Codex.TurnTimeoutMs <= 0 {
		errs = append(errs, "codex.turn_timeout_ms must be > 0")
	}

	if cfg.Codex.ReadTimeoutMs <= 0 {
		errs = append(errs, "codex.read_timeout_ms must be > 0")
	}

	if cfg.Hooks.TimeoutMs <= 0 {
		errs = append(errs, "hooks.timeout_ms must be > 0")
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}
