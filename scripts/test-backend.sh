#!/usr/bin/env bash
# Backend-Testsuite mit vollem Datenbank-Lebenszyklus (ADR 0004):
# ein Run-Stempel für alle Package-Clones, gotestsum-Formatierung wenn
# verfügbar, und ein Sweep am Ende (auch bei Ctrl-C/Fehlschlag), der die
# Clones dieses Laufs droppt und verwaiste Clones toter Läufe einsammelt.
#
# `go test -tags=testhooks ./...` funktioniert weiterhin ohne dieses Skript (selbst-
# initialisierend); dann räumt erst die Generation-GC des nächsten Laufs auf.
#
# Das Leftover-Gate sitzt NICHT hier, sondern im Testprozess selbst: jedes
# Test-Binary vergleicht seinen Clone am Ende mit dem Startzustand und lässt
# das Package scheitern, wenn Zeilen in geteiltem (tenant-losem) Zustand
# übrig sind. Ein nacktes `go test -tags=testhooks ./...` ist damit genauso abgesichert.
#
# Usage: scripts/test-backend.sh [go-test-args...]     (Default: ./...)
set -euo pipefail
repo_root=$(git rev-parse --show-toplevel)
cd "$repo_root/backend"
# shellcheck source=scripts/go-test-workspace.sh
source "$repo_root/scripts/go-test-workspace.sh"

# The full-suite preset runs package binaries concurrently, but gives each Go
# process one runtime worker. On macOS this avoids expensive parallel-GC work
# in links, source ratchets and their child tools. Keep -p and -parallel below:
# this is not serial package or test execution. Explicit -cpu still controls
# the test runtime; other pinned commands and the native app loop are unchanged.
export GOMAXPROCS=1

sweep() {
  status=$?
  if ! go run ./internal/testdb/cmd/sweep && [ "$status" -eq 0 ]; then
    status=1
  fi
  [ -n "${PHX_TEST_RUN_LOCK:-}" ] && rm -f "$PHX_TEST_RUN_LOCK"
  if ! cleanup_go_test_workspace && [ "$status" -eq 0 ]; then status=1; fi
  return "$status"
}
# Stabile Run-ID pro Worktree (Cache-Hebel) plus Overlap-Lock; Details im
# Helper. Der Trap muss vor dem Bootstrap stehen, damit dessen Fehlschlag das
# gerade erworbene Lock ebenfalls aufräumt.
# shellcheck source=scripts/test-run-id.sh
source "$repo_root/scripts/test-run-id.sh"
trap sweep EXIT
prepare_go_test_workspace

# Handshake once per run instead of once per test binary: server erreichbar,
# Template zum Migrations-Hash gebaut. Die Binaries bekommen das Ergebnis
# ueber PHX_TEST_TEMPLATE und ueberspringen beide Schritte (#2419). Ein
# nacktes `go test -tags=testhooks ./...` ohne diese Zeile macht sie weiterhin selbst.
PHX_TEST_TEMPLATE=$(go run ./internal/testdb/cmd/bootstrap)
export PHX_TEST_TEMPLATE

if [ "$#" -eq 0 ]; then
  set -- ./...
fi

# Use the same conservative package budget as the changed-test loop. Heavy
# links, nested tools and a running IDE/dev loop need headroom beyond each
# package's own RSS. CLI arguments follow the defaults so an explicit -p
# still overrides this budget. Keep -parallel 8 fixed: it sizes the DB pool
# (13 connections per binary including its keeper) and is a test-cache input.
package_workers=$("$repo_root/scripts/go-package-workers.sh")
# Test failures and panic stacks use Go's runtime line tables, not DWARF.
# Omit only debugger metadata from these disposable test executables. Keep
# symbols and runtime line tables; an explicit caller -ldflags still wins.
# Apply the lower GC target to test processes, not the go test build driver.
# Preserve an explicit caller GOGC.
CONCURRENCY=(-p "$package_workers" -parallel 8 -tags=testhooks -ldflags=-w
  -exec "env GOGC=${GOGC-50}")

if go tool gotestsum --help >/dev/null 2>&1; then
  go tool gotestsum --format pkgname-and-test-fails -- "${CONCURRENCY[@]}" "$@"
else
  go test "${CONCURRENCY[@]}" "$@"
fi
