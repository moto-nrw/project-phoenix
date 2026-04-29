#!/usr/bin/env bash
# scripts/check-bloat.sh
#
# Architectural-debt ratchet. Compares current violation counts against
# .claude/rules/baseline.txt. Fails if any count rose. Detection commands
# stay in sync with .claude/rules/backend-conventions.md (see Rule 13).
#
# Usage:
#   scripts/check-bloat.sh                                      # check vs baseline
#   scripts/check-bloat.sh --print                              # print current counts
#   scripts/check-bloat.sh --print > .claude/rules/baseline.txt # update baseline

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BASELINE_FILE=".claude/rules/baseline.txt"

for tool in rg gocognit; do
    command -v "$tool" >/dev/null || {
        echo "ERROR: $tool not installed (rg = ripgrep; gocognit = github.com/uudashr/gocognit)" >&2
        exit 2
    }
done

# --- detection commands (one per ratchet key) ---

count_layer_skip_in_handlers() {
    (rg --type go -g '!*_test.go' \
        '^\s+\w+\s+[\w\.]*Repository\b|\.\w+Repository\(\)' \
        backend/api/ 2>/dev/null || true) | wc -l | tr -d ' '
}

count_fat_handlers() {
    (gocognit -over 15 backend/api/ 2>/dev/null || true) | wc -l | tr -d ' '
}

count_test_handler_wrappers() {
    (rg -U --type go -g '!*_test.go' \
        '^func \([^)]+\) \w+Handler\(\) http\.HandlerFunc \{[^}]*return [^}]+\.\w+\s*\}' \
        backend/api/ 2>/dev/null || true) | grep -c 'Handler() http.HandlerFunc' || true
}

count_duplicate_error_helpers() {
    (rg --type go \
        'func ErrorInvalidRequest|func ErrorNotFound|func ErrorForbidden|func ErrorInternalServer|func ErrorUnauthorized|func ErrorConflict' \
        backend/api/ 2>/dev/null || true) | grep -v '/common/' | wc -l | tr -d ' '
}

count_service_query_construction() {
    (rg --type go -g '!*_test.go' \
        '\.(NewSelect|NewUpdate|NewInsert|NewDelete|NewRaw)\(' \
        backend/services/ 2>/dev/null || true) | wc -l | tr -d ' '
}

count_model_state_mutations() {
    (rg --type go -g '!*_test.go' \
        '^func \([^)]+\) (Mark|End|Close|Reset|Activate|Deactivate|Cancel|Approve|Reject|Promote|Demote|Suspend|Restore|Archive)\w*\(' \
        backend/models/ 2>/dev/null || true) | wc -l | tr -d ' '
}

count_model_rbac_decisions() {
    (rg --type go -g '!*_test.go' \
        '^func \([^)]+\) (HasPermission|IsAdmin|IsOperator|CanAccess|CanEdit|CanDelete)\w*\(' \
        backend/models/ 2>/dev/null || true) | wc -l | tr -d ' '
}

count_model_magic_numbers() {
    (rg --type go -g '!*_test.go' \
        '\d+\s*\*\s*time\.(Hour|Minute|Second|Day)' \
        backend/models/ 2>/dev/null || true) | wc -l | tr -d ' '
}

KEYS=(
    layer_skip_in_handlers
    fat_handlers
    test_handler_wrappers
    duplicate_error_helpers
    service_query_construction
    model_state_mutations
    model_rbac_decisions
    model_magic_numbers
)

# --- --print mode: emit current counts ---

if [[ "${1:-}" == "--print" ]]; then
    echo "# Architectural-debt baseline. Counts cannot increase."
    echo "# Detection commands live in .claude/rules/backend-conventions.md (Rule 13)."
    echo "# Generated $(date -u +%Y-%m-%d) by scripts/check-bloat.sh --print"
    echo ""
    for key in "${KEYS[@]}"; do
        echo "${key}=$("count_${key}")"
    done
    exit 0
fi

# --- check mode: compare to baseline ---

[[ -f "$BASELINE_FILE" ]] || {
    echo "ERROR: $BASELINE_FILE not found." >&2
    echo "Generate one with: scripts/check-bloat.sh --print > $BASELINE_FILE" >&2
    exit 2
}

declare -A BASELINE
while IFS='=' read -r key value; do
    [[ -z "$key" || "$key" =~ ^# ]] && continue
    BASELINE["$key"]="$value"
done < "$BASELINE_FILE"

EXIT=0
printf "%-32s %10s %10s %10s\n" "RULE" "BASELINE" "CURRENT" "DELTA"
echo "--------------------------------------------------------------------"

for key in "${KEYS[@]}"; do
    base="${BASELINE[$key]:-0}"
    curr="$("count_${key}")"
    delta=$((curr - base))

    if [[ "$curr" -gt "$base" ]]; then
        printf "%-32s %10s %10s %10s  FAIL\n" "$key" "$base" "$curr" "+$delta"
        EXIT=1
    elif [[ "$curr" -lt "$base" ]]; then
        printf "%-32s %10s %10s %10s  improved\n" "$key" "$base" "$curr" "$delta"
    else
        printf "%-32s %10s %10s %10s\n" "$key" "$base" "$curr" "$delta"
    fi
done

echo ""
if [[ "$EXIT" -ne 0 ]]; then
    cat <<'EOF'
FAIL: at least one architectural-debt count increased above baseline.

Options:
  1. Fix the violation in your PR (preferred).
  2. If the increase is unavoidable, regenerate the baseline and explain why
     in the PR description:
       scripts/check-bloat.sh --print > .claude/rules/baseline.txt

When you remove violations, run --print and commit the new (lower) baseline
so the win is locked in. Otherwise the next PR can re-introduce them up to
the old baseline.
EOF
    exit 1
fi

echo "PASS — all counts at or below baseline."
