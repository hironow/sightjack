---
name: sightjack-scan
description: >-
  Slash command for the sightjack scanner (refs/issues/0027 jun15 MCP
  pivot). Triggers when the user types "/sightjack-scan", asks to
  "run a sightjack scan via MCP", "fetch the next wave from sightjack",
  or "test the sightjack MCP server end-to-end". Drives the sightjack
  MCP server's tools (next_wave / get_scan_result / update_strictness)
  from inside a human-initiated Claude Code interactive session so
  inference stays on the subscription quota rather than the Agent SDK
  credit pool that gates `claude -p` from 2026-06-15.
version: 0.1.0
argument-hint: "(none) - reads the next wave from sightjack MCP and applies it"
allowed-tools:
  - Read
  - Edit
  - Write
  - Bash
  - Grep
  - Glob
  - Agent
  - mcp__sightjack__sightjack_ping
  - mcp__sightjack__sightjack_next_wave
  - mcp__sightjack__sightjack_get_scan_result
  - mcp__sightjack__sightjack_update_strictness
---

# /sightjack-scan — sightjack scanner

Human-initiated entry point. Drives the sightjack MCP server's tools
without ever invoking `claude -p`, so all inference happens inside
this interactive Claude Code session's subscription quota.

## Prerequisites

The session was launched with the sightjack MCP server attached:

```bash
claude --mcp-config '{"sightjack":{"command":"sightjack","args":["mcp"]}}'
```

If `sightjack mcp` is not on PATH, build it first:

```bash
cd path/to/sightjack && go build -o ./dist/sightjack ./cmd/sightjack
```

`sightjack mcp` must be started from the project root so it can resolve
the base dir (`.siren/` config + scan state). The MCP server answers
the `initialize` handshake, then exposes ping / next_wave /
get_scan_result / update_strictness.

## Workflow

1. **Verify MCP wiring**. Call `mcp__sightjack__sightjack_ping`. The
   tool must return `pong`. If it errors, the MCP server is not
   attached — abort and ask the human to relaunch claude with
   `--mcp-config`.

2. **Fetch the next wave**. Call `mcp__sightjack__sightjack_next_wave`
   with no arguments. It returns the next wave (cluster of related
   issues) from sightjack's scan state. If `initialized == false`,
   **abort**: the operator launched `sightjack mcp` from outside a
   sightjack-initialized project root — ask them to relaunch from the
   project directory.

3. **Apply the wave**. Read the wave body, plan the implementation,
   and apply edits via Read / Edit / Write / Bash. No `claude -p`
   invocations are allowed at any point.

4. **Update strictness if rate-limit signals appear**. Call
   `mcp__sightjack__sightjack_update_strictness` with
   `{"level": "fog|alert|lockdown"}` to adjust the scan strictness.
   The tool validates the level and persists it to `.siren/config.yaml`
   atomically (`persistence: "config.yaml"`, or `"no-op"` when the
   level is unchanged).

5. **Inspect scan history**. Call
   `mcp__sightjack__sightjack_get_scan_result` with
   `{"session_id": "..."}` to fetch persisted ScanResult state for a
   prior scan session.

## What this skill must NOT do

- Invoke `claude -p`, `claude --print`, the Anthropic Agent SDK, or
  any shell wrapper that does so (= refs/issues/0027 §5 billing
  boundary). The repo-wide semgrep gate
  (`.semgrep/jun15-no-headless-llm.yaml`) blocks these patterns in
  production code.
- Auto-trigger inference from a SessionStart hook or any other
  non-human-initiated path. The slash command typed by a human is
  the only valid entry to this workflow.
- Emit a D-Mail by writing to `outbox/` directly. D-Mail emission is
  not exposed as an MCP tool in this skill's tool set; the strictness
  tool above is the canonical config-persistence path. The D-Mail
  9-field schema is fixed in refs 0027 §8.

## Done criteria

A `/sightjack-scan` run is complete when, in a real Claude Code session
with the sightjack MCP server attached:

1. `ping` returns `pong` (handshake + tool dispatch verified).
2. `next_wave` returns the wave (or `initialized: false` → abort).
3. The wave is applied and validated.
4. `update_strictness` (when used) returns `persistence: "config.yaml"`.

## Related

- Canonical plan: `refs/HTMLification/docs/archive/0027-jun15-mcp-pivot.html`
- Pattern reference: sightjack ADR 0018 (`~/tap/sightjack/docs/adr/0018-mcp-pivot.md`)
- Billing boundary table: refs 0027 §5
- Mechanical gate (semgrep rules): refs 0027 §6 + `.semgrep/jun15-no-headless-llm.yaml`
- D-Mail 9-field schema: refs 0027 §8 + paintress `internal/domain/dmail_envelope.go`
