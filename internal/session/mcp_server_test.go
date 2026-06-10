package session_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/session"
)

func TestMCPServer_ListsAllPhase2aTools(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: all 4 Phase 2a tools advertised, with stable names
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("jsonrpc = %v, want 2.0", resp["jsonrpc"])
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %v", resp)
	}
	tools, ok := result["tools"].([]any)
	if !ok {
		t.Fatalf("tools list missing: %v", result["tools"])
	}
	want := map[string]bool{
		"ping":              false,
		"next_wave":         false,
		"get_scan_result":   false,
		"update_strictness": false,
	}
	for _, t0 := range tools {
		entry, _ := t0.(map[string]any)
		if name, _ := entry["name"].(string); name != "" {
			want[name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("missing Phase 2a tool: %s", name)
		}
	}
}

func TestMCPServer_CallsPingTool(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"ping","arguments":{}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %v", resp)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) != 1 {
		t.Fatalf("content list mismatch: %v", result["content"])
	}
	first, _ := content[0].(map[string]any)
	if first["text"] != "pong" {
		t.Errorf("text = %v, want pong", first["text"])
	}
}

func TestMCPServer_RejectsUnknownTool(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"does_not_exist","arguments":{}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	rpcErr, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if code, _ := rpcErr["code"].(float64); int(code) != -32601 {
		t.Errorf("error code = %v, want -32601", rpcErr["code"])
	}
}

func TestMCPServer_NextWave_UninitializedBaseDir(t *testing.T) {
	// given: NewMCPServer without WithBaseDir → uninitialized response.
	in := strings.NewReader(`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"next_wave","arguments":{"session_id":"any"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeFirstText(t, &out)
	if body["initialized"] != false {
		t.Errorf("initialized = %v, want false (empty baseDir)", body["initialized"])
	}
}

func TestMCPServer_NextWave_MissingSessionID(t *testing.T) {
	// given: baseDir set but session_id missing → initialized but error reason.
	baseDir := t.TempDir()
	in := strings.NewReader(`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"next_wave","arguments":{}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil).WithBaseDir(baseDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeFirstText(t, &out)
	if body["initialized"] != true {
		t.Errorf("initialized = %v, want true", body["initialized"])
	}
	if _, ok := body["reason"]; !ok {
		t.Errorf("reason missing for missing session_id: %v", body)
	}
}

func TestMCPServer_NextWave_RealImpl_WithWaveFiles(t *testing.T) {
	// given: baseDir with scan dir + wave_*.json containing 1 available wave.
	baseDir := t.TempDir()
	sessionID := "test-session"
	scanDir := domain.ScanDir(baseDir, sessionID)
	if err := os.MkdirAll(scanDir, 0o755); err != nil {
		t.Fatalf("mkdir scan: %v", err)
	}
	waveJSON := `{
  "cluster_name": "auth",
  "waves": [
    {"id": "w1", "cluster_name": "auth", "title": "Add session expiry", "actions": [], "prerequisites": [], "delta": {"before": 0.25, "after": 0.40}, "status": "available"}
  ]
}`
	if err := os.WriteFile(filepath.Join(scanDir, "wave_00_auth.json"), []byte(waveJSON), 0o644); err != nil {
		t.Fatalf("write wave: %v", err)
	}

	in := strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"next_wave","arguments":{"session_id":"test-session"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil).WithBaseDir(baseDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeFirstText(t, &out)
	if body["initialized"] != true {
		t.Errorf("initialized = %v, want true", body["initialized"])
	}
	if got, _ := body["total_waves"].(float64); int(got) != 1 {
		t.Errorf("total_waves = %v, want 1", body["total_waves"])
	}
	if got, _ := body["available_waves"].(float64); int(got) != 1 {
		t.Errorf("available_waves = %v, want 1", body["available_waves"])
	}
	nextWave, _ := body["next_wave"].(map[string]any)
	if nextWave == nil || nextWave["id"] != "w1" {
		t.Errorf("next_wave.id = %v, want w1", nextWave)
	}
}

func TestMCPServer_GetScanResult_UninitializedBaseDir(t *testing.T) {
	// given: NewMCPServer without WithBaseDir.
	in := strings.NewReader(`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"get_scan_result","arguments":{"session_id":"any"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeFirstText(t, &out)
	if body["initialized"] != false {
		t.Errorf("initialized = %v, want false", body["initialized"])
	}
}

func TestMCPServer_GetScanResult_RealImpl_WithClusterFiles(t *testing.T) {
	// given: baseDir with scan dir + 2 cluster_*.json files.
	baseDir := t.TempDir()
	sessionID := "test-session"
	scanDir := domain.ScanDir(baseDir, sessionID)
	if err := os.MkdirAll(scanDir, 0o755); err != nil {
		t.Fatalf("mkdir scan: %v", err)
	}
	authJSON := `{"name": "auth", "completeness": 0.4, "issues": [], "observations": []}`
	apiJSON := `{"name": "api", "completeness": 0.6, "issues": [], "observations": []}`
	if err := os.WriteFile(filepath.Join(scanDir, "cluster_00_auth.json"), []byte(authJSON), 0o644); err != nil {
		t.Fatalf("write auth: %v", err)
	}
	if err := os.WriteFile(filepath.Join(scanDir, "cluster_01_api.json"), []byte(apiJSON), 0o644); err != nil {
		t.Fatalf("write api: %v", err)
	}

	in := strings.NewReader(`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"get_scan_result","arguments":{"session_id":"test-session"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil).WithBaseDir(baseDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeFirstText(t, &out)
	if body["initialized"] != true {
		t.Errorf("initialized = %v, want true", body["initialized"])
	}
	if got, _ := body["cluster_count"].(float64); int(got) != 2 {
		t.Errorf("cluster_count = %v, want 2", body["cluster_count"])
	}
	// avg completeness = (0.4 + 0.6) / 2 = 0.5
	if got, _ := body["average_completeness"].(float64); got != 0.5 {
		t.Errorf("average_completeness = %v, want 0.5", body["average_completeness"])
	}
}

func TestMCPServer_UpdateStrictness_UninitializedBaseDir(t *testing.T) {
	// given: NewMCPServer without WithBaseDir → uninitialized response.
	in := strings.NewReader(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"update_strictness","arguments":{"level":"alert"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeFirstText(t, &out)
	if body["initialized"] != false {
		t.Errorf("initialized = %v, want false (empty baseDir)", body["initialized"])
	}
	if body["requested"] != "alert" {
		t.Errorf("requested = %v, want alert", body["requested"])
	}
}

func TestMCPServer_UpdateStrictness_Phase4_PersistsToConfig(t *testing.T) {
	// given: temp baseDir with a sightjack config (= default strictness 'fog').
	baseDir := t.TempDir()
	cfgPath := domain.ConfigPath(baseDir)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	cfg := `tracker:
  source: linear
scan:
  team: TEAM-A
strictness:
  default: fog
retry:
  max_attempts: 3
labels: {}
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	in := strings.NewReader(`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"update_strictness","arguments":{"level":"alert"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil).WithBaseDir(baseDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: persisted=true + previous_level=fog + new_level=alert + config
	// file on disk now contains the new level.
	body := decodeFirstText(t, &out)
	if body["initialized"] != true {
		t.Errorf("initialized = %v, want true (body=%v)", body["initialized"], body)
	}
	if body["persisted"] != true {
		t.Errorf("persisted = %v, want true (body=%v)", body["persisted"], body)
	}
	if body["previous_level"] != "fog" {
		t.Errorf("previous_level = %v, want fog", body["previous_level"])
	}
	if body["new_level"] != "alert" {
		t.Errorf("new_level = %v, want alert", body["new_level"])
	}
	if body["persistence"] != "config.yaml" {
		t.Errorf("persistence = %v, want config.yaml", body["persistence"])
	}

	// Verify config.yaml on disk now reflects the new level.
	reloaded, err := session.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if string(reloaded.Strictness.Default) != "alert" {
		t.Errorf("on-disk default = %v, want alert", reloaded.Strictness.Default)
	}
}

func TestMCPServer_UpdateStrictness_Phase4_RejectsInvalidLevel(t *testing.T) {
	// given: temp baseDir + invalid level 'strict' (= not in fog/alert/lockdown).
	baseDir := t.TempDir()
	cfgPath := domain.ConfigPath(baseDir)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	cfg := `strictness:
  default: fog
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	in := strings.NewReader(`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"update_strictness","arguments":{"level":"strict"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil).WithBaseDir(baseDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: persisted=false + reason mentions valid levels.
	body := decodeFirstText(t, &out)
	if body["persisted"] != false {
		t.Errorf("persisted = %v, want false (invalid level)", body["persisted"])
	}
	reason, _ := body["reason"].(string)
	if !strings.Contains(reason, "fog") || !strings.Contains(reason, "alert") || !strings.Contains(reason, "lockdown") {
		t.Errorf("reason should list valid levels, got %q", reason)
	}

	// Verify config.yaml on disk was NOT touched.
	reloaded, err := session.LoadConfig(cfgPath)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if string(reloaded.Strictness.Default) != "fog" {
		t.Errorf("on-disk default = %v, want fog (rejected write should not modify)", reloaded.Strictness.Default)
	}
}

func TestMCPServer_UpdateStrictness_Phase4_NoOpWhenAlreadyAtLevel(t *testing.T) {
	// given: temp baseDir + level equals current default.
	baseDir := t.TempDir()
	cfgPath := domain.ConfigPath(baseDir)
	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		t.Fatalf("mkdir state: %v", err)
	}
	cfg := `strictness:
  default: alert
`
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	in := strings.NewReader(`{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"update_strictness","arguments":{"level":"alert"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil).WithBaseDir(baseDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: persisted=true with persistence='no-op'
	body := decodeFirstText(t, &out)
	if body["persisted"] != true {
		t.Errorf("persisted = %v, want true (no-op is still 'persisted')", body["persisted"])
	}
	if body["persistence"] != "no-op" {
		t.Errorf("persistence = %v, want no-op", body["persistence"])
	}
}

// decodeFirstText extracts the JSON payload from the first content
// item of the MCP tools/call response. Stub responses ship a single
// JSON-string text entry so the body is a JSON object inside a string.
func decodeFirstText(t *testing.T, out *bytes.Buffer) map[string]any {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %v", resp)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("missing content: %v", result)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	var body map[string]any
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("decode inner JSON: %v (raw=%q)", err, text)
	}
	return body
}

func TestMCPServer_RejectsUnknownMethod(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":4,"method":"completion/complete"}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, out.String())
	}
	rpcErr, ok := resp["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error, got %v", resp)
	}
	if code, _ := rpcErr["code"].(float64); int(code) != -32601 {
		t.Errorf("error code = %v, want -32601", rpcErr["code"])
	}
}

func TestMCPServer_Initialize_Handshake(t *testing.T) {
	// given: client sends initialize with a different protocol version
	in := strings.NewReader(`{"jsonrpc":"2.0","id":0,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"claude-code","version":"1.0"}}}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: server returns ITS supported version (not an echo), + tools cap + serverInfo
	var resp struct {
		Result struct {
			ProtocolVersion string                     `json:"protocolVersion"`
			Capabilities    map[string]json.RawMessage `json:"capabilities"`
			ServerInfo      struct {
				Name string `json:"name"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode initialize response: %v (raw=%q)", err, out.String())
	}
	if resp.Result.ProtocolVersion != "2024-11-05" {
		t.Errorf("protocolVersion = %q, want 2024-11-05 (server supported, not echo of client 2025-06-18)", resp.Result.ProtocolVersion)
	}
	if _, ok := resp.Result.Capabilities["tools"]; !ok {
		t.Errorf("capabilities.tools missing: %v", resp.Result.Capabilities)
	}
	if resp.Result.ServerInfo.Name != "sightjack" {
		t.Errorf("serverInfo.name = %q, want sightjack", resp.Result.ServerInfo.Name)
	}
}

func TestMCPServer_NotificationsInitialized_NoResponse(t *testing.T) {
	// given: a JSON-RPC notification (no id)
	in := strings.NewReader(`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: notifications must not produce a response
	if strings.TrimSpace(out.String()) != "" {
		t.Errorf("notification must produce no response, got: %q", out.String())
	}
}
