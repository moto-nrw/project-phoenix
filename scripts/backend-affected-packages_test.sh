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

mkdir -p "$fixture/backend"/{core,consumer,corex,localeconsumer,localization,exportconsumer,services/listexport/assets,templates/email,test}
printf 'module example.test/project\n\ngo 1.25\n' >"$fixture/backend/go.mod"
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

git -C "$fixture" init -q
git -C "$fixture" config user.email selector-test@example.invalid
git -C "$fixture" config user.name selector-test
git -C "$fixture" config commit.gpgSign false
git -C "$fixture" add .
git -C "$fixture" commit -qm baseline

select_packages() {
  working_directory=${1:-$fixture}
  (cd "$working_directory" && "$repo_root/scripts/backend-affected-packages.sh" HEAD)
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
assert_output $'example.test/project/consumer\nexample.test/project/core\nexample.test/project/test'
assert_output $'example.test/project/consumer\nexample.test/project/core\nexample.test/project/test' "$fixture/backend"
git -C "$fixture" restore backend/core/core.go

mkdir -p "$fixture/backend/consumer/testdata"
printf 'fixture\n' >"$fixture/backend/consumer/testdata/input.golden"
assert_output $'example.test/project/consumer\nexample.test/project/test'
rm "$fixture/backend/consumer/testdata/input.golden"

printf 'template\n' >"$fixture/backend/templates/email/probe.html"
assert_output $'example.test/project/templates/email\nexample.test/project/test'
rm "$fixture/backend/templates/email/probe.html"

printf '{}\n' >"$fixture/backend/localization/locales.json"
assert_output $'example.test/project/localeconsumer\nexample.test/project/localization\nexample.test/project/test'
rm "$fixture/backend/localization/locales.json"

printf 'asset\n' >"$fixture/backend/services/listexport/assets/probe.txt"
assert_output $'example.test/project/exportconsumer\nexample.test/project/services/listexport\nexample.test/project/test'
rm "$fixture/backend/services/listexport/assets/probe.txt"

printf '\n// changed\n' >>"$fixture/backend/go.mod"
assert_output './...'
git -C "$fixture" restore backend/go.mod

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
assert_workflow_filter backend-production 'backend/**/testdata/**' absent
for pattern in \
  'backend/templates/email/**' \
  'backend/localization/locales.json' \
  'backend/services/listexport/assets/**'; do
  assert_workflow_filter backend-tests "$pattern" present
  assert_workflow_filter backend-production "$pattern" present
done

assert_workflow_output_contains run-backend '${{ github.event_name == '\''push'\'' ||'
assert_workflow_output_contains run-frontend '${{ github.event_name == '\''push'\'' ||'

echo 'backend affected-package selector tests passed'
