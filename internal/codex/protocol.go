// Package codex implements the Codex app-server JSON-RPC 2.0 client over stdio.
package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
)

// JSON-RPC request ID constants matching the Elixir implementation.
const (
	initializeID  = 1
	threadStartID = 2
	turnStartID   = 3
)

// maxTokenSize is the maximum line size for the scanner (10 MB).
const maxTokenSize = 10 * 1024 * 1024

// maxStreamLogBytes caps the length of non-JSON stderr/stdout lines logged.
const maxStreamLogBytes = 1000

// jsonRPCRequest is a JSON-RPC 2.0 request or notification sent to the app-server.
type jsonRPCRequest struct {
	Method string `json:"method,omitempty"`
	ID     *int   `json:"id,omitempty"`
	Params any    `json:"params,omitempty"`
	Result any    `json:"result,omitempty"`
}

// jsonRPCResponse is a JSON-RPC 2.0 response received from the app-server.
type jsonRPCResponse struct {
	ID     *int            `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  json.RawMessage `json:"error,omitempty"`
	Usage  json.RawMessage `json:"usage,omitempty"`
}

// protoWriter sends newline-delimited JSON to the subprocess stdin.
type protoWriter struct {
	mu sync.Mutex
	w  io.Writer
}

// newProtoWriter creates a protocol writer around the given writer.
func newProtoWriter(w io.Writer) *protoWriter {
	return &protoWriter{w: w}
}

// send marshals msg as JSON and writes it as a single line followed by "\n".
func (pw *protoWriter) send(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal JSON-RPC message: %w", err)
	}
	pw.mu.Lock()
	defer pw.mu.Unlock()
	if _, err := pw.w.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write JSON-RPC message: %w", err)
	}
	return nil
}

// sendResponse sends a response with the given id and result payload.
func (pw *protoWriter) sendResponse(id int, result any) error {
	msg := struct {
		ID     int `json:"id"`
		Result any `json:"result"`
	}{
		ID:     id,
		Result: result,
	}
	return pw.send(msg)
}

// protoReader reads newline-delimited JSON from the subprocess stdout.
type protoReader struct {
	scanner *bufio.Scanner
}

// newProtoReader creates a protocol reader with a 10 MB line buffer.
func newProtoReader(r io.Reader) *protoReader {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxTokenSize)
	return &protoReader{scanner: scanner}
}

// readLine reads the next line from the scanner, blocking until data is available.
// Returns io.EOF when the stream is closed.
func (pr *protoReader) readLine() (string, error) {
	if pr.scanner.Scan() {
		return pr.scanner.Text(), nil
	}
	if err := pr.scanner.Err(); err != nil {
		return "", fmt.Errorf("read JSON-RPC stream: %w", err)
	}
	return "", io.EOF
}

// parseLine attempts to decode a line as a JSON-RPC response.
// Non-JSON lines are logged and a nil response is returned.
func parseLine(line string) *jsonRPCResponse {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	var resp jsonRPCResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		logNonJSONLine(line)
		return nil
	}
	return &resp
}

// logNonJSONLine logs a non-JSON line from the subprocess stream, truncating if needed.
func logNonJSONLine(line string) {
	text := strings.TrimSpace(line)
	if len(text) > maxStreamLogBytes {
		text = text[:maxStreamLogBytes]
	}
	if text == "" {
		return
	}
	lower := strings.ToLower(text)
	if strings.Contains(lower, "error") || strings.Contains(lower, "warn") ||
		strings.Contains(lower, "failed") || strings.Contains(lower, "fatal") ||
		strings.Contains(lower, "panic") || strings.Contains(lower, "exception") {
		slog.Warn("codex stream output", "text", text)
	} else {
		slog.Debug("codex stream output", "text", text)
	}
}

// protocolMessageCandidate checks whether a line looks like it could be a JSON-RPC message.
func protocolMessageCandidate(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "{")
}
