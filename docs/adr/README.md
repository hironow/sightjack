# Architecture Decision Records — sightjack

## Shared ADRs (canonical: phonewave)

0001-0005 are reserved. Canonical versions live in `phonewave/docs/adr/`.

| # | Decision | Linear |
|---|----------|--------|
| 0001 | cobra CLI framework adoption | MY-329 |
| 0002 | stdio convention (stdout=data, stderr=logs) | MY-339 |
| 0003 | OpenTelemetry noop-default + OTLP HTTP | — |
| 0004 | D-Mail Schema v1 specification | MY-352, MY-353 |
| 0005 | fsnotify daemon design (phonewave-specific) | — |

## sightjack-specific ADRs

| # | Decision | Linear |
|---|----------|--------|
| 0006 | [Convergence gate design](0006-convergence-gate-design.md) | MY-355 |
| 0007 | [fake-Claude E2E testing](0007-fake-claude-e2e-testing.md) | MY-340 |
