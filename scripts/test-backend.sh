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
repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root/backend"

# Stabile Run-ID pro Worktree (Cache-Hebel) plus Overlap-Lock; Details im
# Helper. Das Lock raeumt der sweep-Trap unten wieder weg.
# shellcheck source=scripts/test-run-id.sh
source "$repo_root/scripts/test-run-id.sh"

# Handshake once per run instead of once per test binary: server erreichbar,
# Template zum Migrations-Hash gebaut. Die Binaries bekommen das Ergebnis
# ueber PHX_TEST_TEMPLATE und ueberspringen beide Schritte (#2419). Ein
# nacktes `go test ./...` ohne diese Zeile macht sie weiterhin selbst.
PHX_TEST_TEMPLATE=$(go run ./internal/testdb/cmd/bootstrap)
export PHX_TEST_TEMPLATE

sweep() {
  status=$?
  if ! go run ./internal/testdb/cmd/sweep && [ "$status" -eq 0 ]; then
    status=1
  fi
  [ -n "${PHX_TEST_RUN_LOCK:-}" ] && rm -rf "$PHX_TEST_RUN_LOCK"
  return "$status"
}
trap sweep EXIT

if [ "$#" -eq 0 ]; then
  set -- ./...
fi

# Concurrency is pinned rather than left at GOMAXPROCS, because the budget
# that matters is a server-side one: `go test` runs -p package binaries at
# once and each opens a pool of (-parallel + headroom) connections plus one
# keeper, i.e. 13 per binary at -parallel 8 (backend/test/db_clone.go).
# The local postgres-test container runs max_connections=300
# (docker-compose), so 10 x 13 = 130 leaves ample headroom for lifecycle
# and maintenance sessions. CI's service container keeps PostgreSQL's stock
# 100 but never runs this script: test.yml pins its own -p 4 / -p 6.
# -parallel 8 stays pinned on purpose - -test.parallel is part of the Go
# test cache key, and a drifting value would split the cache universe.
CONCURRENCY=(-p 10 -parallel 8)

if go tool gotestsum --help >/dev/null 2>&1; then
  go tool gotestsum --format pkgname-and-test-fails -- "${CONCURRENCY[@]}" "$@"
else
  go test "${CONCURRENCY[@]}" "$@"
fi
