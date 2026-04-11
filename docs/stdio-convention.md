# stdio Convention

Sightjack follows the Unix convention of separating machine-readable data from human-readable diagnostics across standard streams.

## Stream Assignment

| Stream | Purpose | Implementation |
|--------|---------|----------------|
| **stdout** | Machine-readable output (JSON, scan results) | `cmd.OutOrStdout()` |
| **stderr** | Human-readable progress, logs, errors | `cmd.ErrOrStderr()` |
| **stdin** | Prompt input to provider CLI subprocess only | `ProviderRunner.Run()` internal |

## Cobra Wiring

All cobra subcommands MUST use cobra's stream accessors:

```go
logger := platform.NewLogger(cmd.ErrOrStderr(), verbose)
```

Rules:

- Use `cmd.OutOrStdout()` for data output — never `os.Stdout` directly
- Use `cmd.ErrOrStderr()` for logs — never `os.Stderr` directly
- This enables cobra's `cmd.SetOut()` / `cmd.SetErr()` for testing

### Exceptions

Direct `os.Stderr` is acceptable only where cobra's `cmd` is unavailable:

| Location | Reason |
|----------|--------|
| `cmd/sightjack/main.go` | Error handling after `root.ExecuteContext()` returns |
| `internal/tools/docgen/main.go` | Standalone tool outside cobra |

## Pipeline Compatibility

The stream separation ensures correct behavior in Unix pipelines:

```bash
sightjack scan --json | jq '.waves'    # stdout = JSON only
sightjack scan --json 2>/dev/null      # suppress stderr logs
sightjack scan --json 2>scan.log       # split logs to file
```
