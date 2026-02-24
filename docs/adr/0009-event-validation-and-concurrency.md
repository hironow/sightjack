# 0009. Event Validation and Concurrency Control

**Date:** 2026-02-24
**Status:** Accepted

## Context

ADR 0008 introduced event sourcing with JSONL-based persistence. Phase 1 added
idempotent replay, error handling, per-event fsync, and CorrelationID/CausationID.

AWS Prescriptive Guidance for event sourcing recommends several additional
patterns: event validation before ingestion, optimistic concurrency control,
snapshots, schema evolution, CQRS, and event lifecycle management.

Sightjack is a single-process CLI tool with 30-100 events per session and
file-based (JSONL) persistence. No database or AWS services are used.

## Decision

### Apply: Event Validation at Append Level

`ValidateEvent()` checks structural validity (non-empty type, session_id,
schema_version, payload; sequence >= 1; non-zero timestamp) before any event
is persisted. Validation runs in `FileEventStore.Append()`, not in `NewEvent()`,
because deserialized events (e.g. from corrupt files) bypass `NewEvent()`.
The entire batch is rejected if any single event fails validation.

### Apply: In-Memory Sequence Monotonicity Check

`FileEventStore` tracks `lastWrittenSeq` in memory with lazy initialization
from the existing file on first append. Each `Append()` call verifies that
event sequences are strictly monotonically increasing (no gaps, no duplicates).
This is implemented as a concrete method on `FileEventStore`, not as a separate
interface, because the single-process CLI has no true concurrent writers.
The check serves as a bug detector for incorrect sequence management.

### Apply: Event Lifecycle Management

`ListExpiredEventFiles()` and `PruneEventFiles()` follow the same pattern as
the existing `ListExpiredArchive()` / `DeleteArchiveFiles()` for d-mail files.
The `archive-prune` CLI command prunes both d-mail and event files with the
same retention threshold.

### Skip: Snapshots

With 30-100 events per session, full replay completes in < 1ms. Snapshot
complexity (serialization, invalidation, consistency) is not justified.

### Skip: Schema Evolution / Upcasting

Backward compatibility is not required. Old JSONL files can be deleted.
`EventSchemaVersion` is stamped but no upcasting logic is needed.

### Skip: CQRS (Read/Write Separation)

Single process, single projection (`ProjectState`). Read/write separation
adds complexity with no benefit.

## Consequences

### Positive
- Invalid events cannot be persisted (data integrity)
- Sequence bugs are detected immediately at write time
- Old event files are cleaned up alongside d-mail archives
- No new interfaces or abstractions added (minimal complexity)

### Negative
- Concurrent random-order writes to `FileEventStore` are no longer possible (by design)
- Lazy init reads the entire file on first `Append()` (negligible for < 100 events)

### Neutral
- `SessionRecorder` remains the primary write path and manages sequences automatically
- The monotonicity check is a safety net, not a concurrency control mechanism
