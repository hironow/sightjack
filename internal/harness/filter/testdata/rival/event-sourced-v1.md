# Contract: Add session expiry enforcement

## Intent
- Prevent expired sessions from authorizing API calls.
- Success means expired sessions return 401 and active sessions continue to work.

## Domain
- Command: ValidateSession.
- Event: SessionValidationFailed.
- Read model: AuthMiddlewareView (session status, expiry timestamp).
- Aggregate: Session.

## Decisions
- Enforce expiry at command-handler boundary, not in the read-model projection.

## Steps
1. Implement `ValidateSession` command handler enforcing expiry.
   - Target: `internal/auth/session_command_handler.go`
   - Acceptance: expired sessions emit `SessionValidationFailed` event.
2. Project `AuthMiddlewareView` from the new event.
   - Target: `internal/auth/projection_auth_middleware.go`
   - Acceptance: middleware reads from view only.

## Boundaries
- Do not bypass the aggregate by mutating session state directly.
- Do not introduce CRUD shortcuts.

## Evidence
- test: just test
- lint: just lint
- nfr.p95_latency_ms: <= 200
