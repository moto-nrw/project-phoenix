#!/usr/bin/env bash
# Partition the selected import paths from stdin. Selection happens once, not
# on every test runner.
#
# By default every package runs in one job. Each extra job pays 40-60 s of
# setup (services, checkout, history, Go, caches) and Blacksmith bills every
# job rounded up to whole minutes at the same price per vCPU-minute, so the
# former shards for architecture and migrations billed one to two minutes on
# eight vCPUs for about 7 s of tests (11/2026). SPLIT_SUITES=true keeps the
# split for the nightly race run, where instrumented builds limit each runner
# to two packages at a time and one job would take hours.
set -euo pipefail

: "${FULL_SUITE:?FULL_SUITE is required}"

jq -Rsc --argjson full "$FULL_SUITE" --argjson split "${SPLIT_SUITES:-false}" '
  def job($shard; $packages; $large): {
    shard: $shard,
    packages: ($packages | join("\n")),
    runner: (if $large then "blacksmith-8vcpu-ubuntu-2404" else "blacksmith-4vcpu-ubuntu-2404" end),
    workers: (if $large then 6 else 4 end),
    # The architecture suite reads historical source blobs; a partial clone
    # would fetch them one by one.
    blobs: ($packages | any(test("/internal/architecture(/|$)")))
  };
  split("\n") | map(select(length > 0)) | unique as $all |
  ($all | map(select(test("/internal/architecture(/|$)")))) as $architecture |
  ($all | map(select(test("/database/migrations(/|$)")))) as $migrations |
  ($all - $architecture - $migrations) as $application |
  if $split | not then
    {include: [job(0; $all; ($full or ($application | length) > 16))]}
  else
    (if $full or ($application | length) > 64 then 2 else 1 end) as $parts |
    {
      include: ([
        range(0; $parts) as $part |
        ($application | to_entries | map(select(.key % $parts == $part) | .value)) as $packages |
        select(($packages | length) > 0) |
        job($part * 3; $packages; ($full or ($application | length) > 16))
      ] + [
        if ($architecture | length) > 0 then job(1; $architecture; ($full or ($application | length) > 64)) else empty end,
        if ($migrations | length) > 0 then job(2; $migrations; $full) else empty end
      ])
    } |
    # A workflow-only affected selection still needs a successful matrix job.
    if (.include | length) == 0 then {include: [job(0; []; false)]} else . end
  end
'
