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
	case "tools/list":
		return s.respond(msg.ID, map[string]any{"tools": toolDescriptors()})
	case "tools/call":
		return s.handleToolsCall(ctx, msg)
	default:
		return s.respondError(msg.ID, -32601, fmt.Sprintf("method not implemented: %s", msg.Method))
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

// toolDescriptors returns the Phase 2a MVP tool set. Each entry pins
// the interface (name, description, inputSchema) so claude code
// clients see a stable contract. As of Phase 3 all 3 non-ping tools
// are real impl (= read from .siren/.run/<session_id>/ scan + wave
// JSON files + config.yaml strictness default). Mutation persistence
// (= update_strictness writes to config) is Phase 4 follow-up.
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
			"description": "Read current default strictness from .siren/config.yaml + preview the requested level. Phase 3 is preview-only (= no persistence); Phase 4 follow-up wires the projection store mutation.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"level": map[string]any{"type": "string", "description": "strictness level: fog / cloud / clear / strict"},
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
// sightjack config (.siren/config.yaml) and returns a preview of the
// requested level alongside it. It does NOT persist the new level to
// config — full mutation requires the projection store wiring chain
// (Phase 4 follow-up). The session can rely on this preview to
// surface the existing default vs the operator's intended override.
//
// baseDir is the project root from MCPServer.WithBaseDir. When empty
// or the config is missing, the response signals uninitialized so the
// session surfaces a clear error to the operator.
//
// Pattern: paintress.update_gradient (= 83cb3ca) symmetric copy.
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
			"preview_level": payload.Level,
		})
	}
	cfg, err := LoadConfig(domain.ConfigPath(baseDir))
	if err != nil {
		return jsonResult(map[string]any{
			"initialized":   false,
			"reason":        fmt.Sprintf("config load failed: %v", err),
			"requested":     payload.Level,
			"current_level": "",
			"preview_level": payload.Level,
		})
	}
	return jsonResult(map[string]any{
		"initialized":   true,
		"baseDir":       baseDir,
		"current_level": string(cfg.Strictness.Default),
		"requested":     payload.Level,
		"preview_level": payload.Level,
		"persistence":   "preview-only",
		"note":          "Preview only. Persistence of the new default strictness requires the projection store wiring (Phase 4 follow-up). Override per-cluster via `sightjack config set` for now.",
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
