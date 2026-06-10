# 0019. MCP write tools and project wiring (designer write-path restoration)

**Date:** 2026-06-10
**Status:** Accepted

## Context

The jun15 MCP pivot (ADR 0018) retired the headless designer pipeline
but rebuilt only the read side of the MCP surface. Verified on
2026-06-10 (refs issue 0032): no production path could persist scan
results or waves (`EventWavesGenerated` was emitted nowhere), so
`next_wave` / `get_scan_result` served frozen pre-pivot state; D-Mail
emission had no sanctioned path (direct `outbox/` writes would bypass
the transactional outbox); and the entry skill had no distribution
mechanism (zero invocations to date). Claude Code conformance
constraints C1-C6 (refs issue 0032 §5) bound the design.

## Decision

1. **Dot-free tool names** (C1): `sightjack.ping` → `ping` etc. The
   client-side namespace is the server name (`mcp__sightjack__<tool>`);
   dotted names risked mismatching allowed-tools patterns because dot
   normalization is undocumented.
2. **Write tools** `save_scan_result` / `register_waves`: narrow port
   `ScanWriteEmitter` (satisfied by the existing session emitter),
   injected at the composition root with a files-only degradation when
   unwired (dominator ADR 0005 pattern). Write order is pinned:
   scan-dir JSON read models first, then the event-ledger append;
   append failure reports `persistence: "files-only"` + reason and the
   session repairs by re-running (at-least-once + idempotent overwrite
   per cluster — no projector is introduced).
3. **`dmail` emission tool**: typed D-Mail v1 subset mapped 1:1 onto
   `domain.DMail`, validated by `ValidateDMail` plus the producer
   subset `domain.ProducesKinds` (specification / report /
   stall-escalation, mirroring the dmail-sendable manifest), staged and
   flushed through the existing transactional outbox (`ComposeDMail`).
4. **Project wiring** (C4/C5, decision D5(a)): `sightjack init`
   materializes the entry skill into the target project's
   `.claude/skills/` (embedded template is the single source of truth;
   no plugin manifest machinery), and `mcp-config generate` upserts the
   project-root `.mcp.json` merge-aware so sibling tools share one
   omni-session config. The state-dir `.mcp.json` remains for isolated
   `sessions enter`.
5. **`instructions` in the initialize handshake** (C6) for Tool Search
   deferred loading.

## Consequences

### Positive

- The designer loop functions again end-to-end: design → persist →
  serve → emit specification — with the event ledger intact.
- A bare `claude` session in an initialized project discovers
  `/sightjack-scan` and auto-attaches the server (zero-flag UX).
- Emission cannot bypass atomicity: the only D-Mail path is the
  outbox stage→flush.

### Negative

- Dual-write (files + events) admits a transient files-only state;
  the repair contract is manual re-run rather than a projector.
- Renaming tools is a wire-contract break (accepted while invocations
  are zero).

### Neutral

- `plugins/` becomes a pointer README; the skill's canonical source
  lives under `internal/platform/templates/claude-skills/`.
