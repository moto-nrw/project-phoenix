#!/usr/bin/env bash
# Exercise cmd/go's real mtime refresh: after mark, a build of one package and
# sweep, that package must stay fully cached and the other one must be gone.
set -euo pipefail
repo_root=$(git rev-parse --show-toplevel)
sweep="$repo_root/scripts/go-cache-sweep.sh"
fixture=$(mktemp -d)
trap 'rm -rf -- "$fixture"' EXIT
export GOCACHE="$fixture/cache" GOFLAGS='' CGO_ENABLED=0 GOTOOLCHAIN=local
version=$(awk '$1 == "go" {print $2; exit}' "$repo_root/backend/go.mod")
mkdir -p "$fixture/mod/kept" "$fixture/mod/dropped"
printf 'module sweepprobe\n\ngo %s\n' "$version" >"$fixture/mod/go.mod"
printf 'package kept\nfunc Value() int { return 1 }\n' >"$fixture/mod/kept/kept.go"
printf 'package dropped\nfunc Value() int { return 2 }\n' >"$fixture/mod/dropped/dropped.go"
mkdir -p "$GOCACHE/fuzz/sweepprobe"
printf 'corpus' >"$GOCACHE/fuzz/sweepprobe/seed"

compiles() { (cd "$fixture/mod" && go build -x "./$1" 2>&1 | grep -c '/compile ' || true); }
fail() { echo "FAIL: $*" >&2; exit 1; }

(cd "$fixture/mod" && go build ./kept ./dropped)
[ "$(compiles kept)" = 0 ] || fail "kept is not cached after the first build"

"$sweep" mark "$fixture/marker" "$GOCACHE" "$fixture/absent"
(cd "$fixture/mod" && go build ./kept)
"$sweep" sweep "$fixture/marker" "$GOCACHE" "$fixture/absent" >/dev/null

[ "$(compiles kept)" = 0 ] || fail "sweep removed entries the build used"
[ -f "$GOCACHE/fuzz/sweepprobe/seed" ] || fail "sweep touched the fuzz corpus"
[ "$(compiles dropped)" -ge 1 ] || fail "sweep kept entries the build did not use"

if "$sweep" sweep "$fixture/missing-marker" "$GOCACHE" 2>/dev/null; then
  fail "sweep without a marker must fail"
fi
echo "go-cache-sweep: ok"
