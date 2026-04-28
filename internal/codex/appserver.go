package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"time"

	"github.com/anthropics/symphony/internal/config"
	"github.com/anthropics/symphony/internal/domain"
)

// lineResult is the value sent through the shared line-reading channel.
type lineResult struct {
	line string
	err  error
}

// Session represents a running Codex app-server subprocess with an established
// JSON-RPC session (initialized + thread started). It corresponds to the Elixir
// session struct in SymphonyElixir.Codex.AppServer.
type Session struct {
	cmd    *exec.Cmd
	writer *protoWriter
	reader *protoReader
	cancel context.CancelFunc
	lines  chan lineResult // shared channel fed by a persistent reader goroutine

	threadID        string
	workspace       string
	autoApprove     bool
	approvalPolicy  any
	turnSandboxPol  map[string]any
	tools           *toolDispatcher
	tokens          *tokenTracker
	cfg             *config.CodexConfig
	pid             string // OS PID of the subprocess (as string)
	stderrDone      chan struct{}

	mu      sync.Mutex
	stopped bool
}

// StartSession launches the Codex app-server subprocess, performs the
// initialize/initialized handshake, and starts a thread.
//
// Protocol sequence:
//  1. Launch subprocess via bash -lc <command>
//  2. Send initialize (id=1) with clientInfo and capabilities
//  3. Await response for id=1
//  4. Send initialized notification
//  5. Send thread/start (id=2) with approvalPolicy, sandbox, cwd, dynamicTools
//  6. Await response for id=2, extract thread.id
func StartSession(
	ctx context.Context,
	workspacePath string,
	cfg *config.CodexConfig,
	tools []ToolSpec,
	graphqlExec GraphQLExecutor,
) (*Session, error) {
	// Create a cancellable context for the subprocess lifetime.
	subCtx, cancel := context.WithCancel(ctx)

	cmd := exec.CommandContext(subCtx, "bash", "-lc", cfg.Command)
	cmd.Dir = workspacePath

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create stdin pipe: %w", err)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start codex app-server: %w", err)
	}

	pid := ""
	if cmd.Process != nil {
		pid = fmt.Sprintf("%d", cmd.Process.Pid)
	}

	lines := make(chan lineResult, 16)
	protoRdr := newProtoReader(stdoutPipe)

	s := &Session{
		cmd:            cmd,
		writer:         newProtoWriter(stdinPipe),
		reader:         protoRdr,
		cancel:         cancel,
		lines:          lines,
		workspace:      workspacePath,
		autoApprove:    isAutoApprove(cfg.ApprovalPolicy),
		approvalPolicy: cfg.ApprovalPolicy,
		turnSandboxPol: cfg.TurnSandboxPolicy,
		tools:          newToolDispatcher(graphqlExec),
		tokens:         newTokenTracker(),
		cfg:            cfg,
		pid:            pid,
		stderrDone:     make(chan struct{}),
	}

	// Start a single persistent goroutine to read lines from stdout.
	go s.readLines()

	// Drain stderr in a background goroutine to avoid blocking the subprocess.
	go s.drainStderr(stderrPipe)

	// Perform the initialize handshake.
	if err := s.initialize(); err != nil {
		s.Stop()
		return nil, fmt.Errorf("initialize handshake: %w", err)
	}

	// Start the thread.
	threadID, err := s.startThread(tools)
	if err != nil {
		s.Stop()
		return nil, fmt.Errorf("start thread: %w", err)
	}
	s.threadID = threadID

	slog.Info("codex session started",
		"thread_id", threadID,
		"workspace", workspacePath,
		"pid", pid,
	)

	return s, nil
}

// RunTurn sends a turn/start request and streams events until the turn completes.
// The onMessage callback is invoked for each Codex event. It blocks until the
// turn finishes, fails, is cancelled, or an unrecoverable error occurs.
func (s *Session) RunTurn(
	ctx context.Context,
	prompt string,
	issue domain.Issue,
	onMessage func(domain.CodexUpdateEvent),
) error {
	turnID, err := s.startTurn(prompt, issue)
	if err != nil {
		return fmt.Errorf("start turn: %w", err)
	}

	sessionID := fmt.Sprintf("%s-%s", s.threadID, turnID)
	slog.Info("codex turn started",
		"session_id", sessionID,
		"issue_id", issue.ID,
		"issue_identifier", issue.Identifier,
	)

	// Emit session_started event.
	onMessage(domain.CodexUpdateEvent{
		Event:             "session_started",
		Timestamp:         time.Now().UTC(),
		SessionID:         sessionID,
		CodexAppServerPID: s.pid,
		Message: map[string]any{
			"session_id": sessionID,
			"thread_id":  s.threadID,
			"turn_id":    turnID,
		},
	})

	err = s.streamLoop(ctx, sessionID, onMessage)
	if err != nil {
		slog.Warn("codex turn ended with error",
			"session_id", sessionID,
			"error", err,
		)
		onMessage(domain.CodexUpdateEvent{
			Event:             "turn_ended_with_error",
			Timestamp:         time.Now().UTC(),
			SessionID:         sessionID,
			CodexAppServerPID: s.pid,
			Message: map[string]any{
				"session_id": sessionID,
				"reason":     err.Error(),
			},
		})
		return err
	}
	return nil
}

// Stop terminates the subprocess and cleans up resources.
func (s *Session) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	s.stopped = true

	s.cancel()

	// Wait for stderr drain to complete.
	<-s.stderrDone

	// Wait for the process to exit (ignore error since we cancelled it).
	_ = s.cmd.Wait()

	slog.Info("codex session stopped", "pid", s.pid)
}

// PID returns the OS process ID of the subprocess as a string.
func (s *Session) PID() string {
	return s.pid
}

// ThreadID returns the thread ID established during session startup.
func (s *Session) ThreadID() string {
	return s.threadID
}

// initialize sends the initialize request and the initialized notification.
func (s *Session) initialize() error {
	initReq := jsonRPCRequest{
		Method: "initialize",
		ID:     intPtr(initializeID),
		Params: map[string]any{
			"capabilities": map[string]any{
				"experimentalApi": true,
			},
			"clientInfo": map[string]any{
				"name":    "symphony-orchestrator",
				"title":   "Symphony Orchestrator",
				"version": "0.1.0",
			},
		},
	}

	if err := s.writer.send(initReq); err != nil {
		return fmt.Errorf("send initialize: %w", err)
	}

	if _, err := s.awaitResponse(initializeID); err != nil {
		return fmt.Errorf("await initialize response: %w", err)
	}

	// Send initialized notification (no id).
	initdNotif := jsonRPCRequest{
		Method: "initialized",
		Params: map[string]any{},
	}
	if err := s.writer.send(initdNotif); err != nil {
		return fmt.Errorf("send initialized: %w", err)
	}

	return nil
}

// startThread sends thread/start and extracts the thread ID from the response.
func (s *Session) startThread(tools []ToolSpec) (string, error) {
	// Convert tools to []any for JSON marshaling.
	dynamicTools := make([]any, len(tools))
	for i, t := range tools {
		dynamicTools[i] = t
	}

	threadReq := jsonRPCRequest{
		Method: "thread/start",
		ID:     intPtr(threadStartID),
		Params: map[string]any{
			"approvalPolicy": s.approvalPolicy,
			"sandbox":        s.cfg.ThreadSandbox,
			"cwd":            s.workspace,
			"dynamicTools":   dynamicTools,
		},
	}

	if err := s.writer.send(threadReq); err != nil {
		return "", fmt.Errorf("send thread/start: %w", err)
	}

	result, err := s.awaitResponse(threadStartID)
	if err != nil {
		return "", fmt.Errorf("await thread/start response: %w", err)
	}

	// Extract thread.id from result.
	var resultMap map[string]any
	if err := json.Unmarshal(result, &resultMap); err != nil {
		return "", fmt.Errorf("unmarshal thread/start result: %w", err)
	}

	threadPayload, ok := resultMap["thread"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid thread/start response: missing thread payload")
	}

	threadID, ok := threadPayload["id"].(string)
	if !ok {
		return "", fmt.Errorf("invalid thread/start response: missing thread.id")
	}

	return threadID, nil
}

// startTurn sends turn/start and extracts the turn ID from the response.
func (s *Session) startTurn(prompt string, issue domain.Issue) (string, error) {
	title := fmt.Sprintf("%s: %s", issue.Identifier, issue.Title)

	params := map[string]any{
		"threadId": s.threadID,
		"input": []map[string]any{
			{
				"type": "text",
				"text": prompt,
			},
		},
		"cwd":            s.workspace,
		"title":          title,
		"approvalPolicy": s.approvalPolicy,
	}
	if s.turnSandboxPol != nil {
		params["sandboxPolicy"] = s.turnSandboxPol
	}

	turnReq := jsonRPCRequest{
		Method: "turn/start",
		ID:     intPtr(turnStartID),
		Params: params,
	}

	if err := s.writer.send(turnReq); err != nil {
		return "", fmt.Errorf("send turn/start: %w", err)
	}

	result, err := s.awaitResponse(turnStartID)
	if err != nil {
		return "", fmt.Errorf("await turn/start response: %w", err)
	}

	var resultMap map[string]any
	if err := json.Unmarshal(result, &resultMap); err != nil {
		return "", fmt.Errorf("unmarshal turn/start result: %w", err)
	}

	turnPayload, ok := resultMap["turn"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid turn/start response: missing turn payload")
	}

	turnID, ok := turnPayload["id"].(string)
	if !ok {
		return "", fmt.Errorf("invalid turn/start response: missing turn.id")
	}

	return turnID, nil
}

// readLines is the persistent goroutine that feeds s.lines from stdout.
// It runs until the reader returns an error (including io.EOF).
func (s *Session) readLines() {
	defer close(s.lines)
	for {
		line, err := s.reader.readLine()
		s.lines <- lineResult{line: line, err: err}
		if err != nil {
			return
		}
	}
}

// awaitResponse reads lines until we get a response matching the given request ID.
// Non-matching messages are logged and skipped.
func (s *Session) awaitResponse(requestID int) (json.RawMessage, error) {
	timeoutMs := s.cfg.ReadTimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = 5000
	}
	deadline := time.After(time.Duration(timeoutMs) * time.Millisecond)

	for {
		select {
		case <-deadline:
			return nil, fmt.Errorf("response timeout waiting for id=%d", requestID)

		case lr, ok := <-s.lines:
			if !ok {
				return nil, fmt.Errorf("subprocess exited while waiting for id=%d", requestID)
			}
			if lr.err != nil {
				if errors.Is(lr.err, io.EOF) {
					return nil, fmt.Errorf("subprocess exited while waiting for id=%d", requestID)
				}
				return nil, fmt.Errorf("read error waiting for id=%d: %w", requestID, lr.err)
			}

			resp := parseLine(lr.line)
			if resp == nil {
				continue
			}

			// Check if this response matches our request ID.
			if resp.ID != nil && *resp.ID == requestID {
				if len(resp.Error) > 0 {
					return nil, fmt.Errorf("response error for id=%d: %s", requestID, string(resp.Error))
				}
				return resp.Result, nil
			}

			// Not our response; log and continue.
			slog.Debug("ignoring message while awaiting response",
				"expected_id", requestID,
				"message_method", resp.Method,
			)
		}
	}
}

// streamLoop reads the event stream during a turn, handling approvals,
// tool calls, and emitting events until the turn completes.
func (s *Session) streamLoop(
	ctx context.Context,
	sessionID string,
	onMessage func(domain.CodexUpdateEvent),
) error {
	turnTimeoutMs := s.cfg.TurnTimeoutMs
	if turnTimeoutMs <= 0 {
		turnTimeoutMs = 3600000
	}
	deadline := time.After(time.Duration(turnTimeoutMs) * time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-deadline:
			return fmt.Errorf("turn timeout after %dms", turnTimeoutMs)

		case lr, ok := <-s.lines:
			if !ok {
				return fmt.Errorf("subprocess exited during turn")
			}
			if lr.err != nil {
				if errors.Is(lr.err, io.EOF) {
					return fmt.Errorf("subprocess exited during turn")
				}
				return fmt.Errorf("read error during turn: %w", lr.err)
			}

			resp := parseLine(lr.line)
			if resp == nil {
				// Non-JSON line. If it looks like a protocol message, emit malformed event.
				if protocolMessageCandidate(lr.line) {
					onMessage(domain.CodexUpdateEvent{
						Event:             "malformed",
						Timestamp:         time.Now().UTC(),
						SessionID:         sessionID,
						CodexAppServerPID: s.pid,
						Message: map[string]any{
							"raw": lr.line,
						},
					})
				}
				continue
			}

			method := resp.Method
			if err := s.handleStreamMessage(resp, method, sessionID, onMessage); err != nil {
				return err
			}

			// Check for terminal methods.
			if method == "turn/completed" {
				return nil
			}
		}
	}
}

// handleStreamMessage processes a single message from the turn event stream.
func (s *Session) handleStreamMessage(
	msg *jsonRPCResponse,
	method string,
	sessionID string,
	onMessage func(domain.CodexUpdateEvent),
) error {
	// Extract usage and rate limits for every message.
	usage := extractUsage(msg)
	deltaUsage := s.tokens.computeDelta(usage)
	rateLimits := extractRateLimits(msg)

	// Build base event.
	baseEvent := domain.CodexUpdateEvent{
		Timestamp:         time.Now().UTC(),
		SessionID:         sessionID,
		CodexAppServerPID: s.pid,
		Usage:             deltaUsage,
		RateLimits:        rateLimits,
	}

	switch method {
	case "turn/completed":
		baseEvent.Event = "turn_completed"
		baseEvent.Message = rawToMap(msg)
		onMessage(baseEvent)
		return nil

	case "turn/failed":
		baseEvent.Event = "turn_failed"
		baseEvent.Message = rawToMap(msg)
		onMessage(baseEvent)
		return fmt.Errorf("turn failed: %s", truncateRaw(msg.Params, 500))

	case "turn/cancelled":
		baseEvent.Event = "turn_cancelled"
		baseEvent.Message = rawToMap(msg)
		onMessage(baseEvent)
		return fmt.Errorf("turn cancelled: %s", truncateRaw(msg.Params, 500))

	default:
		// Try handling as an approval or tool request.
		result := handleApprovalRequest(s.writer, method, msg, s.autoApprove, s.tools)
		switch result {
		case approvalHandled:
			baseEvent.Event = "approval_auto_approved"
			baseEvent.Message = rawToMap(msg)
			onMessage(baseEvent)
			return nil

		case approvalRequired:
			baseEvent.Event = "approval_required"
			baseEvent.Message = rawToMap(msg)
			onMessage(baseEvent)
			return formatApprovalError(result, method)

		case approvalInputRequired:
			baseEvent.Event = "turn_input_required"
			baseEvent.Message = rawToMap(msg)
			onMessage(baseEvent)
			return formatApprovalError(result, method)

		case approvalUnhandled:
			// Check if this method indicates user input is needed.
			if needsInput(method, msg) {
				baseEvent.Event = "turn_input_required"
				baseEvent.Message = rawToMap(msg)
				onMessage(baseEvent)
				return fmt.Errorf("turn requires input: %s", method)
			}

			// Regular notification.
			baseEvent.Event = "notification"
			baseEvent.Message = rawToMap(msg)
			onMessage(baseEvent)
			slog.Debug("codex notification", "method", method)
			return nil
		}
	}

	return nil
}

// drainStderr reads stderr line by line and logs it. This runs in a goroutine
// to prevent the subprocess from blocking on stderr writes.
func (s *Session) drainStderr(stderr io.Reader) {
	defer close(s.stderrDone)
	scanner := bufio.NewScanner(stderr)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTokenSize)
	for scanner.Scan() {
		logNonJSONLine(scanner.Text())
	}
}

// rawToMap converts a jsonRPCResponse to a map[string]any for event payloads.
func rawToMap(msg *jsonRPCResponse) map[string]any {
	data, err := json.Marshal(msg)
	if err != nil {
		return map[string]any{"raw_error": err.Error()}
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]any{"raw": string(data)}
	}
	return m
}

// truncateRaw converts raw JSON to a string, truncating if longer than maxLen.
func truncateRaw(raw json.RawMessage, maxLen int) string {
	s := string(raw)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

// isAutoApprove checks if the approval policy indicates automatic approval.
func isAutoApprove(policy any) bool {
	switch p := policy.(type) {
	case string:
		return p == "never"
	default:
		return false
	}
}

// intPtr returns a pointer to an int.
func intPtr(i int) *int {
	return &i
}
