package session

import "go.opentelemetry.io/otel/trace"

// SetTracer replaces the package-level tracer for testing and returns a cleanup function.
func SetTracer(t trace.Tracer) func() {
	old := tracer
	tracer = t
	return func() { tracer = old }
}
