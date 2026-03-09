# 0012. Strictness Redefinition

**Date:** 2026-03-09
**Status:** Accepted

## Context

Strictness was originally defined as "DoD analysis depth" (fog=warning only, alert=sub-issue proposal, lockdown=flag all). This conflated two independent concerns: how deeply to analyze DoD and how aggressively to change existing implementations. As the system matured with scan-time issue status tracking and cross-tool feedback via D-Mails, a clearer separation was needed.

The key insight: DoD analysis should ALWAYS be thorough regardless of how much existing work is at stake. Strictness should instead control the tolerance for changing what has already been built.

## Decision

1. **Redefine Strictness** as "change tolerance for existing implementations":
   - fog: No protection. Free to cancel and rebuild.
   - alert: Respect existing implementations. Limit blast radius.
   - lockdown: Protect existing structure. Additive changes only.

2. **DoD checking is always full-depth** regardless of Strictness level.

3. **3-layer resolution** with max semantics: `max(override, estimated, default)`. Strictness can only go up, never down.

4. **Scan-time estimation**: LLM evaluates issue status distribution and code analysis during Pass 2 deep scan to produce per-cluster estimated strictness.

5. **Estimated values persisted to config**: Written to `sightjack.yaml` under `strictness.estimated` after each scan. Git-tracked for team visibility.

6. **Cancel action type**: New wave action for cancelling Backlog/Todo issues when feedback requires fundamental changes.

7. **Cluster key**: English slug alongside display name for stable YAML key references.

## Consequences

### Positive

- Clear separation between analysis depth and change tolerance
- Scan-time estimation prevents stale strictness values
- Cancel action enables clean feedback-driven course corrections
- 3-layer max ensures strictness never accidentally decreases

### Negative

- Breaking change: overrides can no longer lower strictness below default
- Scan-time estimation adds LLM cost to every deep scan
- Estimated values in config may surprise users unfamiliar with auto-generation

### Neutral

- All prompt templates updated to reflect new definition
- Issue status now tracked in scan results (enables future status-based policies)
