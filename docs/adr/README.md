# Architecture Decision Records

## Numbering Scheme

| Range | Scope | Description |
|-------|-------|-------------|
| 0000-0005 | Shared (canonical: phonewave) | Cross-tool decisions. All 4 tools follow these. |
| 0006+ (per tool) | Tool-specific | Each tool numbers its own ADRs starting from 0006. |
| S00XX | Shared additions (canonical: phonewave) | Post-initial shared decisions added during alignment. |

- **Shared ADRs (0000-0005)** live only in phonewave `docs/adr/`. Other tools reference them but do not copy them.
- **Tool-specific ADRs (0006+)** live in each tool's own `docs/adr/` with numbering starting at 0006.
- **S-series ADRs** are shared decisions added after the initial 0000-0005 set. They also live only in phonewave.
- Semgrep rules enforcing shared ADRs are copied to each tool's `.semgrep/shared-adr.yaml`.

## Shared ADRs (canonical: phonewave)

0000-0005 are reserved. Canonical versions live in `phonewave/docs/adr/`.

| # | Decision | Linear |
|---|----------|--------|
| 0001 | cobra CLI framework adoption | MY-329 |
| 0002 | stdio convention (stdout=data, stderr=logs) | MY-339 |
| 0003 | OpenTelemetry noop-default + OTLP HTTP | — |
| 0004 | D-Mail Schema v1 specification | MY-352, MY-353 |
| 0005 | fsnotify daemon design | — |

## S-series Shared ADRs (canonical: phonewave)

Canonical versions live in phonewave `docs/adr/`. Referenced here for discoverability.

| # | Decision | Status |
|---|----------|--------|
| S0011 | SQLite WAL cooperative model for concurrent CLI | Accepted |
| S0012 | Reference data management pattern | Accepted |
| S0013 | COMMAND naming convention (imperative present tense) | Accepted |
| S0014 | POLICY pattern reference implementation | Accepted |
| S0015 | State directory naming convention | Accepted |
| S0016 | Root package file organization | Accepted |
| S0017 | Aggregate root and use case layer | Accepted |
| S0018 | Event Storming alignment and per-tool applicability | Accepted |
| S0019 | Data persistence boundaries (Linear/GitHub/local) | Accepted |
| S0020 | Accepted cross-tool divergence (default subcommand, storage model) | Accepted |
| S0021 | D-Mail receive-side validation (Postel's Law) | Accepted |
| S0022 | OTel Metrics Design | Accepted |
| S0023 | Cross-Tool Contract Testing | Accepted |
| S0024 | CLI Argument Design Decisions | Accepted |
| S0025 | Event Delivery Guarantee Levels | Accepted |
| S0026 | Domain Model Maturity Assessment | Accepted |
| S0027 | RDRA Gap Resolution — D-Mail Protocol Extension | Accepted |

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
