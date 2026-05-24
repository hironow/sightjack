package session

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/platform"
)

// MCPServer is a stdio-based Model Context Protocol server for the
// refs/issues/0027 jun15 MCP pivot.
//
// All four tools are real implementations: sightjack.ping (health
// check), sightjack.next_wave + sightjack.get_scan_result (read the
// session's scan dir under .siren/.run/<session_id>/), and
// sightjack.update_strictness (atomically updates .siren/config.yaml).
//
// Wire it into a claude code interactive session via --mcp-config so
// inference stays on the human-initiated session's subscription quota
// rather than crossing into the Agent SDK credit pool that gates
// `claude -p` from 2026-06-15.
//
// Protocol: JSON-RPC 2.0 over stdio, one envelope per line. Stderr
// carries human-readable diagnostics (per the project stdout/stderr
// separation invariant). Pattern follows paintress Phase 1 (ADR 0017)
// + paintress Phase 3 real impl (= e84988b / 83cb3ca) WithContinent
// pattern.
//
// baseDir is the project root used to resolve config / scan results /
// wave plan paths. When empty, real-impl tools return uninitialized.
type MCPServer struct {
	in      io.Reader
	out     io.Writer
	logger  domain.Logger
	baseDir string
}

// NewMCPServer wires explicit I/O so tests can drive the server
// without subprocess overhead. Passing nil for logger uses NopLogger.
func NewMCPServer(in io.Reader, out io.Writer, logger domain.Logger) *MCPServer {
	if logger == nil {
		logger = &domain.NopLogger{}
	}
	return &MCPServer{in: in, out: out, logger: logger}
}

// WithBaseDir sets the project root used by real-impl MCP tools to
// resolve config / scan results / wave plan paths. Returns s for
// chaining (= paintress.WithContinent symmetric).
func (s *MCPServer) WithBaseDir(baseDir string) *MCPServer {
	s.baseDir = baseDir
	return s
}

// jsonrpcMessage is the minimum JSON-RPC 2.0 envelope this server
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
		if err := s.handle(ctx, line); err != nil {
			s.logger.Warn("mcp server: handle: %v", err)
		}
	}
	if err := scanner.Err(); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("mcp server: read stdin: %w", err)
	}
	return nil
}

func (s *MCPServer) handle(ctx context.Context, line []byte) error {
	var msg jsonrpcMessage
	if err := json.Unmarshal(line, &msg); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	switch msg.Method {
	case "initialize":
		return s.respond(msg.ID, initializeResult())
	case "notifications/initialized":
		// JSON-RPC notification (no id): the client signals it finished
		// the handshake. No response is sent.
		return nil
	case "tools/list":
		return s.respond(msg.ID, map[string]any{"tools": toolDescriptors()})
	case "tools/call":
		return s.handleToolsCall(ctx, msg)
	default:
		// Unknown notifications (no id) are ignored per JSON-RPC; only
		// id-bearing requests get a method-not-found error.
		if len(msg.ID) == 0 {
			return nil
		}
		return s.respondError(msg.ID, -32601, fmt.Sprintf("method not implemented: %s", msg.Method))
	}
}

// mcpProtocolVersion is the single MCP protocol version this server
// implements. Per the MCP lifecycle spec, the server returns the
// version it actually supports (not an echo of the client's request):
// echoing an unsupported client version would falsely claim support
// and break future / draft clients. The client decides compatibility
// from this value.
const mcpProtocolVersion = "2024-11-05"

// initializeResult builds the MCP initialize handshake response. The
// claude code session sends `initialize` first; without a valid reply
// it never proceeds to tools/list. The server advertises its supported
// protocol version + the tools capability.
func initializeResult() map[string]any {
	return map[string]any{
		"protocolVersion": mcpProtocolVersion,
		"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
		"serverInfo":      map[string]any{"name": "sightjack", "version": "0.1.0"},
	}
}

// handleToolsCall dispatches a single tools/call request and records
// MCP invocation metrics (mcp.tool.invocations counter +
// mcp.tool.duration histogram) for cost-monitoring verification post
// 2026-06-15 (refs/issues/0027 Phase 3 cost monitoring (a)).
func (s *MCPServer) handleToolsCall(ctx context.Context, msg jsonrpcMessage) error {
	start := time.Now()
	var call struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(msg.Params, &call); err != nil {
		platform.RecordMCPInvocation(ctx, "", "error", time.Since(start))
		return s.respondError(msg.ID, -32602, "invalid tools/call params")
	}

	status := "ok"
	var result map[string]any
	switch call.Name {
	case "sightjack.ping":
		result = textResult("pong")
	case "sightjack.next_wave":
		result = realNextWave(s.baseDir, call.Arguments)
	case "sightjack.get_scan_result":
		result = realGetScanResult(s.baseDir, call.Arguments)
	case "sightjack.update_strictness":
		result = realUpdateStrictness(s.baseDir, call.Arguments)
	default:
		platform.RecordMCPInvocation(ctx, call.Name, "error", time.Since(start))
		return s.respondError(msg.ID, -32601, fmt.Sprintf("unknown tool: %s", call.Name))
	}

	err := s.respond(msg.ID, result)
	if err != nil {
		status = "error"
	}
	platform.RecordMCPInvocation(ctx, call.Name, status, time.Since(start))
	return err
}

// toolDescriptors returns the tool set. Each entry pins the interface
// (name, description, inputSchema) so claude code clients see a stable
// contract. All 3 non-ping tools are real impl: next_wave +
// get_scan_result read from .siren/.run/<session_id>/ scan + wave JSON
// files; update_strictness writes the strictness default to
// .siren/config.yaml atomically.
func toolDescriptors() []map[string]any {
	return []map[string]any{
		{
			"name":        "sightjack.ping",
			"description": "Health check. Returns 'pong'.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "sightjack.next_wave",
			"description": "Return the first 'available' wave from wave_*.json files in the session's scan dir, plus a count of total / available waves. Requires session_id (= use `sightjack sessions list` to look up).",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"session_id": map[string]any{"type": "string"},
				},
				"required": []any{"session_id"},
			},
		},
		{
			"name":        "sightjack.get_scan_result",
			"description": "Return aggregated cluster info for the given session (cluster count + names + average completeness). For full per-cluster detail, read scan_dir/cluster_*.json directly.",
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
			"description": "Update the default scan strictness in .siren/config.yaml and persist it atomically. Validates the level (fog / alert / lockdown) before write.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"level": map[string]any{"type": "string", "description": "strictness level: fog / alert / lockdown"},
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

// realNextWave reads wave_*.json files from the session's scan
// directory (.siren/.run/<session_id>/) and returns the first wave
// with status='available' (= ready to be picked) plus a count of
// total / available waves. The session itself decides whether to
// claim the wave (= write to outbox / call apply via skill).
//
// Pattern: paintress.next_issue (= 83cb3ca) symmetric copy for the
// scan/wave domain. baseDir is the project root from
// MCPServer.WithBaseDir; session_id is the args arg.
func realNextWave(baseDir string, args json.RawMessage) map[string]any {
	var payload struct {
		SessionID string `json:"session_id"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	if baseDir == "" {
		return jsonResult(map[string]any{
			"initialized": false,
			"reason":      "sightjack mcp baseDir not configured (start `sightjack mcp` from the project root)",
		})
	}
	if payload.SessionID == "" {
		return jsonResult(map[string]any{
			"initialized": true,
			"reason":      "session_id required: query `sightjack sessions list` first or pass an explicit session_id",
		})
	}
	scanDir := domain.ScanDir(baseDir, payload.SessionID)
	entries, err := os.ReadDir(scanDir)
	if err != nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"session_id":  payload.SessionID,
			"scan_dir":    scanDir,
			"reason":      fmt.Sprintf("scan dir read failed: %v", err),
		})
	}
	var availableWaves []map[string]any
	totalWaves := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "wave_") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		result, err := ParseWaveGenerateResult(filepath.Join(scanDir, e.Name()))
		if err != nil {
			continue
		}
		for _, w := range result.Waves {
			totalWaves++
			if w.Status == "available" {
				availableWaves = append(availableWaves, map[string]any{
					"id":           w.ID,
					"cluster_name": w.ClusterName,
					"title":        w.Title,
					"status":       w.Status,
				})
			}
		}
	}
	var nextWave map[string]any
	if len(availableWaves) > 0 {
		nextWave = availableWaves[0]
	}
	return jsonResult(map[string]any{
		"initialized":     true,
		"session_id":      payload.SessionID,
		"scan_dir":        scanDir,
		"total_waves":     totalWaves,
		"available_waves": len(availableWaves),
		"next_wave":       nextWave,
		"all_available":   availableWaves,
		"instruction":     "Pick a wave from all_available by id, then run `sightjack apply` (via skill workflow) to claim it.",
	})
}

// realGetScanResult reads cluster_*.json files from the session's
// scan directory and returns aggregated cluster info (= count +
// names + average completeness). Phase 3 scope: read-only summary,
// not the full ClusterScanResult shape — for full detail the
// session can read individual files at scan_dir/cluster_<n>.json.
//
// Pattern: paintress.next_issue (= 83cb3ca) symmetric copy.
func realGetScanResult(baseDir string, args json.RawMessage) map[string]any {
	var payload struct {
		SessionID string `json:"session_id"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	if baseDir == "" {
		return jsonResult(map[string]any{
			"initialized": false,
			"reason":      "sightjack mcp baseDir not configured",
		})
	}
	if payload.SessionID == "" {
		return jsonResult(map[string]any{
			"initialized": true,
			"reason":      "session_id required: query `sightjack sessions list` first or pass an explicit session_id",
		})
	}
	scanDir := domain.ScanDir(baseDir, payload.SessionID)
	entries, err := os.ReadDir(scanDir)
	if err != nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"session_id":  payload.SessionID,
			"scan_dir":    scanDir,
			"reason":      fmt.Sprintf("scan dir read failed: %v", err),
		})
	}
	clusterNames := make([]string, 0)
	totalCompleteness := 0.0
	clusterCount := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "cluster_") || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		result, err := ParseClusterScanResult(filepath.Join(scanDir, e.Name()))
		if err != nil {
			continue
		}
		clusterNames = append(clusterNames, result.Name)
		totalCompleteness += result.Completeness
		clusterCount++
	}
	avgCompleteness := 0.0
	if clusterCount > 0 {
		avgCompleteness = totalCompleteness / float64(clusterCount)
	}
	return jsonResult(map[string]any{
		"initialized":          true,
		"session_id":           payload.SessionID,
		"scan_dir":             scanDir,
		"cluster_count":        clusterCount,
		"cluster_names":        clusterNames,
		"average_completeness": avgCompleteness,
		"instruction":          "For full cluster detail, read scan_dir/cluster_<name>.json files directly.",
	})
}

// realUpdateStrictness reads the current default strictness from the
// sightjack config (.siren/config.yaml), validates the requested
// level, and writes the new level back to the config via
// UpdateConfig (= atomic load + setConfigField + validate + write).
// Returns the old + new levels with persistence='persisted'.
//
// baseDir is the project root from MCPServer.WithBaseDir. When empty
// or the config is missing, the response signals uninitialized so the
// session surfaces a clear error to the operator.
//
// Validation: requested level must be one of fog / alert / lockdown
// (= domain.StrictnessLevel.Valid()). Invalid input returns
// persisted=false + reason without touching the config.
//
// Pattern: paintress.update_gradient (= 83cb3ca) plus atomic
// persistence via the existing session.UpdateConfig path.
func realUpdateStrictness(baseDir string, args json.RawMessage) map[string]any {
	var payload struct {
		Level string `json:"level"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	if baseDir == "" {
		return jsonResult(map[string]any{
			"initialized":   false,
			"reason":        "sightjack mcp baseDir not configured (start `sightjack mcp` from the project root or pass via WithBaseDir)",
			"requested":     payload.Level,
			"current_level": "",
		})
	}
	cfgPath := domain.ConfigPath(baseDir)
	cfg, err := LoadConfig(cfgPath)
	if err != nil {
		return jsonResult(map[string]any{
			"initialized":   false,
			"reason":        fmt.Sprintf("config load failed: %v", err),
			"requested":     payload.Level,
			"current_level": "",
		})
	}
	currentLevel := string(cfg.Strictness.Default)

	// Validate requested level before touching the file.
	if !domain.StrictnessLevel(payload.Level).Valid() {
		return jsonResult(map[string]any{
			"initialized":   true,
			"persisted":     false,
			"reason":        fmt.Sprintf("invalid strictness level %q: must be fog, alert, or lockdown", payload.Level),
			"current_level": currentLevel,
			"requested":     payload.Level,
		})
	}

	// No-op fast path: requested already equals current.
	if payload.Level == currentLevel {
		return jsonResult(map[string]any{
			"initialized":   true,
			"persisted":     true,
			"current_level": currentLevel,
			"requested":     payload.Level,
			"persistence":   "no-op",
			"note":          "Requested level equals current default; config left unchanged.",
		})
	}

	if err := UpdateConfig(cfgPath, "strictness.default", payload.Level); err != nil {
		return jsonResult(map[string]any{
			"initialized":   true,
			"persisted":     false,
			"reason":        fmt.Sprintf("config update failed: %v", err),
			"current_level": currentLevel,
			"requested":     payload.Level,
		})
	}

	return jsonResult(map[string]any{
		"initialized":    true,
		"persisted":      true,
		"baseDir":        baseDir,
		"previous_level": currentLevel,
		"new_level":      payload.Level,
		"persistence":    "config.yaml",
		"note":           "Default strictness updated in .siren/config.yaml. Re-running `sightjack scan` will use the new threshold.",
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
