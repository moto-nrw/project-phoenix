#!/usr/bin/env bash
set -euo pipefail
repo_root=$(git rev-parse --show-toplevel)

assert_partition() {
  packages=$1
  full=$2
  expected_jobs=$3
  matrix=$(printf '%s\n' "$packages" | FULL_SUITE="$full" SPLIT_SUITES="${SPLIT_SUITES:-false}" "$repo_root/scripts/backend-test-matrix.sh")
  actual=$(jq -r '[.include[].packages | split("\n")[] | select(length > 0)] | sort | .[]' <<<"$matrix")
  expected=$(printf '%s\n' "$packages" | sed '/^$/d' | sort -u)
  [ "$actual" = "$expected" ] || { echo 'test matrix lost or duplicated selected packages' >&2; exit 1; }
  jq -e --argjson count "$expected_jobs" '
    (.include | length) == $count and
    ([.include[].shard] | unique | length) == $count and
    all(.include[]; (.runner == "blacksmith-4vcpu-ubuntu-2404" and .workers == 4) or
                    (.runner == "blacksmith-8vcpu-ubuntu-2404" and .workers == 6))
  ' <<<"$matrix" >/dev/null
}

special=$'example.test/project/internal/architecture\nexample.test/project/database/migrations'
application=$(printf 'example.test/project/package%03d\n' {1..65})

# Default: one job, whatever the selection.
assert_partition '' false 1
assert_partition "$special" false 1
jq -e '.include[0].runner == "blacksmith-4vcpu-ubuntu-2404" and .include[0].blobs == true' <<<"$matrix" >/dev/null
assert_partition 'example.test/project/api' false 1
jq -e '.include[0].blobs == false' <<<"$matrix" >/dev/null
assert_partition "$application"$'\n'"$special" false 1
jq -e '.include[0].runner == "blacksmith-8vcpu-ubuntu-2404" and .include[0].shard == 0' <<<"$matrix" >/dev/null
assert_partition 'example.test/project/api' true 1
jq -e '.include[0].runner == "blacksmith-8vcpu-ubuntu-2404"' <<<"$matrix" >/dev/null

# SPLIT_SUITES=true (nightly race run) keeps the former partition.
export SPLIT_SUITES=true
assert_partition '' false 1
assert_partition 'example.test/project/api' false 1
jq -e '.include[0].runner == "blacksmith-4vcpu-ubuntu-2404"' <<<"$matrix" >/dev/null

assert_partition "$special" false 2
jq -e 'all(.include[]; .runner == "blacksmith-4vcpu-ubuntu-2404")' <<<"$matrix" >/dev/null
assert_partition "$special" true 2
jq -e 'all(.include[]; .runner == "blacksmith-8vcpu-ubuntu-2404")' <<<"$matrix" >/dev/null

application=$(printf 'example.test/project/package%03d\n' {1..64})
assert_partition "$application" false 1
assert_partition "$application" true 2
application+=$'\nexample.test/project/package065'
assert_partition "$application" false 2
assert_partition "$application"$'\n'"$special" false 4
jq -e 'all(.include[] | select(.shard == 1 or .shard == 2); .packages | split("\n") | length == 1)' <<<"$matrix" >/dev/null
jq -e '[.include[] | select(.blobs)] | map(.shard) == [1]' <<<"$matrix" >/dev/null
unset SPLIT_SUITES

# Real main-package paths ending in .test are application packages, not the
# generated Go test mains. Duplicated discovery output must not rerun tests.
assert_partition $'example.test/project/real.test\nexample.test/project/real.test' false 1
echo 'backend test matrix tests passed'
