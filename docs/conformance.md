# What / Why / How Conformance

This is the single source of truth for sightjack's purpose, design rationale, and implementation approach.
Referenced from [README.md](../README.md) and [docs/README.md](README.md).

| Aspect | Description |
|--------|-------------|
| **What** | Interactive AI session that analyzes Linear issues for completeness, dependencies, and architectural gaps |
| **Why** | Bring issue completeness from ~30% to ~85% before autonomous execution begins |
| **How** | Claude MCP tools scan issues → cluster analysis (with estimated strictness) → wave generation → interactive approval (with cancel action) → apply to Linear |
| **Input** | Linear issues via Claude MCP tools, user approval via stdin |
| **Output** | Updated Linear issues, D-Mail reports to downstream tools |
| **Telemetry** | OTel spans: `sightjack.scan`, `claude.invoke` (with `claude.model`, `claude.timeout_sec`, `gen_ai.*`), `context_budget.*` (`context_budget.tools`, `context_budget.skills`, `context_budget.plugins`, `context_budget.mcp_servers`, `context_budget.hook_bytes`, `context_budget.estimated_tokens`) |
| **External Systems** | Linear (via Claude MCP), Claude Code subprocess, OTel exporter (Jaeger/Weave) |

## Layer Architecture

```
cmd              --> usecase, session, usecase/port, platform, domain  (composition root)
usecase          --> usecase/port, domain                              (output port only)
usecase/port     --> domain (+ stdlib)                                 (interface contracts)
session          --> eventsource, usecase/port, platform, domain       (adapter impl)
eventsource      --> domain                                            (event persistence adapter)
platform         --> domain (+ stdlib)                                 (cross-cutting infra)
domain           --> (nothing internal, stdlib only)                   (pure types/logic)
```

`eventsource` is the event persistence adapter based on the [AWS Event Sourcing pattern](https://docs.aws.amazon.com/prescriptive-guidance/latest/cloud-design-patterns/event-sourcing.html).
Its responsibility is limited to append, load, and replay of domain events.
Event store implementation MUST NOT exist outside `internal/eventsource`.
`session` uses `eventsource` as a client but does not implement event persistence itself.

Key constraints enforced by semgrep (ERROR severity):

- `usecase --> session` PROHIBITED (must use output port interfaces)
- `cmd --> eventsource` PROHIBITED (ADR S0008)
- `domain` has no I/O, no `context.Context`

Ref: `.semgrep/layers.yaml`, ADR S0007

## Domain Primitives & Parse-Don't-Validate

Domain command types use the Parse-Don't-Validate pattern:

- Domain primitives (`RepoPath`, `SessionID`, `ClusterName`, `Topic`, `Lang`) validate in `New*()` constructors — invalid values are rejected at parse time
- Command types use unexported fields with `New*Command()` constructors that accept only pre-validated primitives
- Commands are always-valid by construction — no `Validate() []error` methods exist
- Usecase layer receives always-valid commands with no validation boilerplate
- Semgrep rule `domain-no-validate-method` prevents reintroduction of `Validate() []error`

Ref: `.semgrep/layers.yaml`, ADR S0029

## Tracking Mode (Wave vs Linear)

### Claude Subprocess Isolation

- `--disable-slash-commands` prevents user skills from inflating context
- `mcp-config generate` creates `.run/mcp-config.json` (wave: empty, linear: Linear MCP)
- `--strict-mcp-config --mcp-config` enforced when mcp-config.json exists
- User can edit mcp-config.json to add custom MCP servers

- **Wave mode** (default, `--linear` not set): D-Mail archive is the event source for wave state. `AllowedToolsForMode(cfg.Mode)` excludes Linear MCP tools from Claude prompts. `RunReadyLabel` is skipped. `ComposeSpecification` populates the `wave` field with steps derived from WaveActions.
- **Linear mode** (`--linear`): Existing behavior preserved — Linear MCP tools included, labels applied, no wave field in D-Mails.
- `Config.Mode` (runtime-only, `yaml:"-"`) carries the tracking mode through all session functions.

Ref: ADR S0035, `internal/domain/primitives.go` (TrackingMode), `internal/session/claude.go` (AllowedToolsForMode)

## Cross-Tool Conformance

All 4 tools (phonewave, sightjack, paintress, amadeus) maintain a What/Why/How conformance table in `docs/conformance.md` with the same structure. This prevents expression drift across README files.
