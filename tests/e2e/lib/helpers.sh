#!/bin/bash
# ============================================================================
# mockd E2E Test Helpers
# ============================================================================
# Shared functions for all test suites. Sourced by entrypoint.sh.
#
# Provides:
#   api()                  - Call admin API, sets $STATUS and $BODY
#   engine()               - Call mock engine, sets $STATUS and $BODY
#   cli()                  - Run mockd CLI, sets $CLI_EXIT and $CLI_OUT
#   cli_ws()               - Run mockd CLI for workspace commands
#   assert_status()        - Assert HTTP status code
#   assert_json_field()    - Assert JSON field equals expected
#   assert_json_field_gt() - Assert JSON field > threshold
#   assert_body_contains() - Assert body contains string
#   assert_cli_exit()      - Assert CLI exit code
#   assert_cli_contains()  - Assert CLI output contains string
#   assert_cli_not_contains() - Assert CLI output does not contain string
#   assert_cli_json_field() - Assert CLI JSON output field
#   pass() / fail() / skip() - Test result tracking
#   suite_header()         - Print suite name
#   should_run()           - Check if suite should run based on TEST_SUITES

ADMIN="${MOCKD_ADMIN_URL:?MOCKD_ADMIN_URL is required}"
ENGINE="${MOCKD_ENGINE_URL:?MOCKD_ENGINE_URL is required}"
SUITES="${TEST_SUITES:-all}"

PASS=0
FAIL=0
SKIP=0
ERRORS=()

# ─── Logging & Results ────────────────────────────────────────────────────────

log()  { echo "$(date +%H:%M:%S) $*"; }
pass() { PASS=$((PASS + 1)); log "  ✓ $1"; }
fail() { FAIL=$((FAIL + 1)); ERRORS+=("$1: $2"); log "  ✗ $1 — $2"; }
skip() { SKIP=$((SKIP + 1)); log "  ⊘ $1 (skipped)"; }
suite_header() { echo ""; log "━━━ $1 ━━━"; }

# ─── API Helpers ──────────────────────────────────────────────────────────────

# Make an API call and capture status + body
# Usage: api GET /health => sets $STATUS and $BODY
api() {
  local method="$1" path="$2"
  shift 2
  local url="${ADMIN}${path}"
  local resp
  resp=$(curl -s -w '\n%{http_code}' -X "$method" "$url" \
    -H 'Content-Type: application/json' "$@" 2>&1) || true
  BODY=$(echo "$resp" | sed '$d')
  STATUS=$(echo "$resp" | tail -n 1)
}

# Hit the mock engine (not admin)
engine() {
  local method="$1" path="$2"
  shift 2
  local url="${ENGINE}${path}"
  local resp
  resp=$(curl -s -w '\n%{http_code}' -X "$method" "$url" \
    -H 'Content-Type: application/json' "$@" 2>&1) || true
  BODY=$(echo "$resp" | sed '$d')
  STATUS=$(echo "$resp" | tail -n 1)
}

# ─── Assertions ───────────────────────────────────────────────────────────────

assert_status() {
  local expected="$1" test_id="$2"
  if [[ "$STATUS" == "$expected" ]]; then
    pass "$test_id"
  else
    fail "$test_id" "expected HTTP $expected, got $STATUS (body: $(echo "$BODY" | head -c 200))"
  fi
}

assert_json_field() {
  local field="$1" expected="$2" test_id="$3"
  local actual
  actual=$(echo "$BODY" | jq -r "$field" 2>/dev/null) || actual="(jq parse error)"
  if [[ "$actual" == "$expected" ]]; then
    pass "$test_id"
  else
    fail "$test_id" "expected $field=$expected, got $actual"
  fi
}

assert_json_field_gt() {
  local field="$1" threshold="$2" test_id="$3"
  local actual
  actual=$(echo "$BODY" | jq -r "$field" 2>/dev/null) || actual="0"
  if [[ "$actual" -gt "$threshold" ]] 2>/dev/null; then
    pass "$test_id"
  else
    fail "$test_id" "expected $field > $threshold, got $actual"
  fi
}

assert_body_contains() {
  local needle="$1" test_id="$2"
  if echo "$BODY" | grep -q "$needle"; then
    pass "$test_id"
  else
    fail "$test_id" "body does not contain '$needle'"
  fi
}

# ─── CLI Helpers ──────────────────────────────────────────────────────────────

MOCKD_CLI="mockd"
CLI_EXIT=0
CLI_OUT=""

# Run a mockd CLI command against the remote admin server.
# --admin-url is appended at the END so cobra parses subcommands first.
# Usage: cli add --path /test --body '{"ok":true}' => sets $CLI_EXIT, $CLI_OUT
cli() {
  CLI_OUT=$($MOCKD_CLI "$@" --admin-url "$ADMIN" 2>&1) && CLI_EXIT=0 || CLI_EXIT=$?
}

# Same but for workspace commands that use -u instead of --admin-url
# For workspace delete: the ID is positional and must come right after the subcommand.
# We inject -u ADMIN after the first two args (workspace <subcmd>), before everything else.
cli_ws() {
  local cmd="$1" subcmd="$2"
  shift 2
  CLI_OUT=$($MOCKD_CLI "$cmd" "$subcmd" -u "$ADMIN" "$@" 2>&1) && CLI_EXIT=0 || CLI_EXIT=$?
}

assert_cli_exit() {
  local expected="$1" test_id="$2"
  if [[ "$CLI_EXIT" == "$expected" ]]; then
    pass "$test_id"
  else
    fail "$test_id" "expected exit $expected, got $CLI_EXIT (output: $(echo "$CLI_OUT" | head -c 300))"
  fi
}

assert_cli_contains() {
  local needle="$1" test_id="$2"
  if echo "$CLI_OUT" | grep -q "$needle"; then
    pass "$test_id"
  else
    fail "$test_id" "CLI output does not contain '$needle' (output: $(echo "$CLI_OUT" | head -c 300))"
  fi
}

assert_cli_not_contains() {
  local needle="$1" test_id="$2"
  if echo "$CLI_OUT" | grep -q "$needle"; then
    fail "$test_id" "CLI output unexpectedly contains '$needle'"
  else
    pass "$test_id"
  fi
}

assert_cli_json_field() {
  local field="$1" expected="$2" test_id="$3"
  local actual
  actual=$(echo "$CLI_OUT" | jq -r "$field" 2>/dev/null) || actual="(jq parse error)"
  if [[ "$actual" == "$expected" ]]; then
    pass "$test_id"
  else
    fail "$test_id" "expected $field=$expected, got $actual"
  fi
}

# ─── Suite Control ────────────────────────────────────────────────────────────

should_run() {
  [[ "$SUITES" == "all" ]] && return 0
  local suite="$1"
  echo "$SUITES" | tr ',' '\n' | grep -qw "$suite"
}
