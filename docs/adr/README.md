# Architecture Decision Records

## Shared ADRs (canonical: phonewave)

0001-0005 are reserved. Canonical versions live in [phonewave docs/adr/](https://github.com/hironow/phonewave/tree/main/docs/adr).

| # | Decision | Linear |
|---|----------|--------|
| 0001 | cobra CLI framework adoption | MY-329 |
| 0002 | stdio convention (stdout=data, stderr=logs) | MY-339 |
| 0003 | OpenTelemetry noop-default + OTLP HTTP | — |
| 0004 | D-Mail Schema v1 specification | MY-352, MY-353 |
| 0005 | fsnotify daemon design | — |

## Extended Shared ADRs (S-series, canonical: phonewave)

| # | Decision |
|---|----------|
| S0001 | ~~Logger as root package exception~~ (superseded by S0005) |
| S0002 | JSONL append-only event sourcing pattern |
| S0003 | Three-way approval contract |
| S0004 | ~~Layer architecture conventions~~ (superseded by S0005) |
| S0005 | Root infrastructure pattern and layer conventions |

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
