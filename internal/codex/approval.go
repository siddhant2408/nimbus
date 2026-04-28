package codex

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// approvalResult is the outcome of processing an approval/tool request.
type approvalResult int

const (
	approvalHandled       approvalResult = iota // Response sent, continue stream loop.
	approvalRequired                            // Approval needed from a human; abort turn.
	approvalInputRequired                       // User input needed; abort turn.
	approvalUnhandled                           // Not an approval method; caller decides.
)

// nonInteractiveToolInputAnswer is the default answer for tool user-input
// requests in a non-interactive session.
const nonInteractiveToolInputAnswer = "This is a non-interactive session. Operator input is unavailable."

// handleApprovalRequest processes approval and tool-call methods.
// It returns the approvalResult indicating how the stream loop should proceed.
func handleApprovalRequest(
	pw *protoWriter,
	method string,
	msg *jsonRPCResponse,
	autoApprove bool,
	tools *toolDispatcher,
) approvalResult {
	// msg.ID must be non-nil for all approval/tool responses.
	if msg.ID == nil {
		return approvalUnhandled
	}
	id := *msg.ID

	switch method {
	case "item/commandExecution/requestApproval":
		return approveOrRequire(pw, id, "acceptForSession", autoApprove)

	case "execCommandApproval":
		return approveOrRequire(pw, id, "approved_for_session", autoApprove)

	case "applyPatchApproval":
		return approveOrRequire(pw, id, "approved_for_session", autoApprove)

	case "item/fileChange/requestApproval":
		return approveOrRequire(pw, id, "acceptForSession", autoApprove)

	case "item/tool/call":
		return handleToolCall(pw, id, msg.Params, tools)

	case "item/tool/requestUserInput":
		return handleToolRequestUserInput(pw, id, msg.Params, autoApprove)

	default:
		return approvalUnhandled
	}
}

// approveOrRequire sends an auto-approval if autoApprove is true,
// otherwise signals that human approval is required.
func approveOrRequire(pw *protoWriter, id int, decision string, autoApprove bool) approvalResult {
	if !autoApprove {
		return approvalRequired
	}
	err := pw.sendResponse(id, map[string]any{"decision": decision})
	if err != nil {
		slog.Error("failed to send approval response", "id", id, "error", err)
	}
	slog.Debug("auto-approved request", "id", id, "decision", decision)
	return approvalHandled
}

// handleToolCall dispatches a tool/call to the tool dispatcher and sends the result.
func handleToolCall(pw *protoWriter, id int, params json.RawMessage, tools *toolDispatcher) approvalResult {
	result := tools.dispatch(params)
	result = normalizeDynamicToolResult(result)

	err := pw.sendResponse(id, result)
	if err != nil {
		slog.Error("failed to send tool call response", "id", id, "error", err)
	}
	return approvalHandled
}

// handleToolRequestUserInput processes item/tool/requestUserInput.
// When auto-approve is enabled, it searches for an approval option.
// Otherwise it responds with a non-interactive message.
func handleToolRequestUserInput(pw *protoWriter, id int, params json.RawMessage, autoApprove bool) approvalResult {
	if autoApprove {
		answers, ok := findApprovalAnswers(params)
		if ok {
			err := pw.sendResponse(id, map[string]any{"answers": answers})
			if err != nil {
				slog.Error("failed to send user input approval response", "id", id, "error", err)
			}
			slog.Debug("auto-approved user input request", "id", id)
			return approvalHandled
		}
	}

	// Fall back to non-interactive answer.
	answers, ok := buildNonInteractiveAnswers(params)
	if ok {
		err := pw.sendResponse(id, map[string]any{"answers": answers})
		if err != nil {
			slog.Error("failed to send non-interactive input response", "id", id, "error", err)
		}
		slog.Debug("answered user input with non-interactive response", "id", id)
		return approvalHandled
	}

	return approvalInputRequired
}

// findApprovalAnswers looks for approval option labels in the questions.
// Returns a map of question_id -> {answers: [label]} and true if all questions
// have a matching approval option.
func findApprovalAnswers(params json.RawMessage) (map[string]any, bool) {
	questions := extractQuestions(params)
	if len(questions) == 0 {
		return nil, false
	}

	answers := make(map[string]any, len(questions))
	for _, q := range questions {
		qID, ok := q["id"].(string)
		if !ok || qID == "" {
			return nil, false
		}

		options, ok := q["options"].([]any)
		if !ok {
			return nil, false
		}

		label := findApprovalOptionLabel(options)
		if label == "" {
			return nil, false
		}
		answers[qID] = map[string]any{"answers": []string{label}}
	}

	return answers, true
}

// findApprovalOptionLabel searches options for the best approval label.
// Priority: "Approve this Session" > "Approve Once" > any starting with "approve"/"allow".
func findApprovalOptionLabel(options []any) string {
	var labels []string
	for _, opt := range options {
		optMap, ok := opt.(map[string]any)
		if !ok {
			continue
		}
		label, ok := optMap["label"].(string)
		if !ok {
			continue
		}
		labels = append(labels, label)
	}

	// Check for exact matches first.
	for _, label := range labels {
		if label == "Approve this Session" {
			return label
		}
	}
	for _, label := range labels {
		if label == "Approve Once" {
			return label
		}
	}
	// Check for prefixes.
	for _, label := range labels {
		normalized := strings.ToLower(strings.TrimSpace(label))
		if strings.HasPrefix(normalized, "approve") || strings.HasPrefix(normalized, "allow") {
			return label
		}
	}
	return ""
}

// buildNonInteractiveAnswers builds answers for all questions using the
// non-interactive default answer. Returns the answers map and true if successful.
func buildNonInteractiveAnswers(params json.RawMessage) (map[string]any, bool) {
	questions := extractQuestions(params)
	if len(questions) == 0 {
		return nil, false
	}

	answers := make(map[string]any, len(questions))
	for _, q := range questions {
		qID, ok := q["id"].(string)
		if !ok || qID == "" {
			return nil, false
		}
		answers[qID] = map[string]any{"answers": []string{nonInteractiveToolInputAnswer}}
	}
	return answers, true
}

// extractQuestions parses the "questions" array from params.
func extractQuestions(params json.RawMessage) []map[string]any {
	var p map[string]any
	if json.Unmarshal(params, &p) != nil {
		return nil
	}
	rawQuestions, ok := p["questions"].([]any)
	if !ok {
		return nil
	}
	questions := make([]map[string]any, 0, len(rawQuestions))
	for _, rq := range rawQuestions {
		qMap, ok := rq.(map[string]any)
		if !ok {
			continue
		}
		questions = append(questions, qMap)
	}
	return questions
}

// needsInput checks if a method/payload indicates user input is required.
func needsInput(method string, msg *jsonRPCResponse) bool {
	if !strings.HasPrefix(method, "turn/") {
		return false
	}

	inputMethods := map[string]bool{
		"turn/input_required":   true,
		"turn/needs_input":      true,
		"turn/need_input":       true,
		"turn/request_input":    true,
		"turn/request_response": true,
		"turn/provide_input":    true,
		"turn/approval_required": true,
	}
	if inputMethods[method] {
		return true
	}

	return payloadRequiresInput(msg)
}

// payloadRequiresInput checks multiple fields in the message that indicate input is needed.
func payloadRequiresInput(msg *jsonRPCResponse) bool {
	// Check in both the top-level message and params.
	for _, raw := range []json.RawMessage{mustMarshal(msg), msg.Params} {
		if len(raw) == 0 {
			continue
		}
		var m map[string]any
		if json.Unmarshal(raw, &m) != nil {
			continue
		}
		if needsInputField(m) {
			return true
		}
	}
	return false
}

// needsInputField checks a single map for input-required indicator fields.
func needsInputField(m map[string]any) bool {
	if m["requiresInput"] == true || m["needsInput"] == true ||
		m["input_required"] == true || m["inputRequired"] == true {
		return true
	}
	if m["type"] == "input_required" || m["type"] == "needs_input" {
		return true
	}
	return false
}

// mustMarshal marshals v to JSON, returning nil on error.
func mustMarshal(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}

// formatApprovalError returns a descriptive error for approval failures.
func formatApprovalError(result approvalResult, method string) error {
	switch result {
	case approvalRequired:
		return fmt.Errorf("approval required for %s (auto-approve disabled)", method)
	case approvalInputRequired:
		return fmt.Errorf("user input required for %s (non-interactive session)", method)
	default:
		return fmt.Errorf("unhandled approval result for %s", method)
	}
}
