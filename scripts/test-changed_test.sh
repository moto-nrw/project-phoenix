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

mkdir -p "$fixture"/{backend/probe,fake-bin,scripts}
cp "$repo_root/scripts/test-changed.sh" "$fixture/scripts/test-changed.sh"
cp "$repo_root/scripts/test-run-id.sh" "$fixture/scripts/test-run-id.sh"
cat >"$fixture/scripts/backend-affected-packages.sh" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
echo './probe'
EOF
cat >"$fixture/fake-bin/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

case "$*" in
  'run ./internal/testdb/cmd/bootstrap')
    echo bootstrap >>"$TEST_CHANGED_CALL_LOG"
    echo phoenix_test_template_regression
    ;;
  'run ./internal/testdb/cmd/sweep')
    echo sweep >>"$TEST_CHANGED_CALL_LOG"
    ;;
  test\ *)
    if [ "${PHX_TEST_TEMPLATE:-}" != phoenix_test_template_regression ]; then
      echo 'go test did not receive the bootstrapped template' >&2
      exit 1
    fi
    echo "test:$PHX_TEST_TEMPLATE" >>"$TEST_CHANGED_CALL_LOG"
    ;;
  *)
    echo "unexpected go command: $*" >&2
    exit 1
    ;;
esac
EOF
chmod +x \
  "$fixture/scripts/test-changed.sh" \
  "$fixture/scripts/test-run-id.sh" \
  "$fixture/scripts/backend-affected-packages.sh" \
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
(
  cd "$fixture"
  PATH="$fixture/fake-bin:$PATH" TEST_CHANGED_CALL_LOG="$call_log" \
    scripts/test-changed.sh origin/development
)

expected=$'bootstrap\ntest:phoenix_test_template_regression\nsweep'
actual=$(cat "$call_log")
if [ "$actual" != "$expected" ]; then
  printf 'expected calls:\n%s\nactual calls:\n%s\n' "$expected" "$actual" >&2
  exit 1
fi

echo 'changed-test bootstrap test passed'
