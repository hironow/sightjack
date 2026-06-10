package port

import (
	"time"

	"github.com/hironow/sightjack/internal/domain"
)

// ScanWriteEmitter is the narrow write surface the MCP data plane needs
// to persist designer output (refs issue 0032: producer write-path
// restoration): scan results and generated waves. The full
// SessionEventEmitter satisfies it structurally; the MCP server depends
// only on these two methods so tests can fake the seam without the
// 13-method emitter (pattern: dominator ADR 0005 JudgmentEventEmitter).
type ScanWriteEmitter interface {
	EmitRecordScan(payload domain.ScanCompletedPayload, now time.Time) error
	EmitRecordWavesGenerated(payload domain.WavesGeneratedPayload, now time.Time) error
}
