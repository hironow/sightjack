# 0011. D-Mail Waiting Mode

**Date:** 2026-03-09
**Status:** Accepted

## Context

After all waves are processed, sightjack exits the interactive loop and terminates the session. When D-Mails arrive from other tools (amadeus, paintress) after processing completes, users must manually restart sightjack to process them. This breaks the continuous feedback loop that the D-Mail protocol is designed to support.

## Decision

Add an inbox-driven waiting mode to sightjack's interactive loop. After all waves are processed, instead of exiting, sightjack enters a waiting phase that monitors the existing fsnotify-based FeedbackCollector for new D-Mail arrivals.

Key design choices:

1. **Extend existing fsnotify infrastructure** — MonitorInbox already uses fsnotify and feeds a channel to FeedbackCollector. The waiting phase watches the same NotifyCh channel. No new monitoring infrastructure needed.

2. **Timeout-based exit** — Default 30-minute timeout (configurable via `--wait-timeout`). `timeout=0` means no timeout (wait indefinitely). Negative timeout disables waiting mode entirely.

3. **Resume logic by D-Mail kind** — Specification D-Mails log a rescan notice (future: trigger rescan). Feedback/report D-Mails resume the interactive loop with re-evaluated wave unlocks.

4. **FeedbackCollector notification channel** — Added buffered `chan struct{}` (size 1) with non-blocking send after each `addMail`. `Snapshot()` and `NewSinceSnapshot()` track arrivals during the waiting phase.

5. **User quit semantics preserved** — The `outerLoop` label and `break outerLoop` for user quit ('q') only breaks the inner loop. A `userQuit` flag propagates to break the outer waiting cycle.

## Consequences

### Positive

- Transforms sightjack from one-shot to persistent session without daemon infrastructure
- Leverages existing fsnotify watcher (zero additional OS resources)
- All three entry points (RunSession, RunResumeSession, RunRescanSession) gain waiting mode automatically
- Configurable timeout provides flexibility for different workflows

### Negative

- Specification D-Mail rescan is not yet implemented (logged but not acted upon)
- Waiting mode adds complexity to the interactive loop control flow (two nested loops with labels)

### Neutral

- Integration tests disable waiting mode via `WaitTimeout: -1` to avoid blocking
- The `--wait-timeout` flag follows existing Cobra flag override pattern
