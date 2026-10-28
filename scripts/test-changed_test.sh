#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
fixture=$(mktemp -d)
cleanup() {
  if [ -n "$fixture" ] && [ "$fixture" != / ] && [ -d "$fixture/.git" ]; then
    rm -rf -- "$fixture"
    if [ -e "$fixture" ]; then
      echo "Fixture cleanup failed: $fixture still exists" >&2
      return 1
    fi
  else
    echo "Refusing to delete unexpected fixture path: $fixture" >&2
    return 1
  fi
}
trap cleanup EXIT

mkdir -p "$fixture"/{backend/probe,fake-bin,scripts,frontend}
for script in test-backend.sh test-changed.sh test-frontend-changed.mjs test-run-id.sh go-package-workers.sh go-test-workspace.sh; do
  cp "$repo_root/scripts/$script" "$fixture/scripts/$script"
done
cat >"$fixture/scripts/total-memory-gb.sh" <<'EOF'
#!/usr/bin/env bash
echo "${PHX_FIXTURE_RAM_GB:-64}"
EOF
cat >"$fixture/scripts/backend-affected-packages.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [ "${PHX_FIXTURE_NO_BACKEND:-0}" -eq 0 ]; then echo './probe'; fi
EOF
cat >"$fixture/fake-bin/pnpm" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf 'pnpm:%s\n' "$*" >>"$TEST_CHANGED_CALL_LOG"
case "$1" in
  install)
    [ "$CI" = true ]
    exit "${PHX_FIXTURE_INSTALL_STATUS:-0}"
    ;;
  exec)
    if [ "$3" = list ]; then
      printf '[{"file":"%s/src/listed.test.ts"},{"file":"%s/src/app/api/proxy-routes.test.ts"}]\n' "$PWD" "$PWD"
      exit "${PHX_FIXTURE_CHANGED_STATUS:-0}"
    fi
    [ "$3" = run ]
    echo 'expected error log from a behavior test' >&2
    exit "${PHX_FIXTURE_VITEST_STATUS:-0}"
    ;;
  vitest)
    echo 'expected error log from a behavior test' >&2
    exit "${PHX_FIXTURE_VITEST_STATUS:-0}"
    ;;
  *) exit 99 ;;
esac
EOF
cat >"$fixture/fake-bin/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [ "$*" != 'env GOCACHE' ]; then
  [ -n "${GOTMPDIR:-}" ] && [ -d "$GOTMPDIR" ] || {
    echo 'Go command has no usable build workspace' >&2
    exit 1
  }
  if [ -z "${PHX_FIXTURE_GOTMPDIR:-}" ]; then
    [ "$TMPDIR" = "$GOTMPDIR" ] || { echo 'temporary roots disagree' >&2; exit 1; }
  fi
  printf '%s\n' "$GOTMPDIR" >>"$TEST_CHANGED_WORKSPACE_LOG"
fi
case "$*" in
  'env GOCACHE')
    echo "$PHX_FIXTURE_GO_CACHE"
    ;;
  'run ./internal/testdb/cmd/bootstrap')
    echo bootstrap >>"$TEST_CHANGED_CALL_LOG"
    if [ "${PHX_FIXTURE_BOOTSTRAP_STATUS:-0}" -ne 0 ]; then
      exit "$PHX_FIXTURE_BOOTSTRAP_STATUS"
    fi
    echo phoenix_test_template_regression
    ;;
  'run ./internal/testdb/cmd/sweep')
    echo sweep >>"$TEST_CHANGED_CALL_LOG"
    exit "${PHX_FIXTURE_SWEEP_STATUS:-0}"
    ;;
  'tool gotestsum --help')
    exit "${PHX_FIXTURE_GOTESTSUM_STATUS:-1}"
    ;;
  'tool gotestsum --format pkgname-and-test-fails -- '*)
    echo gotestsum >>"$TEST_CHANGED_CALL_LOG"
    shift 5
    exec "$0" test "$@"
    ;;
  test\ *)
    if [ "${PHX_TEST_TEMPLATE:-}" != phoenix_test_template_regression ]; then
      echo 'go test did not receive the bootstrapped template' >&2
      exit 1
    fi
    echo "test:$PHX_TEST_TEMPLATE:$*" >>"$TEST_CHANGED_CALL_LOG"
    if [ "${PHX_FIXTURE_TEST_STATUS:-0}" -ne 0 ]; then
      echo 'FAIL github.com/moto-nrw/project-phoenix/probe'
      exit "$PHX_FIXTURE_TEST_STATUS"
    fi
    ;;
  *)
    echo "unexpected go command: $*" >&2
    exit 1
    ;;
esac
EOF
cat >"$fixture/fake-bin/getconf" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[ "$1" = _NPROCESSORS_ONLN ]
echo "${PHX_FIXTURE_CPUS:-32}"
EOF
cat >"$fixture/fake-bin/uname" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
[ "$1" = -s ]
echo "${PHX_FIXTURE_OS:-Darwin}"
EOF
chmod +x \
  "$fixture/scripts/test-backend.sh" \
  "$fixture/scripts/test-changed.sh" \
  "$fixture/scripts/test-run-id.sh" \
  "$fixture/scripts/go-package-workers.sh" \
  "$fixture/scripts/total-memory-gb.sh" \
  "$fixture/scripts/backend-affected-packages.sh" \
  "$fixture/fake-bin/getconf" \
  "$fixture/fake-bin/uname" \
  "$fixture/fake-bin/pnpm" \
  "$fixture/fake-bin/go"

printf 'package probe\n' >"$fixture/backend/probe/probe.go"
git -C "$fixture" init -q
git -C "$fixture" config user.email test-changed@example.invalid
git -C "$fixture" config user.name test-changed
git -C "$fixture" config commit.gpgSign false
git -C "$fixture" add .
git -C "$fixture" commit -qm baseline
git -C "$fixture" update-ref refs/remotes/origin/development HEAD
printf '\n// changed\n' >>"$fixture/backend/probe/probe.go"
git -C "$fixture" add backend/probe/probe.go
git -C "$fixture" commit -qm changed

call_log="$fixture/calls.log"
output_log="$fixture/output.log"
workspace_log="$fixture/workspace.log"
run_wrapper() {
  local wrapper=$1 expected_status=$2
  shift 2
  : >"$call_log"
  : >"$workspace_log"
  status=0
  (
    cd "$fixture"
    PATH="$fixture/fake-bin:$PATH" TEST_CHANGED_CALL_LOG="$call_log" \
      PHX_FIXTURE_GO_CACHE="$fixture/go-cache" TEST_CHANGED_WORKSPACE_LOG="$workspace_log" \
      GOTMPDIR="${PHX_FIXTURE_GOTMPDIR:-}" TMPDIR="${PHX_FIXTURE_TMPDIR:-}" \
      "scripts/$wrapper" "$@"
  ) >"$output_log" 2>&1 || status=$?
  if [ "$status" -ne "$expected_status" ]; then
    cat "$output_log" >&2
    echo "$wrapper: expected status $expected_status, got $status" >&2
    exit 1
  fi
  local workspace
  IFS= read -r workspace <"$workspace_log"
  if [ -n "${PHX_FIXTURE_GOTMPDIR:-}" ]; then
    [ "$workspace" = "$PHX_FIXTURE_GOTMPDIR" ] && [ -d "$workspace" ]
  elif [ -n "${PHX_FIXTURE_TMPDIR:-}" ]; then
    [ "$workspace" = "$PHX_FIXTURE_TMPDIR" ] && [ -d "$workspace" ]
  else
    [[ "$workspace" = "$fixture/go-cache/phoenix-tests."* ]] && [ ! -e "$workspace" ] || {
      echo "$wrapper did not create and clean its private build workspace" >&2
      exit 1
    }
  fi
  if [ -e "$fixture/.git/phx-test-run.lock" ]; then
    echo "$wrapper did not release its test-run lock" >&2
    exit 1
  fi
}

assert_calls() {
  local expected=$1 actual
  actual=$(cat "$call_log")
  if [ "$actual" != "$expected" ]; then
    printf 'expected calls:\n%s\nactual calls:\n%s\n' "$expected" "$actual" >&2
    exit 1
  fi
}

# Keep this list explicit: these are the filesystem scan contracts, not
# import dependencies. The fake list returns one guard too: it must run only once.
guards=(
  src/app/api/proxy-routes.test.ts
  src/app/api/route-cache-policy.test.ts
  src/server/auth/route-handler-coverage.test.ts
  src/lib/app-route-invariants.test.ts
  src/lib/analytics-routes.test.ts
  src/lib/help-topics.test.ts
  src/lib/loading-boundaries.test.ts
  src/lib/navigation-sync.test.ts
  src/lib/personal-storage.test.ts
  src/lib/browser-token-ratchet.test.ts
  src/components/ui/tenant-page-scaffold.test.ts
  src/components/ui/plan-design-guards.test.ts
  src/components/settings/settings-filter.test.ts
  src/components/school-setup/setup-tour-targets.test.ts
  src/lib/hooks/use-sse.test.ts
  src/lib/service-worker.test.ts
  src/styles/moto-token-drift.test.ts
  src/server/node-header-limit.test.ts
  src/lib/permission-labels.test.ts
  src/lib/modules.test.ts
  src/lib/role-labels.test.ts
  scripts/oxlint-plugin-bauart.test.ts
  scripts/product-screenshots/shot-list.test.ts
  scripts/product-screenshots/pipeline.test.ts
  scripts/product-screenshots/publish.test.ts
)
# Validate against real source files, not only the mirrored fixture list.
node "$repo_root/scripts/test-frontend-changed.mjs" --check-guards
guard_call="pnpm:exec vitest run ${guards[*]} src/listed.test.ts"
changed_call='pnpm:exec vitest list --filesOnly --json --changed origin/development'

frontend_calls=$'pnpm:install --frozen-lockfile\n'"$changed_call"$'\n'"$guard_call"

# Exercise both public wrappers, not just the arithmetic helper. Low memory,
# low CPU and OS-specific clone-I/O limits must select the same package budget.
while read -r os cpus ram workers; do
  export PHX_FIXTURE_OS=$os PHX_FIXTURE_CPUS=$cpus PHX_FIXTURE_RAM_GB=$ram
  for wrapper in test-backend.sh test-changed.sh; do
    if [ "$wrapper" = test-backend.sh ]; then
      run_wrapper "$wrapper" 0 ./probe
    else
      run_wrapper "$wrapper" 0 origin/development
    fi
    expected=$(printf 'bootstrap\ntest:phoenix_test_template_regression:test -p %s -parallel 8 -tags=testhooks ./probe' "$workers")
    if [ "$wrapper" = test-changed.sh ]; then expected+=$'\n'"$frontend_calls"; fi
    assert_calls "$expected"$'\nsweep'
  done
done <<'EOF'
Darwin 32 8 1
Darwin 32 16 2
Darwin 32 64 2
Linux 32 8 1
Linux 32 16 2
Linux 32 64 4
Darwin 1 64 1
Linux 2 64 1
Linux 6 64 3
Linux 16 4 1
EOF

export PHX_FIXTURE_OS=Darwin PHX_FIXTURE_CPUS=16 PHX_FIXTURE_RAM_GB=16
run_wrapper test-backend.sh 0
assert_calls $'bootstrap\ntest:phoenix_test_template_regression:test -p 2 -parallel 8 -tags=testhooks ./...\nsweep'

# Defaults precede explicit CLI flags so users retain their existing override.
for gotestsum_status in 0 1; do
  export PHX_FIXTURE_GOTESTSUM_STATUS=$gotestsum_status
  run_wrapper test-backend.sh 0 -p 3 ./probe
  expected=$'bootstrap\n'
  if [ "$gotestsum_status" -eq 0 ]; then expected+=$'gotestsum\n'; fi
  expected+=$'test:phoenix_test_template_regression:test -p 2 -parallel 8 -tags=testhooks -p 3 ./probe\nsweep'
  assert_calls "$expected"
done
unset PHX_FIXTURE_GOTESTSUM_STATUS

# Bootstrap/test failures must still sweep and release the run lock. A failed
# sweep fails an otherwise successful run, but never masks a test's exit code.
for wrapper in test-backend.sh test-changed.sh; do
  args=(./probe)
  if [ "$wrapper" = test-changed.sh ]; then args=(origin/development); fi

  export PHX_FIXTURE_BOOTSTRAP_STATUS=5
  run_wrapper "$wrapper" 5 "${args[@]}"
  assert_calls $'bootstrap\nsweep'
  unset PHX_FIXTURE_BOOTSTRAP_STATUS

  export PHX_FIXTURE_TEST_STATUS=7 PHX_FIXTURE_SWEEP_STATUS=9
  run_wrapper "$wrapper" 7 "${args[@]}"
  assert_calls $'bootstrap\ntest:phoenix_test_template_regression:test -p 2 -parallel 8 -tags=testhooks ./probe\nsweep'
  if [ "$wrapper" = test-changed.sh ]; then
    grep -q 'go test failure summary' "$output_log"
    grep -q '^FAIL github.com/moto-nrw/project-phoenix/probe$' "$output_log"
  fi
  unset PHX_FIXTURE_TEST_STATUS PHX_FIXTURE_SWEEP_STATUS

  export PHX_FIXTURE_SWEEP_STATUS=9
  run_wrapper "$wrapper" 1 "${args[@]}"
  expected=$'bootstrap\ntest:phoenix_test_template_regression:test -p 2 -parallel 8 -tags=testhooks ./probe'
  if [ "$wrapper" = test-changed.sh ]; then expected+=$'\n'"$frontend_calls"; fi
  assert_calls "$expected"$'\nsweep'
  unset PHX_FIXTURE_SWEEP_STATUS
done

# Caller-owned workspaces are preserved by both wrappers.
mkdir -p "$fixture/custom-go-tmp"
export PHX_FIXTURE_GOTMPDIR="$fixture/custom-go-tmp"
run_wrapper test-changed.sh 0 origin/development
run_wrapper test-backend.sh 0 ./probe
unset PHX_FIXTURE_GOTMPDIR
export PHX_FIXTURE_TMPDIR="$fixture/custom-go-tmp"
run_wrapper test-changed.sh 0 origin/development
run_wrapper test-backend.sh 0 ./probe
unset PHX_FIXTURE_TMPDIR

# Frontend failures must survive both the frontend-only path (no EXIT trap)
# and the mixed-stack path, whose backend cleanup runs after Vitest.
mkdir -p "$fixture/frontend/src"
for no_backend in 0 1; do
  export PHX_FIXTURE_NO_BACKEND=$no_backend
  for selection in changed full; do
    rm -f "$fixture/frontend/package.json" "$fixture/frontend/src/probe.ts"
    if [ "$selection" = full ]; then
      printf '{}\n' >"$fixture/frontend/package.json"
      vitest_call='pnpm:vitest run'
    else
      printf 'export {};\n' >"$fixture/frontend/src/probe.ts"
      vitest_call="$changed_call"
    fi
    for failure in success install vitest changed; do
      if [ "$selection" = full ] && [ "$failure" = changed ]; then continue; fi
      export PHX_FIXTURE_INSTALL_STATUS=0 PHX_FIXTURE_VITEST_STATUS=0 PHX_FIXTURE_CHANGED_STATUS=0
      expected_status=0
      case "$failure" in
        install) export PHX_FIXTURE_INSTALL_STATUS=6; expected_status=6 ;;
        vitest) export PHX_FIXTURE_VITEST_STATUS=8; expected_status=8 ;;
        changed) export PHX_FIXTURE_CHANGED_STATUS=9; expected_status=9 ;;
      esac
      : >"$call_log"
      status=0
      (cd "$fixture" && PATH="$fixture/fake-bin:$PATH" \
        TEST_CHANGED_CALL_LOG="$call_log" TEST_CHANGED_WORKSPACE_LOG="$workspace_log" \
        PHX_FIXTURE_GO_CACHE="$fixture/go-cache" \
        scripts/test-changed.sh --fast origin/development) >"$output_log" 2>&1 || status=$?
      if [ "$status" -ne "$expected_status" ]; then
        cat "$output_log" >&2
        echo "frontend $selection/$failure (no_backend=$no_backend): expected $expected_status, got $status" >&2
        exit 1
      fi
      expected_calls=
      if [ "$no_backend" -eq 0 ]; then
        expected_calls=$'bootstrap\ntest:phoenix_test_template_regression:test -p 2 -parallel 8 -tags=testhooks ./probe\n'
      fi
      expected_calls+='pnpm:install --frozen-lockfile'
      if [ "$failure" != install ]; then expected_calls+=$'\n'"$vitest_call"; fi
      if [ "$selection" = changed ] && [[ "$failure" = success || "$failure" = vitest ]]; then
        expected_calls+=$'\n'"$guard_call"
      fi
      if [ "$no_backend" -eq 0 ]; then expected_calls+=$'\nsweep'; fi
      assert_calls "$expected_calls"
      [ ! -e "$fixture/.git/phx-test-run.lock" ]
    done
  done
done

# Backend-only inputs use the same inventory as frontend changes.
rm -f "$fixture/frontend/package.json" "$fixture/frontend/src/probe.ts"
mkdir -p "$fixture/backend/modules/settings/internal/defaults"
printf 'package defaults\n' >"$fixture/backend/modules/settings/internal/defaults/probe.go"
export PHX_FIXTURE_INSTALL_STATUS=0 PHX_FIXTURE_VITEST_STATUS=0 PHX_FIXTURE_CHANGED_STATUS=0
: >"$call_log"
(cd "$fixture" && PATH="$fixture/fake-bin:$PATH" TEST_CHANGED_CALL_LOG="$call_log" \
  scripts/test-changed.sh --fast origin/development) >"$output_log" 2>&1
assert_calls $'pnpm:install --frozen-lockfile\n'"$changed_call"$'\n'"$guard_call"

# Project filters used by CI reach both invocations unchanged.
: >"$call_log"
(cd "$fixture" && PATH="$fixture/fake-bin:$PATH" TEST_CHANGED_CALL_LOG="$call_log" \
  node scripts/test-frontend-changed.mjs origin/development --disableConsoleIntercept --project api-node --project logic-node) >"$output_log" 2>&1
options='--disableConsoleIntercept --project api-node --project logic-node'
assert_calls "$changed_call $options"$'\n'"${guard_call/pnpm:exec vitest run/pnpm:exec vitest run $options}"

# Committed inputs still select the guards when modified or deleted.
git -C "$fixture" add backend/modules/settings/internal/defaults/probe.go
git -C "$fixture" commit -qm defaults
git -C "$fixture" update-ref refs/remotes/origin/development HEAD
for change in modify delete; do
  if [ "$change" = modify ]; then
    printf '// changed\n' >>"$fixture/backend/modules/settings/internal/defaults/probe.go"
  else
    rm "$fixture/backend/modules/settings/internal/defaults/probe.go"
  fi
  : >"$call_log"
  (cd "$fixture" && PATH="$fixture/fake-bin:$PATH" TEST_CHANGED_CALL_LOG="$call_log" \
    scripts/test-changed.sh --fast origin/development) >"$output_log" 2>&1
  assert_calls $'pnpm:install --frozen-lockfile\n'"$changed_call"$'\n'"$guard_call"
done

# Clear the defaults deletion so it cannot mask a missing fixed-input trigger.
git -C "$fixture" add -u
git -C "$fixture" commit -qm remove-defaults
git -C "$fixture" update-ref refs/remotes/origin/development HEAD

# Fixed backend inputs and the native launcher select guards even without Go
# packages. Cover creation, committed modification, deletion and rename.
for input in backend/auth/authorize/permissions/catalog.json \
  backend/modules/settings/records/modules.go \
  backend/modules/identityaccess/mails.go \
  backend/modules/dataimport/internal/application/importservice/staff_import_config.go \
  scripts/dev-native.sh; do
  mkdir -p "$fixture/$(dirname "$input")"
  printf 'probe\n' >"$fixture/$input"
  for change in create modify rename delete; do
    case "$change" in
      modify)
        git -C "$fixture" add "$input"
        git -C "$fixture" commit -qm "input $input"
        git -C "$fixture" update-ref refs/remotes/origin/development HEAD
        printf 'changed\n' >>"$fixture/$input" ;;
      rename) mv "$fixture/$input" "$fixture/$input.renamed" ;;
      delete) rm "$fixture/$input.renamed" ;;
    esac
    : >"$call_log"
    (cd "$fixture" && PATH="$fixture/fake-bin:$PATH" TEST_CHANGED_CALL_LOG="$call_log" \
      scripts/test-changed.sh --fast origin/development) >"$output_log" 2>&1
    assert_calls "$frontend_calls"
  done
  # Start the next case without the deleted input still selecting guards.
  git -C "$fixture" add -u
  git -C "$fixture" commit -qm "remove $input"
  git -C "$fixture" update-ref refs/remotes/origin/development HEAD
done

# Workflow-only changes must select the tour guard, not skip frontend tests.
mkdir -p "$fixture/.github/workflows"
printf '# changed E2E paths\n' >"$fixture/.github/workflows/e2e.yml"
: >"$call_log"
(cd "$fixture" && PATH="$fixture/fake-bin:$PATH" TEST_CHANGED_CALL_LOG="$call_log" \
  scripts/test-changed.sh --fast origin/development) >"$output_log" 2>&1
assert_calls $'pnpm:install --frozen-lockfile\n'"$changed_call"$'\n'"$guard_call"

echo 'full/changed-test resource budget, build workspace, filesystem guards and lifecycle tests passed'
