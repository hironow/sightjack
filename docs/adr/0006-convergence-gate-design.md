# 0006. Convergence Gate Design

**Date:** 2026-02-23
**Status:** Accepted

## Context

D-Mail convergence signals indicate that an upstream tool (e.g., amadeus) has
detected a significant state change requiring human attention before proceeding.
sightjack sessions that receive convergence D-Mails must pause and obtain
explicit human approval before continuing. A naive implementation risks either
silently swallowing the signal (unsafe) or blocking indefinitely on stdin
(poor UX in automated environments).

MY-355 defined the requirements: notification, interactive approval prompt,
CI-friendly auto-approve, and fail-closed semantics.

## Decision

Implement a convergence gate architecture with the following properties:

1. **Fail-closed default**: Any error in the approval flow denies the gate.
   `RunConvergenceGate` returns `(false, err)` on failure, never `(true, err)`.

2. **Notify + Approve separation**: `Notifier` (fire-and-forget, non-blocking
   with 30s timeout) and `Approver` (blocking, fail-closed) are independent
   interfaces. Notification failure does not block approval.

3. **Three Approver implementations**:
   - `StdinApprover`: Interactive terminal prompt. Reads one byte at a time
     (`readLine`) to avoid consuming data beyond the newline from a shared
     reader (e.g., stdin). Context cancellation via goroutine + channel;
     the goroutine may leak until process exit, which is acceptable for a
     single approval prompt. The reader is intentionally NOT closed on cancel
     (closing fd 0 would break subsequent reads).
   - `CmdApprover`: External command with `{message}` placeholder.
     Exit 0 = approve, non-zero `ExitError` = deny, other error = fail.
     Shell injection prevented via `ShellQuote()`.
   - `AutoApprover`: Always approves. Used with `--auto-approve` flag for CI.

4. **Redrain loop**: `RunConvergenceGateWithRedrain` re-drains the inbox
   channel after each approval to catch convergence D-Mails that arrived
   during the approval prompt. The loop exits when no new convergence
   D-Mails are found in the drain.

5. **CLI flags**: `--notify-cmd`, `--approve-cmd`, `--auto-approve` on the
   `run` subcommand. Config file values are overridden when flags are
   explicitly set (cobra `Changed()` pattern).

## Consequences

### Positive

- Fail-closed semantics prevent accidental continuation past convergence signals
- StdinApprover's byte-at-a-time read preserves shared stdin for subsequent reads
- Redrain loop closes the TOCTOU race window between drain and gate check
- `--auto-approve` enables unattended CI/CD usage

### Negative

- StdinApprover goroutine leaks on context cancel (acceptable: single prompt,
  cleaned up on process exit)
- Redrain loop may prompt multiple times if convergence signals keep arriving

### Neutral

- Notification runs in a detached goroutine with independent span context
  so it does not block gate evaluation or get cancelled by gate context
