// Package usecase orchestrates COMMAND → Aggregate → EVENT flows.
// It validates commands, creates/restores aggregates, and delegates I/O to the session layer.
package usecase
