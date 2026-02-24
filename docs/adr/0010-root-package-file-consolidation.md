# 0010. Root Package File Consolidation

**Date:** 2026-02-24
**Status:** Accepted

## Context

The root `sightjack` package had grown to 30 source files (22,000+ lines).
Several micro-files existed with minimal content:

- `event_store.go` (9 lines): only the `EventStore` interface
- `tools.go` (66 lines): tool allow-list variables for Claude subprocess
- `parallel.go` (81 lines): generic `RunParallel` used only by scanner

Additionally, `session.go` (1,082 lines) had accumulated wave utility
functions, gate factory functions, and state constants that belonged
elsewhere. `cli.go` (347 lines) mixed input (prompt) and output (display)
concerns.

Moving files to `internal/` was considered for CLI-only leaf files
(archive.go, doctor.go, init.go) but rejected: each had significant
dependencies on root package types/functions, making interface extraction
cost exceed benefit. Circular import risk was also a concern for session.go
with 15+ root function calls.

## Decision

Perform Tidy First structural-only refactoring within the root package:

1. **Merge micro-files** into their natural homes:
   - `event_store.go` → `event.go` (EventStore interface alongside Event type)
   - `event_payloads.go` → `event.go` (payload structs alongside Event type)
   - `tools.go` → `claude.go` (tool lists alongside Claude subprocess logic)
   - `parallel.go` → `scanner.go` (RunParallel alongside its sole caller)

2. **Extract misplaced code** from `session.go`:
   - 12 wave utility functions → `wave.go`
   - `StateFormatVersion` constant → `state.go`
   - `buildNotifier` / `buildApprover` factories → `gate.go`

3. **Separate concerns** in `cli.go`:
   - 7 Display* output functions → `navigator.go` (rendering layer)
   - Input/prompt functions remain in `cli.go`

Zero behavioral changes. Every commit verified with `go build`, `go vet`,
and `go test`.

## Consequences

### Positive
- Source file count reduced from 30 to 26 (test files from 30 to 28)
- `session.go` reduced from 1,082 to ~850 lines
- `cli.go` reduced from 347 to ~250 lines (input-only)
- Smallest file increased from 9 lines to ~31 lines
- Each file has a clearer single responsibility

### Negative
- `event.go` and `wave.go` grew larger (but remain cohesive)
- `scanner.go` gained the `pond` import (acceptable: RunParallel is scanner-specific)

### Neutral
- No API changes; all exports remain at the same package path
- Test coverage unchanged (tests moved alongside their subjects)
