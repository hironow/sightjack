// Package port defines context-aware interface contracts and trivial default
// implementations (null objects) for the port-adapter pattern.
// Concrete I/O implementations live in session and platform layers.
// Port may only import domain (+ stdlib such as context, errors).
// No imports of upper internal layers (cmd, usecase, session, eventsource, platform).
//
// This package lives under internal/usecase/ because port contracts are
// consumed primarily by the usecase and session layers.
package port
