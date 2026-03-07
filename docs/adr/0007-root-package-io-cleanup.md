# 0007. Root Package I/O Cleanup

**Date:** 2026-02-25
**Status:** Accepted

## Context

ADR 0011 established the layer architecture (cmd -> session -> domain) and declared that the root package should contain "types, interfaces, constants, templates (go:embed). No business logic." However, after ADR 0011's ES layer extraction, several I/O functions remained in the root package:

- `telemetry.go`: OTel tracer init/span functions (called only from `internal/cmd/root.go`)
- `archive.go`: D-mail archive pruning (called only from `internal/cmd/archive_prune.go`)
- `state.go`: 6 I/O functions — EnsureMailDirs, EnsureScanDir, WriteGitIgnore, WriteScanResult, LoadScanResult, CanResume (called from ~35 sites across session/cmd)
- `config.go`: LoadConfig YAML reader (called from 3 sites)

These violated the root-is-types-only principle and created an implicit dependency from root to the filesystem that obscured the architecture.

## Decision

Move all I/O functions out of the root package in four incremental steps, each independently buildable and testable:

1. **telemetry.go -> internal/cmd/telemetry.go**: Single caller in same package. Functions made unexported.
2. **archive.go -> internal/session/archive.go**: D-mail lifecycle management belongs in session layer.
3. **state.go I/O -> internal/session/state_io.go**: 6 functions moved. Root state.go retains constants (StateDir, InboxDir, etc.) and pure path helpers (MailDir, ConfigPath, ScanDir).
4. **config.go LoadConfig -> internal/session/config.go**: YAML file reader. Root config.go retains all type definitions and pure functions (ResolveStrictness, DefaultConfig, ValidLang).

### Items explicitly NOT moved (with rationale)

| Item | Reason |
|------|--------|
| `init.go` (InstallSkills, RenderInitConfig) | go:embed requires relative path from same package; embed.FS is compile-time bound |
| `prompt.go` (Render* functions) | go:embed constraint; DoD functions are tightly coupled to DoDTemplate type |
| `logger.go` (Logger type + methods) | Infrastructure type used as parameter in 23+ files; migration cost far exceeds benefit |
| `config.go` pure functions | Circular import: domain -> root -> domain. ADR 0011 explicitly noted this constraint |

## Consequences

### Positive

- Root package now contains only types, interfaces, constants, go:embed templates, and pure functions
- I/O is consolidated in internal/session/ making the filesystem dependency boundary explicit
- telemetry functions are unexported in internal/cmd/ (smallest possible visibility)
- Each step was a [STRUCTURAL] commit with zero behavioral changes

### Negative

- 60+ call sites required mechanical updates (sightjack.X -> session.X or prefix removal)
- Test files split across root and internal/ (I/O tests moved with their functions, pure tests stay in root)

### Neutral

- Three go:embed-bound items remain as accepted ADR 0011 violations (InstallSkills, Render* prompts, Logger)
- root package still exports ~70 types and ~10 pure functions, which is appropriate for a Go library package
