# 0011. ES Layer-First Refactoring

**Date:** 2026-02-25
**Status:** Accepted

## Context

The sightjack codebase had a flat root package with 26+ source files, mixing types, pure business logic, I/O orchestration, and CLI wiring. This violated Go's convention of small, focused packages and made the dependency graph opaque. The root package imported everything implicitly (same package), preventing incremental builds and clear layering.

ADR 0010 consolidated the root to types-only but didn't split implementation into layers.

## Decision

Restructure the codebase into four layers with strict dependency direction:

```
cmd → session → domain
         ↓
     eventsource → domain
```

- **Root (`sightjack`)**: Types, interfaces, constants, templates (go:embed). No business logic.
- **`internal/domain`**: Pure functions (no I/O, no context.Context, no side effects). Wave scheduling, scan utilities, event projection.
- **`internal/session`**: Orchestration with I/O, subprocess invocation, file writes, OTel tracing.
- **`internal/eventsource`**: Event store infrastructure. Imports root + domain only.
- **`internal/cmd`**: Cobra CLI commands (pre-existing).

Migration was bottom-up: interfaces first (ADR 0010), then session extraction, then domain extraction, then eventsource restructure. Each commit was strictly structural with zero behavioral changes, verified by build+vet+test.

## Consequences

### Positive

- Clear dependency DAG prevents circular imports
- Pure functions in domain are independently testable without mocks
- Session layer changes don't affect domain logic
- go:embed constraints satisfied (templates stay in root)

### Negative

- Cross-package function calls require explicit import and prefix (`domain.WaveKey()` vs bare `WaveKey()`)
- Test files for domain functions currently remain in session_test (functional but not ideal organization)
- config.go pure functions (ResolveStrictness, DefaultConfig) cannot move to domain due to circular import (domain → root → domain)

### Neutral

- Root package test files (`_test.go` with external test package) can still import internal/ packages without creating cycles
- `OverrideNewCmd` and `OverrideTracer` exposed as regular exported functions in session for cross-package test injection (export_test.go bridge pattern insufficient for cross-package access)
