---
name: dmail-sendable
description: Declares outbound D-Mail kinds for phonewave routing discovery.
license: Apache-2.0
metadata:
  dmail-schema-version: "1"
  produces:
    - kind: specification
      description: wave specification for the implementer (designed by /sightjack-scan)
    - kind: report
      description: designer-side status report for the verifier
    - kind: stall-escalation
      description: stalled-loop escalation to the verifier
---

D-Mail send capability for sightjack.
