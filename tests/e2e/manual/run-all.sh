#!/usr/bin/env bash
set -uo pipefail

# Manual E2E test script for sightjack.
# Run inside the Docker container: docker compose -f tests/e2e/compose-e2e.yaml run --rm -it e2e /bin/sh
# Then: sh tests/e2e/manual/run-all.sh

PASS=0
FAIL=0

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

echo "=== sightjack E2E Manual Tests ==="
echo ""

# --- 1. Version ---
echo "--- version ---"
if sightjack version | grep -q "sightjack v"; then
    pass "version"
else
    fail "version"
fi

if sightjack version --json | python3 -c "import sys,json; json.load(sys.stdin)" 2>/dev/null || \
   sightjack version --json | grep -q '"version"'; then
    pass "version --json"
else
    fail "version --json"
fi

# --- 2. Doctor ---
echo "--- doctor ---"
DOCTOR_OUT=$(sightjack doctor 2>&1)
if [ -n "$DOCTOR_OUT" ]; then
    pass "doctor"
else
    fail "doctor"
fi

# --- 3. Help ---
echo "--- help ---"
HELP_OUT=$(sightjack --help 2>&1)
if echo "$HELP_OUT" | grep -q "scan"; then
    pass "--help"
else
    fail "--help"
fi

# --- 4. Unknown command ---
echo "--- unknown command ---"
if ! sightjack nonexistent 2>/dev/null; then
    pass "unknown command exits non-zero"
else
    fail "unknown command should fail"
fi

# --- 5. Init ---
echo "--- init ---"
WORKDIR=$(mktemp -d)
printf "TestTeam\nTestProject\n\n\n" | sightjack init "$WORKDIR" 2>/dev/null
if [ -f "$WORKDIR/.siren/config.yaml" ]; then
    pass "init creates config"
else
    fail "init missing config"
fi

# --- 6. Scan --json ---
echo "--- scan --json ---"
SCANDIR=$(mktemp -d)
mkdir -p "$SCANDIR/.siren"
cat > "$SCANDIR/.siren/config.yaml" <<'YAML'
lang: en
claude:
  command: claude
  timeout_sec: 30
scan:
  max_concurrency: 1
  chunk_size: 50
linear:
  team: ENG
  project: TestProject
strictness:
  default: fog
retry:
  max_attempts: 1
  base_delay_sec: 0
labels:
  enabled: false
scribe:
  enabled: false
YAML

SCAN_OUT=$(sightjack scan --json "$SCANDIR" 2>/dev/null)
if echo "$SCAN_OUT" | grep -q '"clusters"'; then
    pass "scan --json"
else
    fail "scan --json"
fi

# --- 7. Pipe: waves ---
echo "--- pipe: waves ---"
WAVE_OUT=$(echo "$SCAN_OUT" | sightjack waves "$SCANDIR" 2>/dev/null)
if echo "$WAVE_OUT" | grep -q '"waves"'; then
    pass "pipe: scan | waves"
else
    fail "pipe: scan | waves"
fi

# --- 8. Apply --dry-run ---
echo "--- apply --dry-run ---"
WAVE_JSON='{"id":"test-w1","cluster_name":"Test","title":"Test Wave","description":"test","status":"available","actions":[{"type":"add_dod","issue_id":"T-1","description":"test","detail":""}],"prerequisites":[],"delta":{"before":0.3,"after":0.7}}'
APPLY_OUT=$(echo "$WAVE_JSON" | sightjack apply --dry-run "$SCANDIR" 2>&1)
if echo "$APPLY_OUT" | grep -qi "dry-run"; then
    pass "apply --dry-run"
else
    fail "apply --dry-run"
fi

# --- 9. Show (no state) ---
echo "--- show (no state) ---"
EMPTYDIR=$(mktemp -d)
if ! sightjack show "$EMPTYDIR" 2>/dev/null; then
    pass "show with no state fails"
else
    fail "show should fail without state"
fi

# --- 10. Show (after scan) ---
echo "--- show (after scan) ---"
if sightjack show "$SCANDIR" 2>/dev/null | grep -q .; then
    pass "show after scan"
else
    fail "show after scan"
fi

# --- 11. Archive prune ---
echo "--- archive-prune ---"
if sightjack archive-prune "$EMPTYDIR" 2>&1 | grep -qi "no expired\|threshold"; then
    pass "archive-prune empty"
else
    fail "archive-prune empty"
fi

# --- 12. Run --dry-run ---
echo "--- run --dry-run ---"
DRYDIR=$(mktemp -d)
mkdir -p "$DRYDIR/.siren"
cp "$SCANDIR/.siren/config.yaml" "$DRYDIR/.siren/config.yaml"
DRYRUN_OUT=$(sightjack run --dry-run "$DRYDIR" 2>&1)
if echo "$DRYRUN_OUT" | grep -qi "dry-run\|prompt"; then
    pass "run --dry-run"
else
    fail "run --dry-run"
fi

# --- Summary ---
echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
echo "ALL MANUAL TESTS PASSED"
