#!/usr/bin/env bash
# Backend-Testsuite mit vollem Datenbank-Lebenszyklus (ADR 0004):
# ein Run-Stempel für alle Package-Clones, gotestsum-Formatierung wenn
# verfügbar, und ein Sweep am Ende (auch bei Ctrl-C/Fehlschlag), der die
# Clones dieses Laufs droppt und verwaiste Clones toter Läufe einsammelt.
#
# `go test ./...` funktioniert weiterhin ohne dieses Skript (selbst-
# initialisierend); dann räumt erst die Generation-GC des nächsten Laufs auf.
#
# Das Leftover-Gate sitzt NICHT hier, sondern im Testprozess selbst: jedes
# Test-Binary vergleicht seinen Clone am Ende mit dem Startzustand und lässt
# das Package scheitern, wenn Zeilen in geteiltem (tenant-losem) Zustand
# übrig sind. Ein nacktes `go test ./...` ist damit genauso abgesichert.
#
# Usage: scripts/test-backend.sh [go-test-args...]     (Default: ./...)
set -euo pipefail
cd "$(git rev-parse --show-toplevel)/backend"

PHX_TEST_RUN_ID=$(od -An -N6 -tx1 /dev/urandom | tr -d ' \n')
export PHX_TEST_RUN_ID

sweep() {
  status=$?
  if ! go run ./internal/testdb/cmd/sweep && [ "$status" -eq 0 ]; then
    status=1
  fi
  return "$status"
}
trap sweep EXIT

if [ "$#" -eq 0 ]; then
  set -- ./...
fi

# Concurrency is pinned rather than left at GOMAXPROCS, because the budget
# that matters is a server-side one: `go test` runs -p package binaries at
# once and each opens a pool of (-parallel + headroom) connections plus one
# keeper. With these values that is 4 x 13 = 52 connections, which fits the
# 100 a stock postgres:17 allows — the CI service container runs on that
# default. Raising either number without raising max_connections trades test
# failures for "too many clients" errors.
CONCURRENCY=(-p 4 -parallel 8)

if go tool gotestsum --help >/dev/null 2>&1; then
  go tool gotestsum --format pkgname-and-test-fails -- "${CONCURRENCY[@]}" "$@"
else
  go test "${CONCURRENCY[@]}" "$@"
fi
