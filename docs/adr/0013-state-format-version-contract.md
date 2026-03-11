# 0013. State Format Version Backward-Compatibility Contract

**Date:** 2026-03-11
**Status:** Accepted

## Context

`StateFormatVersion` was `"0.0.11"`, matching an old sightjack release
number. This caused confusion because the constant tracks the on-disk
JSON schema, not the tool release version.

With the move away from `v0.0.x` release numbering, we reset the format
version to `"1"` and establish a formal backward-compatibility contract.

## Decision

1. Reset `StateFormatVersion` from `"0.0.11"` to `"1"`.
2. Readers MUST accept all prior format versions. The known set is
   `{"0.0.11", "1"}` at the time of this ADR.
3. Writers MUST always emit the current `StateFormatVersion`.
4. When the `SessionState` JSON structure changes incompatibly, bump
   the version and add a migration path for every prior version.

## Consequences

### Positive

- Clear separation between release version and wire-format version
- Explicit contract prevents accidental breaking of existing state files
- Clean numbering going forward

### Negative

- Must maintain migration code for each prior version indefinitely
