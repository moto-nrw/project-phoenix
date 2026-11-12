#!/usr/bin/env bash
set -euo pipefail

source "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/test-helpers/git-fixture.sh"
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

mkdir -p "$fixture/backend"/{architecture/artifacts,core,consumer,consumerwithouttests,corex,internal/architecture,internal/passthrough,modules/delivery/localeconsumer,modules/delivery/internal/adapters/localization,modules/documentrendering/exportconsumer,modules/documentrendering/internal/application/listrenderer/assets,templates/email,test}
printf 'module example.test/project\n\ngo 1.25\n' >"$fixture/backend/go.mod"
printf '{}\n' >"$fixture/backend/architecture/policy.json"
printf 'package architecture\n\nimport a \"example.test/project/architecture/artifacts\"\nvar Value = a.Value\n' >"$fixture/backend/internal/architecture/architecture.go"
printf 'package passthrough\n\nimport a \"example.test/project/architecture/artifacts\"\nvar Value = a.Value\n' >"$fixture/backend/internal/passthrough/passthrough.go"
printf 'package artifacts\nconst Value = 1\n' >"$fixture/backend/architecture/artifacts/artifacts.go"
printf 'package artifacts\n' >"$fixture/backend/architecture/artifacts/artifacts_test.go"
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
printf 'package localization\n\nconst Value = 1\n' >"$fixture/backend/modules/delivery/internal/adapters/localization/locales.go"
printf 'package listrenderer\n\nconst Value = 1\n' >"$fixture/backend/modules/documentrendering/internal/application/listrenderer/service.go"
printf 'package email\n' >"$fixture/backend/templates/email/template_test.go"
printf 'package test\n' >"$fixture/backend/test/test.go"
mkdir -p "$fixture/backend/database/audits"
audit_names=(phase_edit_stale_pending_activations phase_edit_stale_class_switches booking_led_care_without_source_or_task)
for audit in "${audit_names[@]}"; do
  printf 'SELECT 1;\n' >"$fixture/backend/database/audits/$audit.sql"
done
# The query-budget register is its own package: only packages asserting budgets
# depend on it, while ordinary importers of the shared test package do not.
mkdir -p "$fixture/backend"/{test/query-budgets/api/students,budgetuser,fixtureuser}
printf 'package querybudgets\n' >"$fixture/backend/test/query-budgets/register.go"
printf '{}\n' >"$fixture/backend/test/query-budgets/api/students/list.json"
printf 'package budgetuser\n\nimport _ \"example.test/project/test/query-budgets\"\n' >"$fixture/backend/budgetuser/budgetuser_test.go"
printf 'package fixtureuser\n\nimport _ \"example.test/project/test\"\n' >"$fixture/backend/fixtureuser/fixtureuser_test.go"
cat >"$fixture/backend/modules/delivery/localeconsumer/consumer.go" <<'EOF'
package localeconsumer

import "example.test/project/modules/delivery/internal/adapters/localization"

const Value = localization.Value
EOF
printf 'package localeconsumer\n' >"$fixture/backend/modules/delivery/localeconsumer/consumer_test.go"
cat >"$fixture/backend/modules/documentrendering/exportconsumer/consumer.go" <<'EOF'
package exportconsumer

import "example.test/project/modules/documentrendering/internal/application/listrenderer"

const Value = listrenderer.Value
EOF
printf 'package exportconsumer\n' >"$fixture/backend/modules/documentrendering/exportconsumer/consumer_test.go"
# Embedded registers belong to their package; other JSON inputs are read by
# path from tests that no import edge reveals.
mkdir -p "$fixture/backend"/{modules/organizationtenancy/compose,database/migrations,auth/authorize/permissions,modules/identityaccess/internal/domain,architecture,cmd}
printf 'package organizationtenancy\n\nconst Value = 1\n' >"$fixture/backend/modules/organizationtenancy/register.go"
cat >"$fixture/backend/modules/organizationtenancy/compose/compose.go" <<'EOF'
package compose

import "example.test/project/modules/organizationtenancy"

const Value = organizationtenancy.Value
EOF
printf 'package compose\n' >"$fixture/backend/modules/organizationtenancy/compose/compose_test.go"
mkdir -p "$fixture/backend/modules/organizationtenancy/inbound/datareturn"
printf 'package datareturn\n' >"$fixture/backend/modules/organizationtenancy/inbound/datareturn/register.go"
printf 'package migrations\n' >"$fixture/backend/database/migrations/coverage_test.go"
printf 'package permissions\n\nconst Value = 1\n' >"$fixture/backend/auth/authorize/permissions/constants.go"
cat >"$fixture/backend/modules/identityaccess/internal/domain/domain.go" <<'EOF'
package domain

import "example.test/project/auth/authorize/permissions"

const Value = permissions.Value
EOF
printf 'package domain\n' >"$fixture/backend/modules/identityaccess/internal/domain/domain_test.go"
mkdir -p "$fixture/backend"/{testdata,sharedfixtureuser}
printf 'package sharedfixtureuser\n\nconst path = "../testdata/shared-cases.json"\n' >"$fixture/backend/sharedfixtureuser/cases_test.go"
printf 'package main\n\nfunc main() {}\n' >"$fixture/backend/cmd/main.go"
printf 'package main\n' >"$fixture/backend/cmd/main_test.go"
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

# A real main package may legitimately end in .test. Keep it in the graph,
# while excluding generated .test mains. Also cover test-only importers.
mkdir -p "$fixture/backend"/{graphbase,legitimate.test,testonly}
mkdir -p "$fixture/backend/retired"
printf 'package retired\n' >"$fixture/backend/retired/retired.go"
printf 'package graphbase\n\nconst Value = 1\n' >"$fixture/backend/graphbase/base.go"
printf 'package graphbase\n' >"$fixture/backend/graphbase/base_test.go"
cat >"$fixture/backend/legitimate.test/main.go" <<'EOF'
package main

import "example.test/project/graphbase"

func main() { _ = graphbase.Value }
EOF
printf 'package main\n' >"$fixture/backend/legitimate.test/main_test.go"
printf 'package testonly\n' >"$fixture/backend/testonly/consumer.go"
cat >"$fixture/backend/testonly/consumer_test.go" <<'EOF'
package testonly_test

import _ "example.test/project/graphbase"
EOF

init_git_fixture "$fixture"
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

# Audit SQL has no import edge. CI selects its test package, while --direct
# leaves the targeted source-test selection to test-changed.sh.
for audit in "${audit_names[@]}"; do
  audit_path="backend/database/audits/$audit.sql"
  printf '\n-- changed\n' >>"$fixture/$audit_path"
  assert_output 'example.test/project/test'
  assert_output_direct ''
  git -C "$fixture" restore "$audit_path"
  rm "$fixture/$audit_path"
  assert_output 'example.test/project/test'
  git -C "$fixture" restore "$audit_path"
done
printf 'SELECT 1;\n' >"$fixture/backend/database/audits/new_audit.sql"
assert_output 'example.test/project/test'
rm "$fixture/backend/database/audits/new_audit.sql"

printf '{}\n' >"$fixture/backend/modules/delivery/internal/adapters/localization/locales.json"
assert_output $'example.test/project/modules/delivery/internal/adapters/localization\nexample.test/project/modules/delivery/localeconsumer\nexample.test/project/test'
rm "$fixture/backend/modules/delivery/internal/adapters/localization/locales.json"

printf 'asset\n' >"$fixture/backend/modules/documentrendering/internal/application/listrenderer/assets/probe.txt"
assert_output $'example.test/project/modules/documentrendering/exportconsumer\nexample.test/project/modules/documentrendering/internal/application/listrenderer\nexample.test/project/test'
rm "$fixture/backend/modules/documentrendering/internal/application/listrenderer/assets/probe.txt"

# Embedded production registers select their package and its importers. The
# lifecycle catalog is also read by path from the migration coverage test.
printf '{}\n' >"$fixture/backend/modules/organizationtenancy/return_register.json"
assert_output $'example.test/project/modules/organizationtenancy\nexample.test/project/modules/organizationtenancy/compose\nexample.test/project/test'
assert_lint_output pull_request false $'./modules/organizationtenancy\n./modules/organizationtenancy/compose\n./test'
rm "$fixture/backend/modules/organizationtenancy/return_register.json"
printf '{}\n' >"$fixture/backend/modules/organizationtenancy/data_lifecycle_catalog.json"
assert_output $'example.test/project/database/migrations\nexample.test/project/modules/organizationtenancy\nexample.test/project/modules/organizationtenancy/compose\nexample.test/project/test'
rm "$fixture/backend/modules/organizationtenancy/data_lifecycle_catalog.json"
printf '{}\n' >"$fixture/backend/modules/organizationtenancy/inbound/datareturn/openapi.json"
assert_output $'example.test/project/modules/organizationtenancy/inbound/datareturn\nexample.test/project/test'
rm "$fixture/backend/modules/organizationtenancy/inbound/datareturn/openapi.json"

# Read by path from tests only: no production closure, although the permission
# catalog sits beside a widely imported package.
printf '[]\n' >"$fixture/backend/auth/authorize/permissions/catalog.json"
assert_output $'example.test/project/modules/identityaccess/internal/domain\nexample.test/project/test'
rm "$fixture/backend/auth/authorize/permissions/catalog.json"
printf '{}\n' >"$fixture/backend/architecture/composition.json"
assert_output $'example.test/project/cmd\nexample.test/project/test'
rm "$fixture/backend/architecture/composition.json"

# Fixtures shared across packages select the tests that name them.
printf '[]\n' >"$fixture/backend/testdata/shared-cases.json"
assert_output $'example.test/project/sharedfixtureuser\nexample.test/project/test'
rm "$fixture/backend/testdata/shared-cases.json"
printf 'note\n' >"$fixture/backend/testdata/README.md"
assert_output ''
rm "$fixture/backend/testdata/README.md"

# Notes beside these inputs stay outside the selection.
printf 'note\n' >"$fixture/backend/modules/organizationtenancy/README.md"
assert_output ''
rm "$fixture/backend/modules/organizationtenancy/README.md"

# Shrink-only registers feed the ratchets in ./test, which no import edge can
# reveal. A register-only change must still run them.
for register in transaction_boundaries durable_side_effect_notices module_contract_inventory; do
  printf '[]\n' >"$fixture/backend/test/$register.json"
  assert_output 'example.test/project/test'
  rm "$fixture/backend/test/$register.json"
done

printf '{"changed":true}\n' >"$fixture/backend/architecture/policy.json"
assert_output $'example.test/project/internal/architecture\nexample.test/project/test'
git -C "$fixture" restore backend/architecture/policy.json

printf '\n// changed\n' >>"$fixture/backend/architecture/artifacts/artifacts.go"
assert_output $'example.test/project/architecture/artifacts\nexample.test/project/internal/architecture\nexample.test/project/internal/passthrough\nexample.test/project/test'
git -C "$fixture" restore backend/architecture/artifacts/artifacts.go
printf '\n// changed\n' >>"$fixture/backend/architecture/artifacts/artifacts_test.go"
assert_output $'example.test/project/architecture/artifacts\nexample.test/project/test'
git -C "$fixture" restore backend/architecture/artifacts/artifacts_test.go


# Reviewed inventory fragments are inputs to independent architecture and debt
# tests, even when no Go source changes. The selector must emit separate rows.
for artifact in command-effects passthrough; do
  mkdir -p "$fixture/backend/architecture/$artifact/reviews"
  printf '{}\n' >"$fixture/backend/architecture/$artifact/reviews/owner.json"
  assert_output $'example.test/project/internal/architecture\nexample.test/project/internal/passthrough\nexample.test/project/test'
  rm -r "$fixture/backend/architecture/$artifact"
done
printf '{"changed":true}\n' >"$fixture/backend/test/query-budgets/api/students/list.json"
assert_output $'example.test/project/budgetuser\nexample.test/project/internal/architecture\nexample.test/project/test\nexample.test/project/test/query-budgets'
assert_output_direct $'example.test/project/budgetuser\nexample.test/project/internal/architecture\nexample.test/project/test/query-budgets'
git -C "$fixture" restore backend/test/query-budgets/api/students/list.json


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

printf '\n// changed\n' >>"$fixture/backend/graphbase/base.go"
assert_output $'example.test/project/graphbase\nexample.test/project/legitimate.test\nexample.test/project/test\nexample.test/project/testonly'
assert_output_direct $'example.test/project/graphbase\nexample.test/project/legitimate.test\nexample.test/project/testonly'
git -C "$fixture" restore backend/graphbase/base.go

# Removing an unused package must validate imports without compiling the whole
# backend first. A cold cache makes accidental export builds observable even
# when this test is run repeatedly on a developer's warm Go cache.
cat >"$fixture/no-compile" <<'EOF'
#!/usr/bin/env bash
echo 'package selection unexpectedly invoked the Go compiler' >&2
exit 1
EOF
chmod +x "$fixture/no-compile"
mv "$fixture/backend/retired/retired.go" "$fixture/retired.go"
actual=$(GOCACHE="$fixture/cold-cache" GOFLAGS="${GOFLAGS:-} -toolexec=$fixture/no-compile" select_packages)
if [ -n "$actual" ]; then
  echo "deleted unused package unexpectedly selected packages: $actual" >&2
  exit 1
fi
mv "$fixture/retired.go" "$fixture/backend/retired/retired.go"

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
assert_workflow_filter backend-tests 'backend/database/audits/*.sql' present
assert_workflow_filter backend-tests 'backend/architecture/policy.json' present
assert_workflow_filter error-codes 'frontend/src/lib/error-codes/*.generated.ts' present
assert_workflow_filter backend-production 'backend/**/testdata/**' absent
assert_workflow_filter backend-lint 'scripts/backend-affected-packages.sh' present
assert_workflow_filter backend-lint 'scripts/backend-affected-packages_test.sh' present
assert_workflow_filter backend-lint 'scripts/backend-lint-packages.sh' present
assert_workflow_filter backend-test-infra 'scripts/backend-lint-packages.sh' present
assert_workflow_filter backend-test-infra 'scripts/test-run-id.sh' present
assert_workflow_filter backend-test-infra 'scripts/backend-test-matrix.sh' present
assert_workflow_filter backend-test-infra 'scripts/backend-test-matrix_test.sh' present
assert_workflow_filter backend-test-infra 'scripts/backend-source-ratchets.mjs' present
# Local wrappers start their self-tests in ci-scripts, not the full backend suite.
for path in 'scripts/test-changed.sh' 'scripts/test-changed_test.sh' 'scripts/test-backend.sh' 'scripts/go-test-link-cache.sh'; do
  assert_workflow_filter backend-test-infra "$path" absent
  assert_workflow_filter backend-local-test-scripts "$path" present
done
assert_workflow_output_contains run-ci-scripts "steps.filter.outputs.backend-local-test-scripts == 'true'"
assert_workflow_output_contains toolchain-changed "steps.filter.outputs.backend-local-test-scripts == 'true'"
for path in 'scripts/backend-source-ratchets.mjs' 'backend/test/*_test.go'; do
  assert_workflow_filter script-tests "$path" present
  # Do not buy the release-restore or extra toolchain-ratchet jobs for these.
  assert_workflow_filter tooling "$path" absent
  assert_workflow_filter hooks "$path" absent
done
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
  'backend/modules/delivery/internal/adapters/localization/locales.json' \
  'backend/modules/documentrendering/internal/application/listrenderer/assets/**'; do
  assert_workflow_filter backend-tests "$pattern" present
  assert_workflow_filter backend-production "$pattern" present
done
assert_workflow_filter backend-tests 'backend/test/*.json' present
assert_workflow_filter backend-production 'backend/test/*.json' absent
for pattern in \
  'backend/modules/organizationtenancy/return_register.json' \
  'backend/modules/organizationtenancy/inbound/datareturn/openapi.json' \
  'backend/modules/organizationtenancy/data_lifecycle_catalog.json'; do
  for filter in backend-tests backend-lint backend-production; do
    assert_workflow_filter "$filter" "$pattern" present
  done
done
for pattern in \
  'backend/auth/authorize/permissions/catalog.json' \
  'backend/architecture/composition.json'; do
  assert_workflow_filter backend-tests "$pattern" present
  assert_workflow_filter backend-production "$pattern" absent
done
# Inventory: a JSON file under backend/ is either listed in the backend-tests
# filter or a new input nobody classified. Add it to the filter and to the
# selector in backend-affected-packages.sh.
backend_test_patterns=$(awk '
  $0 == "            backend-tests:" { active = 1; next }
  active && /^            [a-z-]+:$/ { exit }
  active && match($0, /\047[^\047]+\047/) { print substr($0, RSTART + 1, RLENGTH - 2) }
' "$repo_root/.github/workflows/main.yml")
while IFS= read -r path; do
  case "$path" in
    # Read only by the architecture ratchet job (backend-architecture filter);
    # its tests build their own repositories.
    backend/architecture/query-budget-non-http.json | \
      backend/architecture/query-budget-retirements.json) continue ;;
  esac
  covered=false
  while IFS= read -r pattern; do
    # As in the filter: * stays inside one path segment, ** crosses them,
    # and **/ also matches no directory at all.
    regex=$(printf '%s' "$pattern" | sed -e 's/[.%]/\\&/g' -e 's|\*\*/|(.@/)?|g' \
      -e 's|\*\*|.@|g' -e 's|\*|[^/]*|g' -e 's|@|*|g')
    if [[ "$path" =~ ^${regex}$ ]]; then
      covered=true
      break
    fi
  done <<< "$backend_test_patterns"
  [ "$covered" = true ] || { echo "backend JSON input missing from the backend-tests filter: $path" >&2; exit 1; }
done < <(git -C "$repo_root" ls-files 'backend/*.json')
# Every literal path the selector maps must still exist in the repository.
for path in \
  backend/architecture/policy.json \
  backend/templates/email \
  backend/modules/delivery/internal/adapters/localization/locales.json \
  backend/modules/documentrendering/internal/application/listrenderer/assets \
  backend/test/transaction_boundaries.json \
  backend/test/durable_side_effect_notices.json \
  backend/test/module_contract_inventory.json \
  backend/modules/organizationtenancy/return_register.json \
  backend/modules/organizationtenancy/inbound/datareturn/openapi.json \
  backend/modules/organizationtenancy/data_lifecycle_catalog.json \
  backend/database/migrations/data_lifecycle_coverage_test.go \
  backend/auth/authorize/permissions/catalog.json \
  backend/modules/identityaccess/internal/domain/mfa_privilege_test.go \
  backend/architecture/composition.json \
  backend/cmd/composition_smoke_test.go; do
  [ -e "$repo_root/$path" ] || { echo "selector path no longer exists: $path" >&2; exit 1; }
done

assert_workflow_output_contains run-backend '${{ github.event_name == '\''push'\'' ||'
assert_workflow_output_contains run-frontend '${{ github.event_name == '\''push'\'' ||'
assert_workflow_output_contains run-backend-lint '${{ github.event_name == '\''push'\'' ||'
assert_workflow_output_contains full-backend-lint '${{ github.event_name == '\''push'\'' ||'
assert_workflow_output_contains run-backend-architecture '${{ github.event_name == '\''push'\'' ||'
assert_workflow_output_contains full-backend-suite "steps.backend-size.outputs.full == 'true'"

# Exercise the real cheap guard, including deduplication and non-Go files. A
# broad refactor must enter the fast full-suite path before scheduling tests.
size_script=$(awk '
  /^      - name: Size backend test scope$/ { active = 1; next }
  active && /^        run: \|$/ { script = 1; next }
  script && /^  [a-z-]+:$/ { exit }
  script { sub(/^          /, ""); print }
' "$repo_root/.github/workflows/main.yml")
[ -n "$size_script" ] || { echo 'backend scope size guard missing' >&2; exit 1; }
for count in 63 64; do
  files=$(jq -cn --argjson count "$count" '[range(0; $count) | "backend/package\(.)/source.go", "backend/package\(.)/source_test.go", "backend/notes\(.)/README.md"]')
  CHANGED_COUNT=$((count * 3)) CHANGED_FILES="$files" GITHUB_OUTPUT="$fixture/scope-size" bash -euo pipefail <<<"$size_script"
  expected=false
  [ "$count" -lt 64 ] || expected=true
  grep -qx "full=$expected" "$fixture/scope-size"
  rm "$fixture/scope-size"
done
for manifest in go.mod go.sum; do
  CHANGED_COUNT=1 CHANGED_FILES="[\"backend/$manifest\"]" GITHUB_OUTPUT="$fixture/scope-size" bash -euo pipefail <<<"$size_script"
  grep -qx 'full=true' "$fixture/scope-size"
  rm "$fixture/scope-size"
done
# A release-sized list never reaches the environment (128 KiB per value), so
# the guard decides from the count alone and must not read the empty list.
for count in 499 500; do
  CHANGED_COUNT="$count" CHANGED_FILES='[]' GITHUB_OUTPUT="$fixture/scope-size" bash -euo pipefail <<<"$size_script"
  expected=false
  [ "$count" -lt 500 ] || expected=true
  grep -qx "full=$expected" "$fixture/scope-size"
  rm "$fixture/scope-size"
done
grep -qF "fromJSON(steps.filter.outputs.backend-tests_count) < 500 && steps.filter.outputs.backend-tests_files || '[]'" "$repo_root/.github/workflows/main.yml" ||
  { echo 'backend scope size guard must keep long file lists out of the environment' >&2; exit 1; }

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
    CI_SCRIPTS_REQUIRED=false ENV_CHECK_REQUIRED=false ERROR_CODES_REQUIRED=false \
    ARCHITECTURE_RESULT=skipped LINT_RESULT=skipped TEST_RESULT=skipped \
    CI_SCRIPTS_RESULT=skipped ENV_CHECK_RESULT=skipped \
    "$@" bash -e -u -o pipefail <<<"$ci_gate_script" >/dev/null 2>&1 || actual=fail
  if [[ "$actual" != "$expected" ]]; then
    echo "ci-gate: expected $expected, got $actual for $*" >&2
    exit 1
  fi
}

assert_ci_gate pass
assert_ci_gate pass ARCHITECTURE_REQUIRED=true ARCHITECTURE_RESULT=success
assert_ci_gate fail ARCHITECTURE_REQUIRED=true
for requirement in BACKEND_LINT_REQUIRED FRONTEND_LINT_REQUIRED ERROR_CODES_REQUIRED; do
  assert_ci_gate fail "$requirement=true"
  assert_ci_gate pass "$requirement=true" LINT_RESULT=success
done
for requirement in BACKEND_TEST_REQUIRED FRONTEND_TEST_REQUIRED SEED_SMOKE_REQUIRED; do
  assert_ci_gate fail "$requirement=true"
  assert_ci_gate pass "$requirement=true" TEST_RESULT=success
done
assert_ci_gate fail CI_SCRIPTS_REQUIRED=true
assert_ci_gate pass CI_SCRIPTS_REQUIRED=true CI_SCRIPTS_RESULT=success
assert_ci_gate fail ENV_CHECK_REQUIRED=true
assert_ci_gate pass ENV_CHECK_REQUIRED=true ENV_CHECK_RESULT=success
# Failures/cancellations remain the alls-green action's responsibility. Do not
# permit either outcome while allowing intentionally skipped reusable jobs.
if ! grep -Fq 'CI_NEEDS: ${{ toJSON(needs) }}' "$repo_root/.github/workflows/main.yml" ||
   ! grep -Fq 'id: normalize' "$repo_root/.github/workflows/main.yml" ||
   ! grep -Fq 'run: python3 scripts/normalize-ci-needs.py' "$repo_root/.github/workflows/main.yml" ||
   ! grep -Fq 'jobs: ${{ steps.normalize.outputs.jobs }}' "$repo_root/.github/workflows/main.yml" ||
   ! grep -Fq 'allowed-skips: lint, test, backend-architecture-ratchet, ci-scripts, env-sync-check' "$repo_root/.github/workflows/main.yml" ||
   grep -Eq 'allowed-failures:|allowed-cancelled:' "$repo_root/.github/workflows/main.yml"; then
  echo 'ci-gate must normalize all results before alls-green without allowing failures' >&2
  exit 1
fi
if ! grep -Fq 'packages_output=$(scripts/backend-lint-packages.sh)' \
  "$repo_root/.github/workflows/lint.yml"; then
  echo 'lint workflow does not propagate selector failures' >&2
  exit 1
fi

echo 'backend affected-package selector tests passed'
