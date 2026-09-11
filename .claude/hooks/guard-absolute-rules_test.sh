#!/usr/bin/env bash
# Tests for guard-absolute-rules.sh: table-driven allow/deny checks against a
# hermetic fixture repository (incl. a linked worktree), so nothing depends on
# the real repo's tracked-file state. Asserts BOTH channels of a deny: exit
# code 2 (Codex) and the permissionDecision JSON on stdout (Claude Code).
# Nothing in the table is ever executed - payloads are only piped into the
# hook - so allow cases may name programs that do not exist on this machine.
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
git -C "$repo" config commit.gpgsign false
printf '#!/usr/bin/env bash\necho toolchain "$@"\n' >"$repo/scripts/run-go-toolchain.sh"
printf '#!/usr/bin/env bash\necho backend tests\n' >"$repo/scripts/test-backend.sh"
printf '#!/usr/bin/env bash\necho env check\n' >"$repo/scripts/env-check.sh"
printf 'package backend\n' >"$repo/backend/doc.go"
chmod +x "$repo"/scripts/*.sh
# tracked symlink escaping the repo: must not launder an outside file
printf '#!/usr/bin/env bash\necho outside\n' >"$fixture/outside.sh"
ln -s "$fixture/outside.sh" "$repo/scripts/outside.sh"
git -C "$repo" add -A
git -C "$repo" commit -qm fixture
# untracked on purpose: exists, executable, but not vetted
printf '#!/usr/bin/env bash\necho new\n' >"$repo/scripts/new.sh"
printf '#!/usr/bin/env bash\necho new\n' >"$repo/scripts/new"
printf '#!/usr/bin/env bash\necho untracked launcher\n' >"$repo/env"
chmod +x "$repo/scripts/new.sh" "$repo/scripts/new" "$repo/env"
# gitignored devbox tool farm: entries resolve into the nix store (outside
# the repo) and are exempt from tracking; a broken symlink cycle is not.
mkdir -p "$repo/.devbox/nix/profile/default/bin"
ln -s /usr/bin/true "$repo/.devbox/nix/profile/default/bin/go"
ln -s cycle-b "$repo/.devbox/nix/profile/default/bin/cycle-a"
ln -s cycle-a "$repo/.devbox/nix/profile/default/bin/cycle-b"
outside_dir="$fixture/outside-dir"
mkdir -p "$outside_dir/scripts"
printf '#!/usr/bin/env bash\necho outside\n' >"$outside_dir/scripts/test-backend.sh"
chmod +x "$outside_dir/scripts/test-backend.sh"

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
assert_bash allow "$repo" '(cd backend && ../scripts/run-go-toolchain.sh go test ./...)'
assert_bash allow "$repo" '(cd backend && ../scripts/test-backend.sh); scripts/test-backend.sh'
assert_bash allow "$repo" 'scripts/test-backend.sh -run TestFoo ./...'
assert_bash allow "$repo" './scripts/env-check.sh'
assert_bash allow "$repo" 'bash scripts/env-check.sh'
assert_bash allow "$repo" 'scripts/run-go-toolchain.sh scripts/test-backend.sh'
assert_bash allow "$repo" 'scripts/run-go-toolchain.sh golangci-lint run'
assert_bash allow "$repo" 'scripts/run-go-toolchain.sh govulncheck ./...'
assert_bash allow "$repo" 'scripts/run-go-toolchain.sh gotestsum --format pkgname -- -count=1 ./...'
assert_bash allow "$repo/backend" '../scripts/test-backend.sh'
assert_bash allow "$repo" 'PHX_TEST_RUN_ID=abc scripts/test-backend.sh'
assert_bash allow "$repo" 'source scripts/env-check.sh'
assert_bash allow "$repo" 'builtin cd backend && ../scripts/test-backend.sh'
assert_bash allow "$repo" '/bin/bash scripts/env-check.sh'
assert_bash allow "$worktree" 'scripts/test-backend.sh'
assert_bash allow "$worktree" 'cd backend && ../scripts/run-go-toolchain.sh go vet ./...'

# --- everyday commands: never vetted, never blocked ---
assert_bash allow "$repo" 'node --version'
assert_bash allow "$repo" 'python3 -m venv .venv'
assert_bash allow "$repo" 'git commit -m "fix: x"'
assert_bash allow "$repo" "rg $prod_host docs/"
assert_bash allow "$repo" "grep -R 'test-changed.sh' scripts/"
assert_bash allow "$repo" "printf '%s\\n' 'a; scripts/new.sh'"
assert_bash allow "$repo" 'printf "%s" "text; ./scripts/new.sh"'
assert_bash allow "$repo" '/usr/bin/git --version'
assert_bash allow "$repo" '/usr/bin/env'
assert_bash allow "$repo" '/usr/bin/env FOO=1 /usr/bin/true'
assert_bash allow "$repo" 'if true; then :; fi'
assert_bash allow "$repo" 'export PHX_GUARD_TEST=1; printf "%s" ok'
assert_bash allow "$repo" 'cd backend | scripts/test-backend.sh'
# regression guards for the over-hardened states this guard once had:
assert_bash allow "$repo" 'ls -la /tmp/whatever 2>&1 | head -1'
assert_bash allow "$repo" 'gh pr view 2937 --json state'
assert_bash allow "$repo" 'definitely-not-installed-guard-probe --version'
assert_bash allow "$repo" 'PATH=/opt/somewhere/bin go version'
assert_bash allow "$repo" 'CC=/usr/bin/cc go build ./...'
assert_bash allow "$repo" 'go test -tags=integration -count=1 ./...'
assert_bash allow "$repo" 'go tool pprof -top cpu.out'
assert_bash allow "$repo" 'go generate ./...'
assert_bash allow "$repo" 'command -v gh'
assert_bash allow "$repo" 'stdbuf -o0 cat somefile'
assert_bash allow "$repo" 'printf x | node'
assert_bash allow "$repo" "find . -name '*.tmp' -exec rm {} \\;"
assert_bash allow "$repo" 'xargs rm'
assert_bash allow "$repo" "$repo/.devbox/nix/profile/default/bin/go version"

# --- unvetted execution: denied ---
assert_bash deny "$repo" './scripts/new.sh'
assert_bash deny "$repo" 'scripts/new.sh'
assert_bash deny "$repo" './scripts/new'
assert_bash deny "$repo" './scripts/does-not-exist.sh'
assert_bash deny "$repo" 'bash /tmp/definitely-missing-guard-test.sh'
assert_bash deny "$repo" 'scripts/run-go-toolchain.sh /tmp/evil.sh'
assert_bash deny "$repo" "scripts/run-go-toolchain.sh node -e 'echo evil'"
assert_bash deny "$repo" 'scripts/run-go-toolchain.sh pnpm exec evil'
assert_bash deny "$repo" 'scripts/run-go-toolchain.sh go test -exec ./scripts/new ./...'
assert_bash deny "$repo" 'GOFLAGS=-exec=/tmp/evil go test ./...'
assert_bash deny "$repo" '(cd backend && :); ../scripts/new'
assert_bash deny "$repo" "scripts/run-go-toolchain.sh \"\$CMD\""
assert_bash deny "$repo" 'scripts/run-go-toolchain.sh <(printf x)'
assert_bash deny "$repo" 'bash scripts/run-go-toolchain.sh /tmp/evil.sh'
assert_bash deny "$repo" "sh scripts/run-go-toolchain.sh \"\$CMD\""
assert_bash deny "$repo" 'env bash scripts/run-go-toolchain.sh /tmp/evil.sh'
assert_bash deny "$repo" 'source scripts/run-go-toolchain.sh /tmp/evil.sh'
assert_bash deny "$repo" "\"\$CMD\""
assert_bash deny "$repo" "BASH_ENV=\$CMD bash scripts/env-check.sh"
assert_bash deny "$repo" "PATH=\$CMD bash scripts/env-check.sh"
assert_bash deny "$repo" "env BASH_ENV=\$CMD bash scripts/env-check.sh"
assert_bash deny "$repo" 'LD_PRELOAD=/tmp/evil /usr/bin/true'
assert_bash deny "$repo" 'LD_LIBRARY_PATH=/tmp /usr/bin/true'
assert_bash deny "$repo" 'DYLD_INSERT_LIBRARIES=/tmp/evil /usr/bin/true'
assert_bash deny "$repo" 'cd "$CMD"; scripts/test-backend.sh'
assert_bash deny "$repo" 'cd "$(pwd)"; scripts/test-backend.sh'
assert_bash deny "$fixture" 'repo/scripts/test-backend.sh' # outside any repo root
assert_bash deny "$repo" 'source /tmp/some-env-file'
assert_bash deny "$repo" 'scripts/outside.sh'
assert_bash deny "$repo" './env /usr/bin/true'
assert_bash deny "$repo" "$repo/.devbox/nix/profile/default/bin/cycle-a"
assert_bash deny "$repo" $'cd backend\n../scripts/new.sh'
assert_bash deny "$repo" 'nice -n 5 ./scripts/new'
assert_bash deny "$repo" 'python3 ./scripts/new'
assert_bash deny "$repo" 'xargs ./scripts/new'
assert_bash deny "$repo" 'if true; then ./scripts/new; fi'
assert_bash deny "$repo" 'foo() { ./scripts/new; }; foo'
assert_bash deny "$repo" 'foo(){ ./scripts/new; }; foo'
assert_bash deny "$repo" 'foo (){ ./scripts/new; }; foo'
assert_bash deny "$repo" 'cat <(./scripts/new)'
assert_bash deny "$repo" "env --chdir=$outside_dir scripts/test-backend.sh"
assert_bash deny "$repo" "env -C$outside_dir scripts/test-backend.sh"
assert_bash deny "$repo" 'find . -exec ./scripts/new.sh \;'
assert_bash deny "$repo" 'find . -execdir ./scripts/new.sh \;'
assert_bash deny "$repo" 'cd backend | ./scripts/new'
assert_bash deny "$repo" 'case x in x) ./scripts/new.sh;; esac'
assert_bash deny "$repo" 'stdbuf -o0 ./scripts/new'
assert_bash deny "$repo" "builtin eval \"\$(cat somefile)\""
assert_bash deny "$repo" 'builtin source /tmp/some-env-file'
assert_bash deny "$repo" 'builtin . /tmp/some-env-file'
assert_bash deny "$repo" "trap './scripts/new' EXIT"
assert_bash deny "$repo" "builtin trap './scripts/new' EXIT"
assert_bash deny "$repo" 'pushd /tmp; ./scripts/test-backend.sh'

# --- inline payloads and eval: denied, incl. the -lc flag cluster ---
assert_bash deny "$repo" "bash -c 'echo hi'"
assert_bash deny "$repo" "bash -lc 'echo hi'"
assert_bash deny "$repo" "sh -c 'echo hi'"
assert_bash deny "$repo" "env bash -c 'echo hi'"
assert_bash deny "$repo" "command sh -c 'echo hi'"
assert_bash deny "$repo" "timeout 10 bash -c 'echo hi'"
assert_bash deny "$repo" "sudo bash -c 'echo hi'"
assert_bash deny "$repo" "python3 -c 'print(1)'"
assert_bash deny "$repo" "node -e 'console.log(1)'"
assert_bash deny "$repo" "eval \"\$(cat somefile)\""
assert_bash deny "$repo" "echo \"\$(./scripts/new.sh)\""
assert_bash deny "$repo" "echo \`./scripts/new.sh\`"

# --- rule 1: production hosts ---
assert_bash deny "$repo" "curl https://$prod_host/health"
assert_bash deny "$repo" "xh https://$prod_host2/api"
assert_bash deny "$repo" $'rg api.moto-app.de docs/\ncurl https://api.moto-app.de/health'

# --- rule 2: sops env files ---
assert_bash deny "$repo" 'echo FOO=1 >> environments/production.sops.env'
assert_bash deny "$repo" $'sops environments/production.sops.env\necho FOO=1 >> environments/production.sops.env'
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
