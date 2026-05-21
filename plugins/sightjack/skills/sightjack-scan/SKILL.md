---
name: sightjack-scan
description: >-
  Phase 2a slash command for the sightjack jun15 MCP pivot
  (refs/issues/0027). Triggers when the user types "/sightjack-scan",
  asks to "run a sightjack scan via MCP", "fetch the next wave from
  sightjack", or "test the sightjack MCP server end-to-end". Drives
  the sightjack MCP server's stub tools (next_wave / get_scan_result
  / update_strictness) from inside a human-initiated claude code
  interactive session so inference stays on the subscription quota
  rather than the Agent SDK credit pool that gates `claude -p` from
  2026-06-15.
version: 0.1.0
argument-hint: "(none) - reads next wave from sightjack MCP and surfaces the stub contract"
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

# /sightjack-scan — sightjack MCP pivot Phase 2a

Human-initiated entry point. Drives the sightjack MCP server's tools
without ever invoking `claude -p`, so all inference happens inside
this interactive claude code session's subscription quota.

## Prerequisites

The session was launched with the sightjack MCP server attached:

```bash
claude --mcp-config '{"sightjack":{"command":"sightjack","args":["mcp"]}}'
```

If `sightjack mcp` is not on PATH, build it first:

```bash
cd path/to/sightjack && go build -o ./dist/sightjack ./cmd/sightjack
```

## Workflow

1. **Verify MCP wiring**. Call `mcp__sightjack__sightjack_ping`. The
   tool must return `pong`. If it errors, the MCP server is not
   attached — abort and ask the human to relaunch claude with
   `--mcp-config`.

2. **Fetch the next wave**. Call
   `mcp__sightjack__sightjack_next_wave` with no arguments. During
   Phase 2a the response is a stub:

   ```json
   {
     "stub": true,
     "wave": null,
     "reason": "phase-2a-mvp: real implementation lands when ...",
     "contract": {"id": "string", "title": "string", "cluster_name": "string", "status": "string", "issues": "array of issue ids"}
   }
   ```

   While `stub == true`, **do NOT proceed to D-Mail emit or wave
   application**. Surface the contract descriptor so the human can
   verify the shape, and stop. Real wiring lands in a subsequent
   commit on the `feat/jun15-mcp-pivot` branch.

3. **(Post-stub) Apply the wave**. Read the wave body, plan the
   implementation, and apply edits via Read / Edit / Write / Bash.
   No `claude -p` invocations are allowed at any point.

4. **(Post-stub) Update strictness if rate-limit signals appear**.
   Call `mcp__sightjack__sightjack_update_strictness` with
   `{"level": "lax|normal|strict"}` to adjust the scan strictness.
   Phase 2a stub echoes the requested level with a placeholder
   `previous_level`.

5. **(Post-stub) Inspect scan history**. Call
   `mcp__sightjack__sightjack_get_scan_result` with
   `{"session_id": "..."}` to fetch persisted ScanResult state.
   Phase 2a stub echoes the session_id with a contract descriptor
   for the real shape.

## What this skill must NOT do

- Invoke `claude -p`, `claude --print`, the Anthropic Agent SDK, or
  any shell wrapper that does so (= refs/issues/0027 §5 billing
  boundary). The repo-wide semgrep gate
  (`.semgrep/jun15-no-headless-llm.yaml`) blocks these patterns in
  production code.
- Auto-trigger inference from a SessionStart hook or any other
  non-human-initiated path. The slash command typed by a human is
  the only valid entry to this workflow.
- Emit a D-Mail by writing to `outbox/` directly. The
  `sightjack.emit_dmail` MCP tool ships in a later commit; that tool
  encapsulates the transactional outbox + the 9-field schema fixed
  in refs 0027 §8.

## Phase 2a MVP exit criteria

This skill is considered Phase 2a MVP complete when:

1. Calling `/sightjack-scan` in a real claude code session with the
   sightjack MCP server attached returns the stub responses from
   steps 1-2 without error.
2. The `claude_adapter.go` and `doctor.go` `claude --print`
   invocations are removed and the semgrep transitional excludes
   on those two files are deleted (= the final commit on the
   `feat/jun15-mcp-pivot` branch flips the lint gate from advisory
   to enforced).

## Related

- Canonical plan: `refs/HTMLification/docs/issues/0027-jun15-mcp-pivot.html`
- Pattern reference: paintress ADR 0017 (`~/tap/paintress/docs/adr/0017-mcp-pivot.md`)
- Billing boundary table: refs 0027 §5
- Mechanical gate (semgrep rules): refs 0027 §6 + `.semgrep/jun15-no-headless-llm.yaml`
- D-Mail 9-field schema: refs 0027 §8 + paintress `internal/domain/dmail_envelope.go`
