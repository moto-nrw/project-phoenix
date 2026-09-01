#!/usr/bin/env bash
# Tests for guard-absolute-rules.sh: table-driven allow/deny checks against a
# hermetic fixture repository (incl. a linked worktree), so nothing depends on
# the real repo's tracked-file state. Asserts BOTH channels of a deny: exit
# code 2 (Codex) and the permissionDecision JSON on stdout (Claude Code).
#
# Production hostnames below are assembled by concatenation on purpose, so
# content scans over this file never see them as literals.
set -euo pipefail

command -v jq >/dev/null 2>&1 || {
    echo "jq is required" >&2
    exit 1
}

hook_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
hook="$hook_dir/guard-absolute-rules.sh"

fixture=$(mktemp -d "${TMPDIR:-/tmp}/guard-test.XXXXXX")
cleanup() { rm -rf "$fixture"; }
trap cleanup EXIT

prod_host="api.moto-app"".de"
prod_host2="portal.moto"".nrw"
rls_sql="DISABLE ROW"" LEVEL SECURITY"

repo="$fixture/repo"
mkdir -p "$repo/scripts" "$repo/backend" "$repo/environments"
git -C "$fixture" init -q -b main repo
git -C "$repo" config user.email guard-test@example.invalid
git -C "$repo" config user.name "Guard Test"
printf '#!/usr/bin/env bash\necho toolchain "$@"\n' >"$repo/scripts/run-go-toolchain.sh"
printf '#!/usr/bin/env bash\necho backend tests\n' >"$repo/scripts/test-backend.sh"
printf '#!/usr/bin/env bash\necho env check\n' >"$repo/scripts/env-check.sh"
printf 'package backend\n' >"$repo/backend/doc.go"
chmod +x "$repo"/scripts/*.sh
git -C "$repo" add -A
git -C "$repo" commit -qm fixture
# untracked on purpose: exists, executable, but not vetted
printf '#!/usr/bin/env bash\necho new\n' >"$repo/scripts/new.sh"
chmod +x "$repo/scripts/new.sh"

worktree="$fixture/wt"
git -C "$repo" worktree add -q "$worktree" -b guard-test-wt

failures=0
checks=0

# run_hook <allow|deny> <label> <payload-json>
run_hook() {
    local expect=$1 label=$2 payload=$3 out status decision
    checks=$((checks + 1))
    set +e
    out=$(printf '%s' "$payload" | bash "$hook" 2>/dev/null)
    status=$?
    set -e
    decision=$(printf '%s' "$out" | jq -r '.hookSpecificOutput.permissionDecision // empty' 2>/dev/null) || decision=""
    case "$expect" in
        allow)
            if [[ $status -ne 0 || -n "$out" ]]; then
                echo "FAIL (want allow): $label -> exit=$status out=$out" >&2
                failures=$((failures + 1))
            fi
            ;;
        deny)
            if [[ $status -ne 2 || "$decision" != deny ]]; then
                echo "FAIL (want deny): $label -> exit=$status decision=$decision" >&2
                failures=$((failures + 1))
            fi
            ;;
    esac
}

# assert_bash <allow|deny> <cwd> <command>
assert_bash() {
    local expect=$1 cwd=$2 cmd=$3
    run_hook "$expect" "Bash: $cmd (cwd=$cwd)" "$(jq -n --arg cmd "$cmd" --arg cwd "$cwd" \
        '{tool_name: "Bash", tool_input: {command: $cmd}, cwd: $cwd}')"
}

# --- tracked scripts inside the repo: allowed, in every invocation form ---
assert_bash allow "$repo" 'cd backend && ../scripts/run-go-toolchain.sh go test ./...'
assert_bash allow "$repo" 'scripts/test-backend.sh -run TestFoo ./...'
assert_bash allow "$repo" './scripts/env-check.sh'
assert_bash allow "$repo" 'bash scripts/env-check.sh'
assert_bash allow "$repo" 'scripts/run-go-toolchain.sh scripts/test-backend.sh'
assert_bash allow "$repo/backend" '../scripts/test-backend.sh'
assert_bash allow "$repo" 'PHX_TEST_RUN_ID=abc scripts/test-backend.sh'
assert_bash allow "$repo" 'source scripts/env-check.sh'
assert_bash allow "$worktree" 'scripts/test-backend.sh'
assert_bash allow "$worktree" 'cd backend && ../scripts/run-go-toolchain.sh go vet ./...'

# --- interpreters and plain commands: no categorical block ---
assert_bash allow "$repo" 'node --version'
assert_bash allow "$repo" 'python3 -m venv .venv'
assert_bash allow "$repo" 'git commit -m "fix: x"'
assert_bash allow "$repo" "rg $prod_host docs/"

# --- unvetted execution: denied ---
assert_bash deny "$repo" './scripts/new.sh'
assert_bash deny "$repo" 'scripts/new.sh'
assert_bash deny "$repo" './scripts/does-not-exist.sh'
assert_bash deny "$repo" 'bash /tmp/definitely-missing-guard-test.sh'
assert_bash deny "$repo" 'scripts/run-go-toolchain.sh /tmp/evil.sh'
assert_bash deny "$fixture" 'repo/scripts/test-backend.sh' # outside any repo root
assert_bash deny "$repo" 'source /tmp/some-env-file'

# --- inline payloads and eval: denied, incl. the -lc flag cluster ---
assert_bash deny "$repo" "bash -c 'echo hi'"
assert_bash deny "$repo" "bash -lc 'echo hi'"
assert_bash deny "$repo" "sh -c 'echo hi'"
assert_bash deny "$repo" 'eval "$(cat somefile)"'

# --- rule 1: production hosts ---
assert_bash deny "$repo" "curl https://$prod_host/health"
assert_bash deny "$repo" "xh https://$prod_host2/api"

# --- rule 2: sops env files ---
assert_bash deny "$repo" 'echo FOO=1 >> environments/production.sops.env'
run_hook deny "Edit: production.sops.env" "$(jq -n --arg f "$repo/environments/production.sops.env" \
    '{tool_name: "Edit", tool_input: {file_path: $f, old_string: "a", new_string: "b"}}')"

# --- rule 3: RLS in migrations ---
run_hook deny "Write: RLS in migration" "$(jq -n --arg f "$repo/backend/database/migrations/001_x.go" --arg c "ALTER TABLE x $rls_sql" \
    '{tool_name: "Write", tool_input: {file_path: $f, content: $c}}')"

# --- rule 4: --no-verify ---
assert_bash deny "$repo" 'git commit --no-verify -m x'
assert_bash deny "$repo" 'git commit -n -m x'

# --- WebFetch: production hosts ---
run_hook deny "WebFetch: prod host" "$(jq -n --arg u "https://$prod_host2/login" \
    '{tool_name: "WebFetch", tool_input: {url: $u}}')"
run_hook allow "WebFetch: localhost" "$(jq -n \
    '{tool_name: "WebFetch", tool_input: {url: "http://localhost:3000"}}')"

if [[ $failures -gt 0 ]]; then
    echo "$failures of $checks guard checks failed" >&2
    exit 1
fi
echo "all $checks guard checks passed"
