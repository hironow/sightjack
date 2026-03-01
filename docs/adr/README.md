# Architecture Decision Records

## Shared ADRs (canonical: phonewave)

0001-0005 are reserved. Canonical versions live in `phonewave/docs/adr/`.

| # | Decision | Linear |
|---|----------|--------|
| 0001 | cobra CLI framework adoption | MY-329 |
| 0002 | stdio convention (stdout=data, stderr=logs) | MY-339 |
| 0003 | OpenTelemetry noop-default + OTLP HTTP | — |
| 0004 | D-Mail Schema v1 specification | MY-352, MY-353 |
| 0005 | fsnotify daemon design | — |

## Extended Shared ADRs (S-series, canonical: phonewave)

Canonical versions live in phonewave `docs/adr/`. Referenced here for discoverability.

| # | Decision | Status |
|---|----------|--------|
| S0001 | ~~Logger as root package exception~~ | Superseded by S0005 |
| S0002 | JSONL append-only event sourcing pattern | Accepted |
| S0003 | Three-way approval contract | Accepted |
| S0004 | ~~Layer architecture conventions~~ | Superseded by S0005 |
| S0005 | Root infrastructure pattern and layer conventions | Accepted |
| S0011 | SQLite WAL cooperative model for concurrent CLI | Accepted |
| S0012 | Reference data management pattern | Accepted |
| S0013 | COMMAND naming convention (imperative present tense) | Accepted |
| S0014 | POLICY pattern reference implementation | Accepted |
| S0015 | State directory naming convention | Accepted |
| S0016 | Root package file organization | Accepted |
| S0017 | Aggregate root and use case layer | Accepted |
| S0018 | Event Storming alignment and per-tool applicability | Accepted |
| S0019 | Data persistence boundaries (Linear/GitHub/local) | Accepted |

## sightjack-specific ADRs (0006~)

| # | Decision | Linear |
|---|----------|--------|
| [0006](0006-convergence-gate-design.md) | Convergence gate design | MY-355 |
| [0007](0007-fake-claude-e2e-testing.md) | fake-Claude E2E testing | MY-340 |
| [0008](0008-event-sourcing-state-management.md) | Event sourcing state management | — |
| [0009](0009-event-validation-and-concurrency.md) | Event validation and concurrency control | — |
| [0010](0010-root-package-file-consolidation.md) | Root package file consolidation | — |
| [0011](0011-es-layer-first-refactoring.md) | ES layer-first refactoring | — |
| [0012](0012-root-package-io-cleanup.md) | Root package I/O cleanup | — |
