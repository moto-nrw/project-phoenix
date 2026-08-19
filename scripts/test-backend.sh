#!/usr/bin/env bash
# Backend-Testsuite mit vollem Datenbank-Lebenszyklus (ADR 0004):
# ein Run-Stempel für alle Package-Clones, gotestsum-Formatierung wenn
# verfügbar, und ein Sweep am Ende (auch bei Ctrl-C/Fehlschlag), der die
# Clones dieses Laufs droppt und verwaiste Clones toter Läufe einsammelt.
#
# `go test ./...` funktioniert weiterhin ohne dieses Skript (selbst-
# initialisierend); dann räumt erst die Generation-GC des nächsten Laufs auf.
#
# Usage: scripts/test-backend.sh [go-test-args...]     (Default: ./...)
#   PHX_TEST_LEFTOVERS=1 scripts/test-backend.sh       # Restdaten-Diagnose
set -uo pipefail
cd "$(git rev-parse --show-toplevel)/backend"

PHX_TEST_RUN_ID=$(od -An -N6 -tx1 /dev/urandom | tr -d ' \n')
export PHX_TEST_RUN_ID

sweep() {
  go run ./internal/testdb/cmd/sweep
}
trap sweep EXIT

if [ "$#" -eq 0 ]; then
  set -- ./...
fi

if go tool gotestsum --help >/dev/null 2>&1; then
  go tool gotestsum --format pkgname-and-test-fails -- "$@"
else
  go test "$@"
fi
