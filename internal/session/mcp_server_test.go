package session_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

// --- write tools (refs issue 0032: producer write-path restoration) ---

// recordingScanEmitter is a session_test-local fake that structurally
// satisfies port.ScanWriteEmitter (EmitRecordScan +
// EmitRecordWavesGenerated) without importing usecase/port.
type recordingScanEmitter struct {
	scans    []domain.ScanCompletedPayload
	waves    []domain.WavesGeneratedPayload
	failWith error
}

func (r *recordingScanEmitter) EmitRecordScan(p domain.ScanCompletedPayload, _ time.Time) error {
	if r.failWith != nil {
		return r.failWith
	}
	r.scans = append(r.scans, p)
	return nil
}

func (r *recordingScanEmitter) EmitRecordWavesGenerated(p domain.WavesGeneratedPayload, _ time.Time) error {
	if r.failWith != nil {
		return r.failWith
	}
	r.waves = append(r.waves, p)
	return nil
}

func decodeToolJSON(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(raw), &resp); err != nil {
		t.Fatalf("decode response: %v (raw=%q)", err, string(raw))
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result: %v", resp)
	}
	content, ok := result["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("content missing: %v", result)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	var body map[string]any
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		t.Fatalf("decode tool body: %v (text=%q)", err, text)
	}
	return body
}

func TestMCPServer_ToolsList_IncludesWriteTools(t *testing.T) {
	// given
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n")
	var out bytes.Buffer
	srv := session.NewMCPServer(in, &out, nil)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	var resp map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	result := resp["result"].(map[string]any)
	tools := result["tools"].([]any)
	want := map[string]bool{"save_scan_result": false, "register_waves": false}
	for _, t0 := range tools {
		entry, _ := t0.(map[string]any)
		if name, _ := entry["name"].(string); name != "" {
			if _, tracked := want[name]; tracked {
				want[name] = true
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("missing write tool in tools/list: %s", name)
		}
	}
}

func TestMCPServer_RegisterWaves_WritesWaveFileAndEmitsEvent(t *testing.T) {
	// given
	baseDir := t.TempDir()
	emitter := &recordingScanEmitter{}
	req := `{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"register_waves","arguments":{"session_id":"s1","cluster_name":"Auth Cluster","waves":[{"id":"w1","cluster_name":"Auth Cluster","title":"Fix token refresh","status":"available","actions":[{"type":"edit","description":"x"}]}]}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).WithBaseDir(baseDir).WithEmitter(emitter)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: response contract
	body := decodeToolJSON(t, out.Bytes())
	if body["registered"] != true {
		t.Fatalf("registered = %v, want true (body=%v)", body["registered"], body)
	}
	if body["persistence"] != "files+event-store" {
		t.Errorf("persistence = %v, want files+event-store", body["persistence"])
	}

	// then: wave_*.json written into the session scan dir, parseable
	scanDir := domain.ScanDir(baseDir, "s1")
	waveFile := filepath.Join(scanDir, "wave_auth_cluster.json")
	result, err := session.ParseWaveGenerateResult(waveFile)
	if err != nil {
		t.Fatalf("ParseWaveGenerateResult(%s): %v", waveFile, err)
	}
	if result.ClusterName != "Auth Cluster" || len(result.Waves) != 1 || result.Waves[0].ID != "w1" {
		t.Errorf("wave file content mismatch: %+v", result)
	}

	// then: event payload emitted with mapped WaveState
	if len(emitter.waves) != 1 || len(emitter.waves[0].Waves) != 1 {
		t.Fatalf("emitted waves = %+v, want 1 payload with 1 wave", emitter.waves)
	}
	ws := emitter.waves[0].Waves[0]
	if ws.ID != "w1" || ws.ClusterName != "Auth Cluster" || ws.Status != "available" || ws.ActionCount != 1 {
		t.Errorf("WaveState mismatch: %+v", ws)
	}

	// then: next_wave roundtrip serves the registered wave
	nwReq := `{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"next_wave","arguments":{"session_id":"s1"}}}` + "\n"
	var out2 bytes.Buffer
	srv2 := session.NewMCPServer(strings.NewReader(nwReq), &out2, nil).WithBaseDir(baseDir)
	if err := srv2.Serve(context.Background()); err != nil {
		t.Fatalf("Serve next_wave: %v", err)
	}
	nw := decodeToolJSON(t, out2.Bytes())
	wave, _ := nw["next_wave"].(map[string]any)
	if wave == nil || wave["id"] != "w1" {
		t.Errorf("next_wave roundtrip = %v, want next_wave id w1", nw)
	}
}

func TestMCPServer_RegisterWaves_FilesOnlyWhenEventAppendFails(t *testing.T) {
	// given
	baseDir := t.TempDir()
	emitter := &recordingScanEmitter{failWith: errors.New("event store offline")}
	req := `{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"register_waves","arguments":{"session_id":"s1","cluster_name":"auth","waves":[{"id":"w1","title":"t","status":"available"}]}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).WithBaseDir(baseDir).WithEmitter(emitter)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: files written, persistence degraded, reason surfaced
	body := decodeToolJSON(t, out.Bytes())
	if body["persistence"] != "files-only" {
		t.Errorf("persistence = %v, want files-only", body["persistence"])
	}
	if reason, _ := body["reason"].(string); !strings.Contains(reason, "event store offline") {
		t.Errorf("reason = %v, want event append error surfaced", body["reason"])
	}
	if _, err := os.Stat(filepath.Join(domain.ScanDir(baseDir, "s1"), "wave_auth.json")); err != nil {
		t.Errorf("wave file should exist even when event append fails: %v", err)
	}
}

func TestMCPServer_RegisterWaves_NilEmitterIsFilesOnly(t *testing.T) {
	// given
	baseDir := t.TempDir()
	req := `{"jsonrpc":"2.0","id":10,"method":"tools/call","params":{"name":"register_waves","arguments":{"session_id":"s1","cluster_name":"auth","waves":[{"id":"w1","title":"t","status":"available"}]}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).WithBaseDir(baseDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeToolJSON(t, out.Bytes())
	if body["persistence"] != "files-only" {
		t.Errorf("persistence = %v, want files-only when emitter is not wired", body["persistence"])
	}
}

func TestMCPServer_RegisterWaves_OverwriteIsIdempotent(t *testing.T) {
	// given: two register calls for the same cluster
	baseDir := t.TempDir()
	emitter := &recordingScanEmitter{}
	req := `{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"register_waves","arguments":{"session_id":"s1","cluster_name":"auth","waves":[{"id":"w1","title":"t","status":"available"}]}}}` + "\n" +
		`{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"register_waves","arguments":{"session_id":"s1","cluster_name":"auth","waves":[{"id":"w1","title":"t","status":"available"},{"id":"w2","title":"t2","status":"available"}]}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).WithBaseDir(baseDir).WithEmitter(emitter)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: exactly one wave file (overwritten), latest content wins
	entries, err := os.ReadDir(domain.ScanDir(baseDir, "s1"))
	if err != nil {
		t.Fatalf("read scan dir: %v", err)
	}
	waveFiles := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "wave_") {
			waveFiles++
		}
	}
	if waveFiles != 1 {
		t.Errorf("wave files = %d, want 1 (idempotent overwrite)", waveFiles)
	}
	result, err := session.ParseWaveGenerateResult(filepath.Join(domain.ScanDir(baseDir, "s1"), "wave_auth.json"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(result.Waves) != 2 {
		t.Errorf("latest content should win: waves = %d, want 2", len(result.Waves))
	}
}

func TestMCPServer_RegisterWaves_ValidatesArgs(t *testing.T) {
	// given: missing session_id
	baseDir := t.TempDir()
	req := `{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"register_waves","arguments":{"cluster_name":"auth","waves":[{"id":"w1"}]}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).WithBaseDir(baseDir)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeToolJSON(t, out.Bytes())
	if body["registered"] != false {
		t.Errorf("registered = %v, want false without session_id", body["registered"])
	}
}

func TestMCPServer_SaveScanResult_WritesClusterFilesAndEmitsEvent(t *testing.T) {
	// given
	baseDir := t.TempDir()
	emitter := &recordingScanEmitter{}
	req := `{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"save_scan_result","arguments":{"session_id":"s1","shibito_count":1,"clusters":[{"name":"Auth","key":"auth","completeness":0.4,"issues":[{"id":"I-1"}]},{"name":"API","key":"api","completeness":0.8,"issues":[]}]}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).WithBaseDir(baseDir).WithEmitter(emitter)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then: response contract
	body := decodeToolJSON(t, out.Bytes())
	if body["saved"] != true || body["persistence"] != "files+event-store" {
		t.Fatalf("save response mismatch: %v", body)
	}
	if int(body["cluster_count"].(float64)) != 2 {
		t.Errorf("cluster_count = %v, want 2", body["cluster_count"])
	}

	// then: cluster files exist and parse
	for _, name := range []string{"cluster_auth.json", "cluster_api.json"} {
		if _, err := session.ParseClusterScanResult(filepath.Join(domain.ScanDir(baseDir, "s1"), name)); err != nil {
			t.Errorf("cluster file %s unparseable: %v", name, err)
		}
	}

	// then: event payload mapped (ClusterState carries issue counts)
	if len(emitter.scans) != 1 {
		t.Fatalf("scans emitted = %d, want 1", len(emitter.scans))
	}
	sp := emitter.scans[0]
	if len(sp.Clusters) != 2 || sp.ShibitoCount != 1 {
		t.Errorf("ScanCompletedPayload mismatch: %+v", sp)
	}
	var authState *domain.ClusterState
	for i := range sp.Clusters {
		if sp.Clusters[i].Name == "Auth" {
			authState = &sp.Clusters[i]
		}
	}
	if authState == nil || authState.IssueCount != 1 {
		t.Errorf("Auth ClusterState mismatch: %+v", sp.Clusters)
	}

	// then: get_scan_result roundtrip sees both clusters
	gsReq := `{"jsonrpc":"2.0","id":15,"method":"tools/call","params":{"name":"get_scan_result","arguments":{"session_id":"s1"}}}` + "\n"
	var out2 bytes.Buffer
	srv2 := session.NewMCPServer(strings.NewReader(gsReq), &out2, nil).WithBaseDir(baseDir)
	if err := srv2.Serve(context.Background()); err != nil {
		t.Fatalf("Serve get_scan_result: %v", err)
	}
	gs := decodeToolJSON(t, out2.Bytes())
	if int(gs["cluster_count"].(float64)) != 2 {
		t.Errorf("get_scan_result roundtrip cluster_count = %v, want 2", gs["cluster_count"])
	}
}

func TestMCPServer_SaveScanResult_FilesOnlyWhenEventAppendFails(t *testing.T) {
	// given
	baseDir := t.TempDir()
	emitter := &recordingScanEmitter{failWith: errors.New("append refused")}
	req := `{"jsonrpc":"2.0","id":16,"method":"tools/call","params":{"name":"save_scan_result","arguments":{"session_id":"s1","clusters":[{"name":"Auth","key":"auth","completeness":0.4,"issues":[]}]}}}` + "\n"
	var out bytes.Buffer
	srv := session.NewMCPServer(strings.NewReader(req), &out, nil).WithBaseDir(baseDir).WithEmitter(emitter)

	// when
	if err := srv.Serve(context.Background()); err != nil {
		t.Fatalf("Serve: %v", err)
	}

	// then
	body := decodeToolJSON(t, out.Bytes())
	if body["persistence"] != "files-only" {
		t.Errorf("persistence = %v, want files-only", body["persistence"])
	}
	if _, err := os.Stat(filepath.Join(domain.ScanDir(baseDir, "s1"), "cluster_auth.json")); err != nil {
		t.Errorf("cluster file should exist even when event append fails: %v", err)
	}
}
