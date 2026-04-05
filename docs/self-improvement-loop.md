# sightjack self-improvement loop

## Purpose

`sightjack` is the design and specification side of the 4-tool loop.

It sits on the path:

`specification -> implementation -> verification -> correction`

and is responsible for taking design-oriented corrective feedback and turning it into a better next wave, not just regenerating specs blindly.

## What this tool now does

`sightjack` now participates in the observable self-improvement loop by carrying corrective context through reruns.

The current implementation does four things:

1. It matches incoming corrective feedback to the relevant wave.
2. It carries normalized corrective metadata into rerun-linked report and feedback D-Mails.
3. It preserves correlation while clearing route-narrowing fields that should not leak into reports.
4. It stores provider pause state in coding session metadata using the shared provider-state vocabulary.

## Shared corrective metadata

The rerun path can carry metadata such as:

- `failure_type`
- `secondary_type`
- `target_agent`
- `recurrence_count`
- `corrective_action`
- `retry_allowed`
- `escalation_reason`
- `correlation_id`
- `trace_id`
- `outcome`

For `sightjack`, this metadata is mainly used to keep a design correction thread attached to the next generated wave output.

## Corrective rerun behavior

`sightjack` does not diagnose the failure by itself.

Instead, it preserves the diagnosis coming from the corrective D-Mail and makes sure the next report or feedback still belongs to the same thread. Matching prefers:

1. wave identity
2. issue overlap fallback

This keeps the design rerun visible to `amadeus` and the rest of the loop.

## Provider pause model

`sightjack` uses the shared provider-state snapshot:

- `active`
- `waiting`
- `degraded`
- `paused`

Those states are persisted into coding session metadata together with:

- `provider_state`
- `provider_reason`
- `provider_retry_budget`
- `provider_resume_at`
- `provider_resume_when`

This allows later tooling to distinguish "provider unavailable, wait" from ordinary session failure.

## Current scope

What is in:

- rerun correlation for corrective design feedback
- carry-forward of retry and escalation context
- provider pause state snapshots in session metadata

What is not in yet:

- learned spec-enrichment policies
- Weave-fed improvement planning
- a dedicated improvement controller outside the existing loop

