# 0010. Auto-discuss ADR generation in auto-approve mode

**Date:** 2026-03-08
**Status:** Accepted

## Context

When sightjack runs in auto-approve mode (`--auto-approve` or `gate.auto_approve`
config), the interactive approval prompt is skipped entirely. ADR generation only
happened through the interactive "Discuss" path (`RunArchitectDiscuss()` ->
`RunScribeADR()`), which required human input. As a result, auto-approve mode
never generated ADRs, and architectural decisions accumulated without
documentation during autonomous operation.

## Decision

Add a Devil's Advocate auto-discussion mechanism that runs after wave
auto-approval but before specification composition. Two AI agents (Architect
and Devil's Advocate) debate the wave's design decisions in configurable
rounds (`scribe.auto_discuss_rounds`, default 2). The resulting discussion
feeds into the existing `RunScribeADR()` flow for ADR generation.

Key design choices:

- **Two separate prompt templates** called alternately (not one prompt with
  two personas). Separation maintains argumentative tension; single-prompt
  self-debate tends to converge prematurely.
- **Devil's Advocate derives challenge perspectives dynamically** from existing
  ADRs and CLAUDE.md principles, rather than using a fixed checklist.
- **`AutoDiscussResult.ToArchitectResponse()`** adapter converts debate output
  to the existing `ArchitectResponse` struct, so `RunScribeADR()` requires
  no changes.
- **`auto_discuss_rounds: 0`** skips auto-discuss entirely, preserving
  backward compatibility with existing auto-approve behavior.
- **All errors are non-fatal** (consistent with existing Discuss path):
  Claude call failures, missing CLAUDE.md, and individual round failures
  are warned and gracefully degraded.

## Consequences

### Positive

- Auto-approve mode now generates ADRs, preventing invisible architectural debt
- Positive feedback loop: more ADRs improve Devil's Advocate arguments, improving ADR quality
- Bounded by fixed round count and ScribeADR's "ADR not needed" escape hatch
- No changes to existing `RunScribeADR()` or `ComposeSpecification()`

### Negative

- Additional Claude API calls per wave in auto-approve mode (2N+1 calls for N rounds)
- Devil's Advocate quality depends on existing ADR corpus; initial runs with few ADRs produce weaker challenges

### Neutral

- Design-feedback D-Mails are threaded into the debate as additional context for the Architect agent
- OTEL spans added for observability (`scribe.auto_discuss`, `scribe.auto_discuss.round`)
