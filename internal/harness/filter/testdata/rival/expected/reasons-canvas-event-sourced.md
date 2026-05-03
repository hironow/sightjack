# Add session expiry enforcement

## Requirements

- Prevent expired sessions from authorizing API calls.
- Success means expired sessions return 401 and active sessions continue to work.

## Entities

### Commands

- ValidateSession

### Events

- SessionValidationFailed

### Read Models

- AuthMiddlewareView (session status, expiry timestamp)

### Aggregates

- Session

## Approach

- Enforce expiry at command-handler boundary, not in the read-model projection.

## Structure

- `internal/auth/session_command_handler.go`
- `internal/auth/projection_auth_middleware.go`

## Operations

1. Implement `ValidateSession` command handler enforcing expiry.
   - Target: `internal/auth/session_command_handler.go`
   - Acceptance: expired sessions emit `SessionValidationFailed` event.
2. Project `AuthMiddlewareView` from the new event.
   - Target: `internal/auth/projection_auth_middleware.go`
   - Acceptance: middleware reads from view only.

## Norms

_(none)_

## Safeguards

- Do not bypass the aggregate by mutating session state directly.
- Do not introduce CRUD shortcuts.

## Validation

- test: just test
- lint: just lint
- nfr.p95_latency_ms: <= 200

## Sync

Source: D-Mail spec-es_bbbbbbbb, revision 2, supersedes spec-es_aaaaaaaa

