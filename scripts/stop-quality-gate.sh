#!/usr/bin/env bash
# stop-quality-gate.sh - Stop hook: block the end of a turn while changed Go
# or frontend files fail the cheap quality checks.
#
# Checks, scoped to what actually changed in the working tree vs HEAD
# (untracked files included):
#   backend/**/*.go   -> cd backend  && go build ./... && go vet ./...
#   frontend/**       -> cd frontend && pnpm run check
#
# Usage: stop-quality-gate.sh [--stop-hook]
#
# Protocol: reads the Stop-hook JSON on stdin; honors stop_hook_active to
# avoid loops; blocks through {"decision":"block","reason":…} on stdout AND
# exit code 2 with the reason on stderr (Claude Code and Codex each accept
# one of the two channels). Environment problems (missing go/pnpm/jq, no
# node_modules) never block - the hook must not wedge a session.
#
# Successful runs are cached per working-tree fingerprint in
# .git/quality-gate.cache, so an unchanged dirty tree does not re-run the
# checks at every turn end.
#
# Opt-out: touch .git/quality-gate-off (per clone) or QUALITY_GATE_OFF=1.
#
# Kept bash-3.2 compatible (macOS /bin/bash).

set -euo pipefail

case "${1:-}" in
    "" | --stop-hook) ;;
    -h | --help)
        sed -n '2,23p' "$0" | sed 's/^# \{0,1\}//'
        exit 0
        ;;
    *)
        echo "unknown option: $1" >&2
        exit 64
        ;;
esac

soft_exit() { exit 0; }

command -v jq >/dev/null 2>&1 || soft_exit
project_root=$(git rev-parse --show-toplevel 2>/dev/null) || soft_exit
cd "$project_root"
git_dir=$(git rev-parse --git-dir 2>/dev/null) || soft_exit

# Deliberate opt-outs.
if [[ -n "${QUALITY_GATE_OFF:-}" ]]; then soft_exit; fi
if [[ -f "$git_dir/quality-gate-off" ]]; then soft_exit; fi

# stop_hook_active is true when the agent is already continuing because of a
# Stop hook. Blocking again there loops forever.
hook_input=""
if [[ ! -t 0 ]]; then
    IFS= read -r -d '' -t 1 hook_input || true
fi
if [[ -n "$hook_input" ]]; then
    active=$(printf '%s' "$hook_input" | jq -r '.stop_hook_active // false' 2>/dev/null || echo false)
    if [[ "$active" == "true" ]]; then soft_exit; fi
fi

# Changed files vs HEAD, untracked included: a brand-new file that does not
# compile must block just like an edited one.
status=$(git status --porcelain -- backend frontend 2>/dev/null) || soft_exit
if [[ -z "$status" ]]; then soft_exit; fi

go_changed=0
frontend_changed=0
while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    path=${line:3}
    # Rename lines are "R  old -> new"; the new path decides.
    case "$path" in *" -> "*) path=${path##* -> } ;; esac
    case "$path" in
        backend/*.go) go_changed=1 ;;
        frontend/*) frontend_changed=1 ;;
    esac
done <<EOF
$status
EOF

if [[ "$go_changed" == 0 && "$frontend_changed" == 0 ]]; then soft_exit; fi

# Fingerprint the relevant working-tree state so a clean verdict is reused
# until backend/ or frontend/ actually changes again.
hash_cmd="shasum -a 256"
command -v shasum >/dev/null 2>&1 || hash_cmd="sha256sum"
fingerprint=$(
    {
        printf '%s\n' "$status"
        git diff HEAD -- backend frontend 2>/dev/null
        git ls-files --others --exclude-standard -- backend frontend 2>/dev/null |
            while IFS= read -r f; do
                [[ -f "$f" ]] && git hash-object "$f" 2>/dev/null
            done
    } | $hash_cmd | cut -d' ' -f1
) || soft_exit

cache_file="$git_dir/quality-gate.cache"
if [[ -f "$cache_file" ]] && [[ "$(cat "$cache_file" 2>/dev/null)" == "clean $fingerprint" ]]; then
    soft_exit
fi

# Both Claude Code and Codex accept a Stop hook blocking either through exit
# code 2 with the reason on stderr, or through {"decision":"block","reason":…}
# on stdout. Emitting both makes the block independent of which channel the
# running tool prefers.
emit_block() {
    local reason="$1"
    jq -n --arg reason "$reason" '{decision: "block", reason: $reason}'
    printf '%s\n' "$reason" >&2
    exit 2
}

run_check() {
    local label="$1" dir="$2"
    shift 2
    local out
    if ! out=$(cd "$dir" && "$@" 2>&1); then
        out=$(printf '%s\n' "$out" | tail -n 30)
        emit_block "Quality gate: '$label' failed for changed files under $dir/. Fix the errors below before finishing (deliberate opt-out: touch .git/quality-gate-off).
$out"
    fi
}

checks_complete=1
if [[ "$go_changed" == 1 ]]; then
    if command -v go >/dev/null 2>&1; then
        run_check "go build ./..." backend go build ./...
        run_check "go vet ./..." backend go vet ./...
    else
        checks_complete=0
    fi
fi
if [[ "$frontend_changed" == 1 ]]; then
    if command -v pnpm >/dev/null 2>&1 && [[ -d frontend/node_modules ]]; then
        run_check "pnpm run check" frontend pnpm run check
    else
        checks_complete=0
    fi
fi

if [[ "$checks_complete" == 1 ]]; then
    printf 'clean %s' "$fingerprint" >"$cache_file" 2>/dev/null || true
fi
exit 0
