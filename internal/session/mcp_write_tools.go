package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hironow/sightjack/internal/domain"
	"github.com/hironow/sightjack/internal/usecase/port"
)

// scanFileSlug converts a cluster name into a filesystem-safe filename
// fragment for the scan-dir read models (wave_<slug>.json /
// cluster_<slug>.json): lowercase, [a-z0-9] kept, runs of anything else
// collapse to a single underscore.
func scanFileSlug(name string) string {
	var b strings.Builder
	prevUnderscore := false
	for _, r := range strings.ToLower(name) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevUnderscore = false
		case !prevUnderscore:
			b.WriteByte('_')
			prevUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

// realRegisterWaves persists designed waves for one cluster (refs issue
// 0032 designer write path). Write order is pinned: the wave_*.json
// read model lands first (next_wave serves it immediately), then the
// EventWavesGenerated ledger entry. On event-append failure the file
// survives and the response degrades to persistence="files-only" with
// the reason — the session repairs by re-running the tool (idempotent
// overwrite, at-least-once ledger).
func realRegisterWaves(baseDir string, emitter port.ScanWriteEmitter, args json.RawMessage) map[string]any {
	var payload struct {
		SessionID   string        `json:"session_id"`
		ClusterName string        `json:"cluster_name"`
		Waves       []domain.Wave `json:"waves"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	if baseDir == "" {
		return jsonResult(map[string]any{
			"initialized": false,
			"registered":  false,
			"reason":      "sightjack mcp baseDir not configured (start `sightjack mcp` from the project root)",
		})
	}
	if payload.SessionID == "" || payload.ClusterName == "" || len(payload.Waves) == 0 {
		return jsonResult(map[string]any{
			"initialized": true,
			"registered":  false,
			"reason":      "session_id, cluster_name and a non-empty waves array are required",
		})
	}
	scanDir := domain.ScanDir(baseDir, payload.SessionID)
	if err := os.MkdirAll(scanDir, 0o755); err != nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"registered":  false,
			"reason":      fmt.Sprintf("scan dir create failed: %v", err),
		})
	}
	for i := range payload.Waves {
		if payload.Waves[i].ClusterName == "" {
			payload.Waves[i].ClusterName = payload.ClusterName
		}
	}
	data, err := json.MarshalIndent(domain.WaveGenerateResult{
		ClusterName: payload.ClusterName,
		Waves:       payload.Waves,
	}, "", "  ")
	if err != nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"registered":  false,
			"reason":      fmt.Sprintf("wave marshal failed: %v", err),
		})
	}
	waveFile := filepath.Join(scanDir, "wave_"+scanFileSlug(payload.ClusterName)+".json")
	if err := atomicWrite(waveFile, data); err != nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"registered":  false,
			"reason":      fmt.Sprintf("wave file write failed: %v", err),
		})
	}

	persistence := "files+event-store"
	reason := ""
	if emitter == nil {
		persistence = "files-only"
		reason = "event emitter not wired (event ledger skipped; restart `sightjack mcp` from an initialized project root)"
	} else if err := emitter.EmitRecordWavesGenerated(domain.WavesGeneratedPayload{
		Waves: domain.WavesToStates(payload.Waves),
	}, time.Now().UTC()); err != nil {
		persistence = "files-only"
		reason = fmt.Sprintf("event append failed (re-run register_waves to repair): %v", err)
	}

	res := map[string]any{
		"initialized": true,
		"registered":  true,
		"session_id":  payload.SessionID,
		"scan_dir":    scanDir,
		"wave_file":   waveFile,
		"wave_count":  len(payload.Waves),
		"persistence": persistence,
	}
	if reason != "" {
		res["reason"] = reason
	}
	return jsonResult(res)
}

// realSaveScanResult persists the session's scan result (refs issue
// 0032 designer write path). Same write-order contract as
// realRegisterWaves: cluster_*.json read models first, then the
// EventScanCompleted ledger entry, degrading to "files-only" + reason
// on append failure (re-run to repair).
func realSaveScanResult(baseDir string, emitter port.ScanWriteEmitter, args json.RawMessage) map[string]any {
	var payload struct {
		SessionID    string                     `json:"session_id"`
		ShibitoCount int                        `json:"shibito_count"`
		Clusters     []domain.ClusterScanResult `json:"clusters"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	if baseDir == "" {
		return jsonResult(map[string]any{
			"initialized": false,
			"saved":       false,
			"reason":      "sightjack mcp baseDir not configured (start `sightjack mcp` from the project root)",
		})
	}
	if payload.SessionID == "" || len(payload.Clusters) == 0 {
		return jsonResult(map[string]any{
			"initialized": true,
			"saved":       false,
			"reason":      "session_id and a non-empty clusters array are required",
		})
	}
	scanDir := domain.ScanDir(baseDir, payload.SessionID)
	if err := os.MkdirAll(scanDir, 0o755); err != nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"saved":       false,
			"reason":      fmt.Sprintf("scan dir create failed: %v", err),
		})
	}

	clusterFiles := make([]string, 0, len(payload.Clusters))
	totalCompleteness := 0.0
	for i := range payload.Clusters {
		c := payload.Clusters[i]
		slugSource := c.Key
		if slugSource == "" {
			slugSource = c.Name
		}
		data, err := json.MarshalIndent(c, "", "  ")
		if err != nil {
			return jsonResult(map[string]any{
				"initialized": true,
				"saved":       false,
				"reason":      fmt.Sprintf("cluster marshal failed (%s): %v", c.Name, err),
			})
		}
		clusterFile := filepath.Join(scanDir, "cluster_"+scanFileSlug(slugSource)+".json")
		if err := atomicWrite(clusterFile, data); err != nil {
			return jsonResult(map[string]any{
				"initialized": true,
				"saved":       false,
				"reason":      fmt.Sprintf("cluster file write failed (%s): %v", c.Name, err),
			})
		}
		clusterFiles = append(clusterFiles, clusterFile)
		totalCompleteness += c.Completeness
	}
	avgCompleteness := totalCompleteness / float64(len(payload.Clusters))

	persistence := "files+event-store"
	reason := ""
	if emitter == nil {
		persistence = "files-only"
		reason = "event emitter not wired (event ledger skipped; restart `sightjack mcp` from an initialized project root)"
	} else if err := emitter.EmitRecordScan(domain.ScanCompletedPayload{
		Clusters:       domain.ClustersToStates(payload.Clusters),
		Completeness:   avgCompleteness,
		ShibitoCount:   payload.ShibitoCount,
		ScanResultPath: domain.RelativeScanResultPath(baseDir, scanDir),
		LastScanned:    time.Now().UTC(),
	}, time.Now().UTC()); err != nil {
		persistence = "files-only"
		reason = fmt.Sprintf("event append failed (re-run save_scan_result to repair): %v", err)
	}

	res := map[string]any{
		"initialized":   true,
		"saved":         true,
		"session_id":    payload.SessionID,
		"scan_dir":      scanDir,
		"cluster_files": clusterFiles,
		"cluster_count": len(payload.Clusters),
		"persistence":   persistence,
	}
	if reason != "" {
		res["reason"] = reason
	}
	return jsonResult(res)
}

// realDMail emits a D-Mail through the transactional outbox (refs
// issue 0031: D-Mail emission restored as an MCP tool). The arguments
// are a typed subset of the D-Mail v1 schema mapped 1:1 onto
// domain.DMail; direct outbox/ writes from the session remain
// forbidden because they would bypass the SQLite stage -> atomic flush
// contract that phonewave's watcher depends on.
//
// Kind validation is two-layered: domain.ValidateDMail enforces the
// full-protocol schema, and domain.ProducesKinds restricts emission to
// the kinds sightjack's dmail-sendable manifest declares
// (specification / report / stall-escalation) so routing always exists.
func realDMail(ctx context.Context, baseDir string, args json.RawMessage) map[string]any {
	var payload struct {
		Kind        string                `json:"kind"`
		Name        string                `json:"name"`
		Description string                `json:"description"`
		Body        string                `json:"body"`
		Issues      []string              `json:"issues"`
		Severity    string                `json:"severity"`
		Action      string                `json:"action"`
		Priority    int                   `json:"priority"`
		Wave        *domain.WaveReference `json:"wave"`
		Metadata    map[string]string     `json:"metadata"`
	}
	if len(args) > 0 {
		_ = json.Unmarshal(args, &payload)
	}
	if baseDir == "" {
		return jsonResult(map[string]any{
			"initialized": false,
			"sent":        false,
			"reason":      "sightjack mcp baseDir not configured (start `sightjack mcp` from the project root)",
		})
	}
	kind := domain.DMailKind(payload.Kind)
	if !domain.ProducesKinds[kind] {
		return jsonResult(map[string]any{
			"initialized": true,
			"sent":        false,
			"reason":      fmt.Sprintf("sightjack does not produce kind %q (produces: specification / report / stall-escalation per the dmail-sendable manifest)", payload.Kind),
		})
	}
	mail := &domain.DMail{
		SchemaVersion: domain.DMailSchemaVersion,
		Name:          payload.Name,
		Kind:          kind,
		Description:   payload.Description,
		Body:          payload.Body,
		Issues:        payload.Issues,
		Severity:      payload.Severity,
		Action:        payload.Action,
		Priority:      payload.Priority,
		Wave:          payload.Wave,
		Metadata:      payload.Metadata,
	}
	store, err := NewOutboxStoreForDir(baseDir)
	if err != nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"sent":        false,
			"reason":      fmt.Sprintf("outbox store open failed: %v", err),
		})
	}
	defer func() { _ = store.Close() }()
	if err := ComposeDMail(ctx, store, mail); err != nil {
		return jsonResult(map[string]any{
			"initialized": true,
			"sent":        false,
			"reason":      fmt.Sprintf("dmail compose failed: %v", err),
		})
	}
	return jsonResult(map[string]any{
		"initialized": true,
		"sent":        true,
		"name":        mail.Name,
		"filename":    mail.Filename(),
		"kind":        string(mail.Kind),
		"persistence": "transactional-outbox",
	})
}
