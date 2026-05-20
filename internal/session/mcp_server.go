package session

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/hironow/sightjack/internal/domain"
)

// MCPServer is a minimal stdio-based Model Context Protocol server
// scaffolded for the refs/issues/0027 jun15 MCP pivot (Phase 2a).
//
// This is a SKELETON: only the sightjack.ping health-check tool is
// exposed. Real tools (sightjack.next_wave, sightjack.get_scan_result,
// sightjack.update_strictness, ...) ship in subsequent commits on the
// feat/jun15-mcp-pivot branch.
//
// Wire it into a claude code interactive session via --mcp-config so
// inference stays on the human-initiated session's subscription quota
// rather than crossing into the Agent SDK credit pool that gates
// `claude -p` from 2026-06-15.
//
// Protocol: JSON-RPC 2.0 over stdio, one envelope per line. Stderr
// carries human-readable diagnostics (per the project stdout/stderr
// separation invariant). Pattern follows paintress Phase 1 (ADR 0017).
type MCPServer struct {
	in     io.Reader
	out    io.Writer
	logger domain.Logger
}

// NewMCPServer wires explicit I/O so tests can drive the server
// without subprocess overhead. Passing nil for logger uses NopLogger.
func NewMCPServer(in io.Reader, out io.Writer, logger domain.Logger) *MCPServer {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	return &MCPServer{in: in, out: out, logger: logger}
}

// jsonrpcMessage is the minimum JSON-RPC 2.0 envelope this skeleton
// understands. Method-specific params decode on demand from
// Params (json.RawMessage).
type jsonrpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonrpcError   `json:"error,omitempty"`
}

type jsonrpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Serve reads messages from in line-by-line and writes responses to
// out until ctx cancels or stdin closes. Per-message decode errors
// surface as JSON-RPC error responses; only stream-level read errors
// abort Serve.
func (s *MCPServer) Serve(ctx context.Context) error {
	scanner := bufio.NewScanner(s.in)
	// 4 MiB buffer to comfortably cover D-Mail bodies in later commits.
	scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		if err := s.handle(line); err != nil {
			s.logger.Warn("mcp server: handle: %v", err)
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("mcp server: read stdin: %w", err)
	}
	return nil
}

func (s *MCPServer) handle(line []byte) error {
	var msg jsonrpcMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	switch msg.Method {
	case "tools/list":
		return s.respond(msg.ID, map[string]any{"tools": toolDescriptors()})
	case "tools/call":
		var call struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(msg.Params, &call); err != nil {
			return s.respondError(msg.ID, -32602, "invalid tools/call params")
		}
		switch call.Name {
		case "sightjack.ping":
			return s.respond(msg.ID, textResult("pong"))
		case "sightjack.next_wave":
			return s.respond(msg.ID, stubNextWave())
		case "sightjack.get_scan_result":
			return s.respond(msg.ID, stubGetScanResult(call.Arguments))
		case "sightjack.update_strictness":
			return s.respond(msg.ID, stubUpdateStrictness(call.Arguments))
		default:
			return s.respondError(msg.ID, -32601, fmt.Sprintf("unknown tool: %s", call.Name))
		}
	default:
		return s.respondError(msg.ID, -32601, fmt.Sprintf("method not implemented: %s", msg.Method))
	}
}

// toolDescriptors returns the Phase 2a MVP tool set. Each entry pins
// the interface (name, description, inputSchema) so claude code
// clients see a stable contract. The handler bodies (stubNextWave /
// stubGetScanResult / stubUpdateStrictness) are placeholders that
// ship in subsequent commits with real domain wiring.
func toolDescriptors() []map[string]any {
	return []map[string]any{
		{
			"name":        "sightjack.ping",
			"description": "Health check. Returns 'pong'.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "sightjack.next_wave",
			"description": "Return the next implementation wave (Phase 2a: stub returns a placeholder Wave payload until the domain wiring lands).",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "sightjack.get_scan_result",
			"description": "Return the latest scan result for the given session id (Phase 2a: stub echoes the requested id with a contract descriptor).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session_id": map[string]any{"type": "string"},
				},
				"required": []any{"session_id"},
			},
		},
		{
			"name":        "sightjack.update_strictness",
			"description": "Update the scan strictness level (Phase 2a: stub echoes the requested level and a placeholder previous level).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"level": map[string]any{"type": "string", "description": "strictness level: lax / normal / strict"},
				},
				"required": []any{"level"},
			},
		},
	}
}

// textResult wraps a plain string into the MCP content envelope.
func textResult(text string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
}

// jsonResult marshals data as JSON and returns an MCP content envelope.
func jsonResult(data any) map[string]any {
	body, err := json.Marshal(data)
	if err != nil {
		return textResult(fmt.Sprintf(`{"error":"marshal failed: %v"}`, err))
	}
	return map[string]any{"content": []map[string]any{{"type": "text", "text": string(body)}}}
}

// stubNextWave returns a fixed placeholder Wave payload. Replaced by
// real domain wiring (= scan event sourcing + cluster ranking) in a
// subsequent commit on feat/jun15-mcp-pivot.
func stubNextWave() map[string]any {
	return jsonResult(map[string]any{
		"stub":     true,
		"wave":     nil,
		"reason":   "phase-2a-mvp: real implementation lands when the domain wiring commit replaces this stub",
		"contract": map[string]any{"id": "string", "title": "string", "cluster_name": "string", "status": "string", "issues": "array of issue ids"},
	})
}

// stubGetScanResult echoes the requested session id with a placeholder
// scan result so claude code clients can exercise the contract
// end-to-end before the real ScanResult wiring lands.
func stubGetScanResult(args json.RawMessage) map[string]any {
	var payload struct {
		SessionID string `json:"session_id"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	return jsonResult(map[string]any{
		"stub":       true,
		"session_id": payload.SessionID,
		"result":     nil,
		"reason":     "phase-2a-mvp: real scan result fetch lands when the eventsource bridge is wired",
		"contract":   map[string]any{"clusters": "array", "completeness": "float (0..1)", "generated_at": "timestamp"},
	})
}

// stubUpdateStrictness echoes the requested level with a placeholder
// previous level.
func stubUpdateStrictness(args json.RawMessage) map[string]any {
	var payload struct {
		Level string `json:"level"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	return jsonResult(map[string]any{
		"stub":           true,
		"requested":      payload.Level,
		"previous_level": "normal",
		"reason":         "phase-2a-mvp: real strictness state wiring lands when the projection store is exposed",
	})
}

func (s *MCPServer) respond(id json.RawMessage, result any) error {
	return s.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: id, Result: result})
}

func (s *MCPServer) respondError(id json.RawMessage, code int, message string) error {
	return s.writeMessage(jsonrpcMessage{JSONRPC: "2.0", ID: id, Error: &jsonrpcError{Code: code, Message: message}})
}

func (s *MCPServer) writeMessage(msg jsonrpcMessage) error {
	out, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode response: %w", err)
	}
	if _, err := s.out.Write(append(out, '\n')); err != nil {
		return fmt.Errorf("write response: %w", err)
	}
	return nil
}
