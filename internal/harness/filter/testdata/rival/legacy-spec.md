# Add session expiry enforcement

## Actions
- Add expiry check to auth middleware in `internal/http/auth_middleware.go`.
- Add unit tests for active, expired, and missing sessions.
- Update existing handler integration test if behavior changes.

## Acceptance
- Expired sessions return 401.
- Active sessions still authorize.
