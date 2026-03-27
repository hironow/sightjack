# sightjack docs

## Architecture

- [conformance.md](conformance.md) — What/Why/How conformance table (single source)
- [siren-directory.md](siren-directory.md) — `.siren/` directory structure specification
- [policies.md](policies.md) — Event → Policy mapping (WHEN event THEN command)
- [otel-backends.md](otel-backends.md) — OpenTelemetry backend configuration (Jaeger, Weave)
- Claude subprocess isolation: `mcp-config generate` creates `.mcp.json` (MCP allowlist) and `.claude/settings.json` (plugin isolation); `--setting-sources ""` + `--settings` + `--strict-mcp-config` enforces it
- Claude log persistence: raw NDJSON saved to `.run/claude-logs/` after each invocation

- [dmail-protocol-conventions.md](dmail-protocol-conventions.md) — D-Mail filename uniqueness and archive retention conventions
- [testing.md](testing.md) — Test strategy, conventions, scenario test observers, wave lifecycle guards, and error fingerprinting

## CLI Reference

- [sightjack](cli/sightjack.md) — Root command
- [sightjack init](cli/sightjack_init.md) — Create .siren/config.yaml
- [sightjack config](cli/sightjack_config.md) — View or update configuration
- [sightjack config show](cli/sightjack_config_show.md) — Show current configuration
- [sightjack config set](cli/sightjack_config_set.md) — Update configuration values
- [sightjack scan](cli/sightjack_scan.md) — Classify and deep-scan Linear issues
- [sightjack run](cli/sightjack_run.md) — Interactive wave approval and apply loop
- [sightjack waves](cli/sightjack_waves.md) — Generate waves from stdin ScanResult JSON
- [sightjack show](cli/sightjack_show.md) — Display last scan results
- [sightjack select](cli/sightjack_select.md) — Interactively pick a wave from stdin WavePlan
- [sightjack discuss](cli/sightjack_discuss.md) — Architect discussion from stdin Wave JSON
- [sightjack apply](cli/sightjack_apply.md) — Apply a wave to Linear from stdin Wave JSON
- [sightjack adr](cli/sightjack_adr.md) — Generate ADR Markdown from stdin DiscussResult
- [sightjack nextgen](cli/sightjack_nextgen.md) — Generate follow-up waves from stdin ApplyResult
- [sightjack status](cli/sightjack_status.md) — Show operational status
- [sightjack doctor](cli/sightjack_doctor.md) — Check environment and tool availability
- [sightjack clean](cli/sightjack_clean.md) — Remove state directory (.siren/)
- [sightjack archive-prune](cli/sightjack_archive-prune.md) — Prune expired d-mails and event files
- [sightjack version](cli/sightjack_version.md) — Print version, commit, and build information
- [sightjack update](cli/sightjack_update.md) — Self-update sightjack to the latest release

## Architecture Decision Records

- [adr/](adr/README.md) — Tool-specific ADRs
- [shared-adr/](shared-adr/README.md) — Cross-tool shared ADRs (S0001–S0035)
