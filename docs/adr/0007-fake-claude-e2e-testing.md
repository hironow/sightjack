# 0007. fake-Claude E2E Testing

**Date:** 2026-02-23
**Status:** Accepted

## Context

sightjack's scan/wave pipeline invokes the Claude Code CLI (`claude -p ...`)
as a subprocess. Testing this pipeline end-to-end requires either a real Claude
API key (expensive, non-deterministic, rate-limited) or a test double. The test
double must be a standalone binary that replaces `claude` in `$PATH` without
requiring any changes to sightjack's production code.

MY-340 established the discussion around fake-Claude requirements and ownership.

## Decision

Implement fake-Claude as a standalone Go binary with fixture-based responses:

1. **Production code zero change**: fake-Claude is installed as
   `/usr/local/bin/claude` in the E2E Docker container. sightjack's
   `cfg.Claude.Command = "claude"` resolves to this binary via `$PATH`.
   No conditional logic, feature flags, or DI overrides in production code.

2. **Fixture pattern matching**: The binary extracts the JSON output path
   from the `-p` flag value using a regex, matches the filename against a
   built-in fixture table (`filepath.Match` patterns), and writes canned
   JSON to that path.

3. **stdout silence**: fake-Claude produces no stdout output. All data
   exchange is file-based (JSON output path). This prevents leaked output
   from contaminating sightjack's stdout pipe (consistent with shared
   ADR 0002 stdio convention).

4. **Docker-based isolation**: E2E tests run in `tests/e2e/compose-e2e.yaml`
   with fake-Claude built and installed at container build time. The host
   system is never modified.

5. **Prompt logging**: When `FAKE_CLAUDE_PROMPT_LOG_DIR` is set, fake-Claude
   logs each received prompt to a sequentially-named file. E2E tests use
   this to verify feedback injection into nextgen prompts.

## Consequences

### Positive

- Zero production code modification for testability
- Deterministic, fast E2E tests (no API calls, no rate limits)
- Fixture table is trivially extensible for new scan phases
- Prompt logging enables assertion on prompt construction without parsing stdout

### Negative

- Fixture responses are static; cannot test dynamic Claude behavior
- Adding new scan phases requires updating both fixture table and E2E test expectations
- Binary must be rebuilt when fixture content changes

### Neutral

- fake-Claude fixtures are ported from `lifecycle_test.go` canned responses,
  maintaining consistency between unit and E2E test expectations
