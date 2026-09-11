#!/usr/bin/env bash
set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
fixture=$(mktemp -d)
cleanup() {
  if [ -n "$fixture" ] && [ "$fixture" != / ] && [ -d "$fixture/.git" ]; then
    rm -rf -- "$fixture"
  else
    echo "Refusing to delete unexpected fixture path: $fixture" >&2
    return 1
  fi
}
trap cleanup EXIT

mkdir -p "$fixture/backend"/{architecture,core,consumer,consumerwithouttests,corex,internal/architecture,localeconsumer,localization,exportconsumer,services/listexport/assets,templates/email,test}
printf 'module example.test/project\n\ngo 1.25\n' >"$fixture/backend/go.mod"
printf '{}\n' >"$fixture/backend/architecture/policy.json"
printf 'package architecture\n' >"$fixture/backend/internal/architecture/architecture.go"
printf 'package core\n\nconst Value = 1\n' >"$fixture/backend/core/core.go"
cat >"$fixture/backend/core/core_test.go" <<'EOF'
package core

import _ "example.test/project/corex"
EOF
cat >"$fixture/backend/consumer/consumer.go" <<'EOF'
package consumer

import "example.test/project/core"

const Value = core.Value
EOF
printf 'package consumer\n' >"$fixture/backend/consumer/consumer_test.go"
cat >"$fixture/backend/consumerwithouttests/consumer.go" <<'EOF'
package consumerwithouttests

import "example.test/project/core"

const Value = core.Value
EOF
printf 'package corex\n' >"$fixture/backend/corex/corex.go"
printf 'package corex\n' >"$fixture/backend/corex/corex_test.go"
printf 'package localization\n\nconst Value = 1\n' >"$fixture/backend/localization/locales.go"
printf 'package listexport\n\nconst Value = 1\n' >"$fixture/backend/services/listexport/export.go"
printf 'package email\n' >"$fixture/backend/templates/email/template_test.go"
printf 'package test\n' >"$fixture/backend/test/test.go"
cat >"$fixture/backend/localeconsumer/consumer.go" <<'EOF'
package localeconsumer

import "example.test/project/localization"

const Value = localization.Value
EOF
printf 'package localeconsumer\n' >"$fixture/backend/localeconsumer/consumer_test.go"
cat >"$fixture/backend/exportconsumer/consumer.go" <<'EOF'
package exportconsumer

import "example.test/project/services/listexport"

const Value = listexport.Value
EOF
printf 'package exportconsumer\n' >"$fixture/backend/exportconsumer/consumer_test.go"
# Depth-2 import chain for the --direct (depth 1) mode, deliberately separate
# from the core chain so the existing closure assertions stay untouched.
mkdir -p "$fixture/backend"/{chainbase,chainmid,chainleaf}
printf 'package chainbase\n\nconst Value = 1\n' >"$fixture/backend/chainbase/chainbase.go"
printf 'package chainbase\n' >"$fixture/backend/chainbase/chainbase_test.go"
cat >"$fixture/backend/chainmid/chainmid.go" <<'EOF'
package chainmid

import "example.test/project/chainbase"

const Value = chainbase.Value
EOF
cat >"$fixture/backend/chainleaf/chainleaf.go" <<'EOF'
package chainleaf

import "example.test/project/chainmid"

const Value = chainmid.Value
EOF

git -C "$fixture" init -q
git -C "$fixture" config user.email selector-test@example.invalid
git -C "$fixture" config user.name selector-test
git -C "$fixture" config commit.gpgSign false
git -C "$fixture" add .
git -C "$fixture" commit -qm baseline
git -C "$fixture" update-ref refs/remotes/origin/base HEAD

select_packages() {
  working_directory=${1:-$fixture}
  (cd "$working_directory" && "$repo_root/scripts/backend-affected-packages.sh" HEAD)
}

select_packages_direct() {
  working_directory=${1:-$fixture}
  (cd "$working_directory" && "$repo_root/scripts/backend-affected-packages.sh" --direct HEAD)
}

select_lint_packages() {
  event_name=$1
  full_backend=$2
  working_directory=${3:-$fixture}
  (
    cd "$working_directory"
    BASE_REF=base EVENT_NAME="$event_name" FULL_BACKEND="$full_backend" \
      "$repo_root/scripts/backend-lint-packages.sh"
  )
}

assert_output() {
  expected=$1
  working_directory=${2:-$fixture}
  actual=$(select_packages "$working_directory")
  if [ "$actual" != "$expected" ]; then
    printf 'expected:\n%s\nactual:\n%s\n' "$expected" "$actual" >&2
    exit 1
  fi
}

assert_output_direct() {
  expected=$1
  working_directory=${2:-$fixture}
  actual=$(select_packages_direct "$working_directory")
  if [ "$actual" != "$expected" ]; then
    printf 'expected (direct):\n%s\nactual:\n%s\n' "$expected" "$actual" >&2
    exit 1
  fi
}

assert_lint_output() {
  event_name=$1
  full_backend=$2
  expected=$3
  actual=$(select_lint_packages "$event_name" "$full_backend")
  if [ "$actual" != "$expected" ]; then
    printf 'expected lint packages:\n%s\nactual:\n%s\n' "$expected" "$actual" >&2
    exit 1
  fi
}

assert_workflow_filter() {
  filter=$1
  pattern=$2
  expected=$3
  actual=$(awk -v filter="$filter:" -v pattern="- '$pattern'" '
    $0 == "            " filter { active = 1; next }
    active && $0 ~ /^            [a-z-]+:$/ { exit }
    active && index($0, pattern) { found = 1 }
    END { print found ? "present" : "absent" }
  ' "$repo_root/.github/workflows/main.yml")
  if [ "$actual" != "$expected" ]; then
    echo "workflow filter $filter: expected $pattern to be $expected, got $actual" >&2
    exit 1
  fi
}

assert_workflow_output_contains() {
  output=$1
  fragment=$2
  line=$(grep -F "      $output:" "$repo_root/.github/workflows/main.yml" || true)
  if [[ "$line" != *"$fragment"* ]]; then
    echo "workflow output $output: expected to contain $fragment" >&2
    exit 1
  fi
}

printf '\n// changed\n' >>"$fixture/backend/core/core_test.go"
assert_output $'example.test/project/core\nexample.test/project/test'
git -C "$fixture" restore backend/core/core_test.go

printf '\n// changed\n' >>"$fixture/backend/core/core.go"
assert_output $'example.test/project/consumer\nexample.test/project/consumerwithouttests\nexample.test/project/core\nexample.test/project/test'
assert_output $'example.test/project/consumer\nexample.test/project/consumerwithouttests\nexample.test/project/core\nexample.test/project/test' "$fixture/backend"
assert_lint_output pull_request false $'./consumer\n./consumerwithouttests\n./core\n./test'
git -C "$fixture" restore backend/core/core.go

assert_lint_output pull_request false './...'
assert_lint_output pull_request true './...'
assert_lint_output push false './...'
if (
  cd "$fixture"
  BASE_REF=missing EVENT_NAME=pull_request FULL_BACKEND=false \
    "$repo_root/scripts/backend-lint-packages.sh" >/dev/null 2>&1
); then
  echo 'partial lint unexpectedly ignored a missing base ref' >&2
  exit 1
fi

mkdir -p "$fixture/backend/consumer/testdata"
printf 'fixture\n' >"$fixture/backend/consumer/testdata/input.golden"
assert_output $'example.test/project/consumer\nexample.test/project/test'
rm "$fixture/backend/consumer/testdata/input.golden"

mkdir -p "$fixture/backend/consumer/testdata/module/source"
printf 'module example.test/fixture\n\ngo 1.25\n' >"$fixture/backend/consumer/testdata/module/go.mod"
printf 'package source\n' >"$fixture/backend/consumer/testdata/module/source/source.go"
assert_output $'example.test/project/consumer\nexample.test/project/test'
rm "$fixture/backend/consumer/testdata/module/source/source.go"
rm "$fixture/backend/consumer/testdata/module/go.mod"
rmdir "$fixture/backend/consumer/testdata/module/source"
rmdir "$fixture/backend/consumer/testdata/module"

printf 'template\n' >"$fixture/backend/templates/email/probe.html"
assert_output $'example.test/project/templates/email\nexample.test/project/test'
rm "$fixture/backend/templates/email/probe.html"

printf '{}\n' >"$fixture/backend/localization/locales.json"
assert_output $'example.test/project/localeconsumer\nexample.test/project/localization\nexample.test/project/test'
rm "$fixture/backend/localization/locales.json"

printf 'asset\n' >"$fixture/backend/services/listexport/assets/probe.txt"
assert_output $'example.test/project/exportconsumer\nexample.test/project/services/listexport\nexample.test/project/test'
rm "$fixture/backend/services/listexport/assets/probe.txt"

printf '{"changed":true}\n' >"$fixture/backend/architecture/policy.json"
assert_output $'example.test/project/internal/architecture\nexample.test/project/test'
git -C "$fixture" restore backend/architecture/policy.json

printf '\n// changed\n' >>"$fixture/backend/go.mod"
assert_output './...'
assert_output_direct './...'
git -C "$fixture" restore backend/go.mod

# --direct: depth-1 importers only, and no auto-appended ./test package.
printf '\n// changed\n' >>"$fixture/backend/chainbase/chainbase.go"
assert_output $'example.test/project/chainbase\nexample.test/project/chainleaf\nexample.test/project/chainmid\nexample.test/project/test'
assert_output_direct $'example.test/project/chainbase\nexample.test/project/chainmid'
git -C "$fixture" restore backend/chainbase/chainbase.go

printf '\n// changed\n' >>"$fixture/backend/chainbase/chainbase_test.go"
assert_output_direct 'example.test/project/chainbase'
git -C "$fixture" restore backend/chainbase/chainbase_test.go

mv "$fixture/backend/corex/corex.go" "$fixture/corex.go"
if select_packages >/dev/null 2>&1; then
  echo 'deleted package imported only by tests unexpectedly passed graph validation' >&2
  exit 1
fi
mv "$fixture/corex.go" "$fixture/backend/corex/corex.go"

mv "$fixture/backend/core/core.go" "$fixture/core.go"
if select_packages >/dev/null 2>&1; then
  echo 'deleted imported package unexpectedly passed graph validation' >&2
  exit 1
fi

assert_workflow_filter backend-tests 'backend/**/testdata/**' present
assert_workflow_filter backend-tests 'backend/architecture/policy.json' present
assert_workflow_filter backend-production 'backend/**/testdata/**' absent
assert_workflow_filter backend-lint 'scripts/backend-affected-packages.sh' present
assert_workflow_filter backend-lint 'scripts/backend-affected-packages_test.sh' present
assert_workflow_filter backend-lint 'scripts/backend-lint-packages.sh' present
assert_workflow_filter backend-test-infra 'scripts/backend-lint-packages.sh' present
assert_workflow_filter backend-test-infra 'scripts/test-run-id.sh' present
assert_workflow_filter backend-test-infra '.claude/hooks/guard-absolute-rules.sh' present
assert_workflow_filter backend-test-infra '.claude/hooks/guard-absolute-rules_test.sh' present
for filter in backend-tests backend-lint backend-architecture; do
  assert_workflow_filter "$filter" 'scripts/backend-architecture.sh' present
  assert_workflow_filter "$filter" 'scripts/backend-architecture/**' present
done
for pattern in \
  'backend/go.mod' \
  'backend/go.sum' \
  'backend/.golangci.yml' \
  'scripts/backend-affected-packages.sh' \
  'scripts/backend-affected-packages_test.sh' \
  'scripts/backend-lint-packages.sh'; do
  assert_workflow_filter backend-lint-full "$pattern" present
done
for pattern in \
  'backend/templates/email/**' \
  'backend/localization/locales.json' \
  'backend/services/listexport/assets/**'; do
  assert_workflow_filter backend-tests "$pattern" present
  assert_workflow_filter backend-production "$pattern" present
done

assert_workflow_output_contains run-backend '${{ github.event_name == '\''push'\'' ||'
assert_workflow_output_contains run-frontend '${{ github.event_name == '\''push'\'' ||'
assert_workflow_output_contains run-backend-lint '${{ github.event_name == '\''push'\'' ||'
assert_workflow_output_contains full-backend-lint '${{ github.event_name == '\''push'\'' ||'
assert_workflow_output_contains run-backend-architecture '${{ github.event_name == '\''push'\'' ||'

# Execute the actual workflow guard: architecture-only changes may skip both
# reusable jobs, but each independently requested job must run.
ci_gate_script=$(awk '
  /^      - name: Reject skipped mandatory checks$/ { active = 1; next }
  active && /^        run: \|$/ { script = 1; next }
  script && /^      - / { exit }
  script { sub(/^          /, ""); print }
' "$repo_root/.github/workflows/main.yml")
if [[ -z "$ci_gate_script" ]]; then
  echo 'mandatory-check guard missing from workflow' >&2
  exit 1
fi

assert_ci_gate() {
  expected=$1
  shift
  actual=pass
  env ARCHITECTURE_REQUIRED=false BACKEND_LINT_REQUIRED=false FRONTEND_LINT_REQUIRED=false \
    BACKEND_TEST_REQUIRED=false FRONTEND_TEST_REQUIRED=false SEED_SMOKE_REQUIRED=false \
    ARCHITECTURE_RESULT=skipped LINT_RESULT=skipped TEST_RESULT=skipped \
    "$@" bash -e -u -o pipefail <<<"$ci_gate_script" >/dev/null 2>&1 || actual=fail
  if [[ "$actual" != "$expected" ]]; then
    echo "ci-gate: expected $expected, got $actual for $*" >&2
    exit 1
  fi
}

assert_ci_gate pass
assert_ci_gate pass ARCHITECTURE_REQUIRED=true ARCHITECTURE_RESULT=success
assert_ci_gate fail ARCHITECTURE_REQUIRED=true
for requirement in BACKEND_LINT_REQUIRED FRONTEND_LINT_REQUIRED; do
  assert_ci_gate fail "$requirement=true"
  assert_ci_gate pass "$requirement=true" LINT_RESULT=success
done
for requirement in BACKEND_TEST_REQUIRED FRONTEND_TEST_REQUIRED SEED_SMOKE_REQUIRED; do
  assert_ci_gate fail "$requirement=true"
  assert_ci_gate pass "$requirement=true" TEST_RESULT=success
done
# Failures/cancellations remain the alls-green action's responsibility. Do not
# permit either outcome while allowing intentionally skipped reusable jobs.
if ! grep -Fq 'jobs: ${{ toJSON(needs) }}' "$repo_root/.github/workflows/main.yml" ||
   ! grep -Fq 'allowed-skips: lint, test, backend-architecture-ratchet' "$repo_root/.github/workflows/main.yml" ||
   grep -Eq 'allowed-failures:|allowed-cancelled:' "$repo_root/.github/workflows/main.yml"; then
  echo 'ci-gate must still pass all results to alls-green without allowing failures' >&2
  exit 1
fi
if ! grep -Fq 'packages_output=$(scripts/backend-lint-packages.sh)' \
  "$repo_root/.github/workflows/lint.yml"; then
  echo 'lint workflow does not propagate selector failures' >&2
  exit 1
fi

echo 'backend affected-package selector tests passed'
