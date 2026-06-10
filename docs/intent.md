# Intent

**Last updated:** 2026-06-10
**Requester:** hironow
**Status:** DRAFT — AI が README / git 履歴から起草。requester 未確認
**Work unit:** sightjack — MCP server + data plane for SIREN-inspired issue architecture

## Goal

Provide a pure data-plane Go CLI (`sightjack mcp`) that serves scan/wave read
models from the session's scan dir and persists scan strictness config over
MCP, so that a human-initiated Claude Code session (the LLM owner after the
jun15 MCP pivot) can drive the scan/wave workflow via the `/sightjack-scan`
skill and the sightjack MCP tools.

## Success Criteria

- `just check` passes (fmt, vet, golangci-lint, semgrep, root-guard, tests, docs-check) — quality gate defined in the justfile and wired into CI under `.github/`
- The four MCP tools documented in README respond as described: `sightjack.ping`, `sightjack.next_wave`, `sightjack.get_scan_result`, `sightjack.update_strictness` (covered by e2e tests incl. the MCP tools-list handshake test, migrated to testcontainers-go)
- Product-level success criteria beyond these mechanical gates: 未定義 — Open Questions 参照

## Scope

### In scope

- MCP server / data plane: serving scan results and wave plans from the session's scan dir
- Atomic persistence of scan strictness to `.siren/config.yaml`
- Supporting data-plane commands and the `plugins/sightjack` Claude Code skill assets

### Out of scope (Non-goals)

- The headless designer pipeline (classify/deep-scan, wave generation, interactive approve loop, Architect/Scribe discuss + ADR steps, D-Mail composition) — explicitly retired per README after the MCP pivot
- Driving the LLM or composing D-Mails from the Go binary — that now lives in the claude-code session

## Constraints

- Go module; lint/semgrep/test gates enforced via justfile recipes and pre-commit hooks (`.golangci.yaml`, `.semgrep/`, `.pre-commit-config.yaml`)
- Part of the D-Mail protocol ecosystem (Designer role, `.siren/` endpoint) alongside paintress / amadeus / phonewave
- Released via GoReleaser (`.goreleaser.yaml`) and distributed through the `hironow/homebrew-tap` cask

## Open Questions

- [ ] requester による本ドラフトのレビュー
- [ ] Product-level success criteria for the MCP pivot (when is the data-plane scope "done"?) — not stated in README or docs
- [ ] Deadlines or milestone targets — none found in the repo
- [ ] `docs/intent.md` and `docs/handover.md` are listed in `.gitignore` — was that intentional, and should this PR keep them tracked or should the ignore entries be removed?
- [ ] Disposition of the items in `docs/decision-queue.md` (added 2026-06-10, #249) — which are still open?
