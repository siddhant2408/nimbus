package codex

import (
	"encoding/json"

	"github.com/siddhant2408/nimbus/internal/domain"
)

// tokenTracker tracks cumulative token usage and computes deltas.
type tokenTracker struct {
	lastInput  int64
	lastOutput int64
	lastTotal  int64
}

// newTokenTracker returns a zero-initialized token tracker.
func newTokenTracker() *tokenTracker {
	return &tokenTracker{}
}

// extractUsage tries to find token usage information from a JSON-RPC message.
// It searches multiple nested paths matching the Elixir implementation:
//   - top-level "usage" field
//   - params.msg.payload.info.total_token_usage
//   - params.usage
//
// Returns nil if no usage data is found.
func extractUsage(msg *jsonRPCResponse) *domain.TokenUsage {
	// Try top-level usage field.
	if usage := parseUsageFromRaw(msg.Usage); usage != nil {
		return usage
	}

	// Try nested paths in params.
	if len(msg.Params) == 0 {
		return nil
	}

	var params map[string]any
	if json.Unmarshal(msg.Params, &params) != nil {
		return nil
	}

	// Try params.usage directly.
	if usage := parseUsageFromAny(params["usage"]); usage != nil {
		return usage
	}

	// Try params.msg.payload.info.total_token_usage
	if msgVal, ok := params["msg"].(map[string]any); ok {
		if payload, ok := msgVal["payload"].(map[string]any); ok {
			if info, ok := payload["info"].(map[string]any); ok {
				if usage := parseUsageFromAny(info["total_token_usage"]); usage != nil {
					return usage
				}
			}
		}
	}

	return nil
}

// computeDelta returns a TokenUsage representing the delta since the last
// reported values and updates the tracker's state. Returns nil if there is
// no change.
func (t *tokenTracker) computeDelta(usage *domain.TokenUsage) *domain.TokenUsage {
	if usage == nil {
		return nil
	}

	deltaInput := usage.InputTokens - t.lastInput
	deltaOutput := usage.OutputTokens - t.lastOutput
	deltaTotal := usage.TotalTokens - t.lastTotal

	if deltaInput == 0 && deltaOutput == 0 && deltaTotal == 0 {
		return nil
	}

	// Update last reported values.
	t.lastInput = usage.InputTokens
	t.lastOutput = usage.OutputTokens
	t.lastTotal = usage.TotalTokens

	return &domain.TokenUsage{
		InputTokens:  deltaInput,
		OutputTokens: deltaOutput,
		TotalTokens:  deltaTotal,
	}
}

// extractRateLimits tries to find rate limit information from a JSON-RPC message.
// It searches nested maps for keys like limit_id/limit_name with primary/secondary/credits values.
func extractRateLimits(msg *jsonRPCResponse) map[string]any {
	if len(msg.Params) == 0 {
		return nil
	}

	var params map[string]any
	if json.Unmarshal(msg.Params, &params) != nil {
		return nil
	}

	// Search in params.msg.payload.info for rate limit data.
	if msgVal, ok := params["msg"].(map[string]any); ok {
		if payload, ok := msgVal["payload"].(map[string]any); ok {
			if info, ok := payload["info"].(map[string]any); ok {
				if rl := findRateLimits(info); rl != nil {
					return rl
				}
			}
		}
	}

	// Also search top-level params for rate_limits or rateLimits.
	for _, key := range []string{"rate_limits", "rateLimits"} {
		if rl, ok := params[key].(map[string]any); ok && len(rl) > 0 {
			return rl
		}
	}

	return nil
}

// findRateLimits searches a map for rate limit structures.
// Returns the first map found that has limit_id/limit_name with recognized value keys.
func findRateLimits(info map[string]any) map[string]any {
	for _, key := range []string{"rate_limits", "rateLimits"} {
		if rl, ok := info[key].(map[string]any); ok && len(rl) > 0 {
			return rl
		}
		if rl, ok := info[key].([]any); ok && len(rl) > 0 {
			return rateLimitSliceToMap(rl)
		}
	}
	return nil
}

// rateLimitSliceToMap converts a slice of rate limit entries to a map keyed by limit ID.
func rateLimitSliceToMap(entries []any) map[string]any {
	result := make(map[string]any, len(entries))
	for _, entry := range entries {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		// Try limit_id first, then limit_name.
		id := ""
		for _, key := range []string{"limit_id", "limit_name", "limitId", "limitName"} {
			if v, ok := m[key].(string); ok && v != "" {
				id = v
				break
			}
		}
		if id != "" {
			result[id] = m
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// parseUsageFromRaw tries to parse token usage from a json.RawMessage.
func parseUsageFromRaw(raw json.RawMessage) *domain.TokenUsage {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return parseUsageMap(m)
}

// parseUsageFromAny tries to parse token usage from an any value (expected map[string]any).
func parseUsageFromAny(v any) *domain.TokenUsage {
	m, ok := v.(map[string]any)
	if !ok || m == nil {
		return nil
	}
	return parseUsageMap(m)
}

// parseUsageMap extracts token counts from a usage map.
// Looks for input_tokens/output_tokens/total_tokens (snake_case)
// and inputTokens/outputTokens/totalTokens (camelCase).
func parseUsageMap(m map[string]any) *domain.TokenUsage {
	input := toInt64(m, "input_tokens", "inputTokens")
	output := toInt64(m, "output_tokens", "outputTokens")
	total := toInt64(m, "total_tokens", "totalTokens")

	if input == 0 && output == 0 && total == 0 {
		return nil
	}
	return &domain.TokenUsage{
		InputTokens:  input,
		OutputTokens: output,
		TotalTokens:  total,
	}
}

// toInt64 extracts an int64 from a map trying multiple key names.
func toInt64(m map[string]any, keys ...string) int64 {
	for _, key := range keys {
		v, ok := m[key]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case float64:
			return int64(n)
		case int:
			return int64(n)
		case int64:
			return n
		case json.Number:
			if i, err := n.Int64(); err == nil {
				return i
			}
		}
	}
	return 0
}
