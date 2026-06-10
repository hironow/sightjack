---
name: sightjack-scan
description: >-
  Slash command for the sightjack scanner (jun15 MCP pivot). Triggers
  when the user types "/sightjack-scan", asks to "run a sightjack scan
  via MCP", "fetch the next wave from sightjack", "次の wave を適用して",
  or "test the sightjack MCP server end-to-end". Drives the sightjack
  MCP server's tools (next_wave / get_scan_result / update_strictness)
  from inside a human-initiated Claude Code interactive session so
  inference stays on the subscription quota rather than the Agent SDK
  credit pool that gates `claude -p` from 2026-06-15.
version: 0.2.0
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

## Execution principle: one invocation = one wave

One `/sightjack-scan` run handles **exactly one wave**, then stops and
reports back to the human. Do not loop into the next wave automatically
— the human re-invokes the slash command for each unit. This keeps the
feedback loop negative (stable, human-paced) and prevents runaway
sessions.

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
   with no arguments. It returns the first `available` wave (cluster of
   related issues) from sightjack's scan state. If
   `initialized == false`, **abort**: the operator launched
   `sightjack mcp` from outside a sightjack-initialized project root —
   ask them to relaunch from the project directory. If no wave is
   available, report that and stop (do not invent work).

3. **Plan before applying**. Read the wave body and the affected files.
   Summarize the plan (files to touch, tests to run) to the human in
   one short paragraph **before** editing. Apply edits via
   Read / Edit / Write / Bash. No `claude -p` invocations are allowed
   at any point.

4. **Verify the wave**. Run the project's test / lint commands (e.g.
   `just test` + `just lint` when a justfile exists, otherwise the
   project's documented commands). A wave is only "applied" when the
   verification passes. If verification fails and you cannot fix it
   within the wave's scope, stop and report the failure — do not
   widen the scope.

5. **Update strictness if rate-limit signals appear**. Call
   `mcp__sightjack__sightjack_update_strictness` with
   `{"level": "fog|alert|lockdown"}` to adjust the scan strictness.
   The tool validates the level and persists it to `.siren/config.yaml`
   atomically (`persistence: "config.yaml"`, or `"no-op"` when the
   level is unchanged).

6. **Inspect scan history when context is needed**. Call
   `mcp__sightjack__sightjack_get_scan_result` with
   `{"session_id": "..."}` to fetch persisted ScanResult state for a
   prior scan session.

7. **Report**. End with: wave id, files changed, verification result,
   strictness changes, and what the human should do next (review /
   re-invoke for the next wave).

## Failure paths

- **MCP tool error mid-run**: report the tool name and the `reason`
  field, stop. Do not retry more than once.
- **Verification failure you cannot fix in-scope**: leave the working
  tree as-is, report exactly what failed (command + output tail), stop.
- **Ambiguous wave body**: ask the human instead of guessing — a wave
  is a contract, not a suggestion.

## Re-run idempotency

Re-invoking `/sightjack-scan` after a partial run is safe: `next_wave`
re-serves the same wave until it is completed, `update_strictness` is
a no-op when the level is unchanged, and file edits should be checked
against the current file state (Read before Edit) rather than assumed.

## What this skill must NOT do

- Invoke `claude -p`, `claude --print`, the Anthropic Agent SDK, or
  any shell wrapper that does so (= billing boundary). The repo-wide
  semgrep gate (`.semgrep/jun15-no-headless-llm.yaml`) blocks these
  patterns in production code.
- Auto-trigger inference from a SessionStart hook or any other
  non-human-initiated path. The slash command typed by a human is
  the only valid entry to this workflow.
- Emit a D-Mail by writing to `outbox/` directly. Direct writes bypass
  the transactional outbox (atomicity / idempotency / OTel audit), so
  they are forbidden even though the session composes the spec content.
  A `sightjack.dmail` emission tool does not exist yet — that gap is
  tracked in refs issue 0031; until it lands, spec D-Mail emission is
  out of scope for this skill.

## Done criteria

A `/sightjack-scan` run is complete when, in a real Claude Code session
with the sightjack MCP server attached:

1. `ping` returns `pong` (handshake + tool dispatch verified).
2. `next_wave` returns the wave (or `initialized: false` → abort).
3. The wave is applied AND the project's verification commands pass.
4. `update_strictness` (when used) returns `persistence: "config.yaml"`.
5. The closing report (wave id / changes / verification / next step)
   is delivered to the human.

## Related

- Canonical plan: `http://localhost:8765/docs/archive/0027-jun15-mcp-pivot.html` (refs)
- refs restructure + skill review: `http://localhost:8765/docs/issues/0030-refs-attic-restructure.html`
- D-Mail emission tool gap: `http://localhost:8765/docs/issues/0031-mcp-tool-surface-gaps.html`
- Pattern reference: sightjack ADR 0018 (`docs/adr/0018-mcp-pivot.md`)
- Mechanical gate (semgrep rules): `.semgrep/jun15-no-headless-llm.yaml`
- D-Mail 9-field schema: `internal/domain/dmail_envelope.go`
