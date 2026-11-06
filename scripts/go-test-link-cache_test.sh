#!/usr/bin/env bash
# Exercise cmd/go's real link keys, not a duplicate cache-key implementation.
set -euo pipefail
repo_root=$(git rev-parse --show-toplevel)
fixture=$(mktemp -d)
trap 'rm -rf -- "$fixture"' EXIT
mkdir -p "$fixture/bin" "$fixture/dep"
export CACHE_PROBE_ROOT="$fixture/cache" CACHE_PROBE_HELPER="$repo_root/scripts/go-test-link-cache.sh"
export CACHE_PROBE_LOG="$fixture/links" CACHE_PROBE_BIN="$fixture/bin" CACHE_PROBE_EXEC="$fixture/runs"
export CGO_ENABLED=0 GOMAXPROCS=1
export CACHE_PROBE_KEY_TOOL="$fixture/link-key"
version=$(awk '$1 == "go" {print $2; exit}' "$repo_root/backend/go.mod")
printf 'module linkcacheprobe\n\ngo %s\n' "$version" >"$fixture/go.mod"
printf 'package dep\nconst Value = "one"\n' >"$fixture/dep/value.go"
cat >"$fixture/value.go" <<'EOF'
package linkcacheprobe
import("embed"; "linkcacheprobe/dep")
//go:embed asset.txt
var asset embed.FS
var LinkValue = "default"
func Value() string { b,_:=asset.ReadFile("asset.txt"); return dep.Value+":"+string(b)+":"+Tag+":"+LinkValue }
EOF
printf 'asset-one' >"$fixture/asset.txt"
printf '//go:build !cacheprobe\n\npackage linkcacheprobe\nconst Tag = "plain"\n' >"$fixture/plain.go"
printf '//go:build cacheprobe\n\npackage linkcacheprobe\nconst Tag = "tagged"\n' >"$fixture/tagged.go"
cat >"$fixture/probe_test.go" <<'EOF'
package linkcacheprobe
import("testing"; "os"; "fmt")
func TestFresh(t *testing.T) {
 t.Parallel()
 f,e:=os.OpenFile(os.Getenv("CACHE_PROBE_EXEC"),os.O_APPEND|os.O_WRONLY|os.O_CREATE,0600)
 if e!=nil { t.Fatal(e) }; defer f.Close()
 fmt.Fprintln(f,Value()); t.Log("value:"+Value())
 if os.Getenv("CACHE_PROBE_FAIL")=="1" { t.Fatal("cache assertion") }
 if os.Getenv("CACHE_PROBE_PANIC")=="1" { panic("cache panic") }
}
EOF
cat >"$fixture/tool" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ ${1##*/} == link ]]; then
 export CACHE_PROBE_REAL_LINK=$1
 shift
 exec "$CACHE_PROBE_HELPER" "$CACHE_PROBE_ROOT" "$CACHE_PROBE_KEY_TOOL" "$CACHE_PROBE_BIN/link" "$@"
fi
exec "$@"
EOF
cat >"$fixture/bin/link" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ ${1:-} == -V=full ]]; then
 version=$("$CACHE_PROBE_REAL_LINK" "$@")
 printf '%s%s\n' "$version" "${CACHE_PROBE_TOOL_ID:-}"
 exit
fi
echo link >>"$CACHE_PROBE_LOG"
if [[ ${CACHE_PROBE_LINK_FAIL:-} == 1 ]]; then echo 'linker failure evidence' >&2; exit 27; fi
if [[ ${CACHE_PROBE_LINK_WARNING:-} == 1 ]]; then echo 'linker warning evidence' >&2; fi
exec "$CACHE_PROBE_REAL_LINK" "$@"
EOF
chmod +x "$fixture/tool" "$fixture/bin/link"
go_bin=$(command -v go)
"$go_bin" build -o "$CACHE_PROBE_KEY_TOOL" "$repo_root/scripts/go-test-link-key.go"
cd "$fixture"
links() { if [[ -f $CACHE_PROBE_LOG ]]; then wc -l <"$CACHE_PROBE_LOG" | tr -d ' '; else echo 0; fi; }
executions() { if [[ -f $CACHE_PROBE_EXEC ]]; then wc -l <"$CACHE_PROBE_EXEC" | tr -d ' '; else echo 0; fi; }
run() {
 local expected_exit=$1 expected_links=$2 before actual_exit=0
 shift 2
 before=$(links)
 "$go_bin" test -trimpath -count=1 -v -ldflags=-w -toolexec="$fixture/tool" "$@" . >"$fixture/output" 2>&1 || actual_exit=$?
 if [[ $actual_exit != "$expected_exit" || $(links) != $((before + expected_links)) ]]; then
   cat "$fixture/output" >&2
   echo "expected exit $expected_exit and $expected_links new links; got $actual_exit and $(($(links) - before))" >&2
   exit 1
 fi
}
run 0 1
cached=$(find "$fixture/cache" -name '*.test' -type f)
checksum=$(cksum <"$cached")
run 0 0
[[ $(executions) == 2 && $(cksum <"$cached") == "$checksum" ]]
# Comment-only production and test edits must also relink. The next warm run
# must hit even though cmd/go moved cold objects from temporary to cache paths.
printf '\n// production source changed\n' >> dep/value.go
run 0 1
run 0 0
printf '\n// test source changed\n' >> probe_test.go
run 0 1
run 0 0
sed 's/one/two/' dep/value.go > dep/new
mv dep/new dep/value.go
run 0 1
grep -q 'value:two:asset-one:plain' "$fixture/output"
sed 's/t.Parallel()/t.Parallel(); t.Log("test-source-edit")/' probe_test.go > new
mv new probe_test.go
run 0 1
grep -q test-source-edit "$fixture/output"
printf 'asset-two' >asset.txt
run 0 1
grep -q 'value:two:asset-two:plain' "$fixture/output"
run 0 1 -ldflags='-s -w'
run 0 0 -ldflags='-s -w'
run 0 1 -tags=cacheprobe
grep -q 'asset-two:tagged' "$fixture/output"
CACHE_PROBE_TOOL_ID=' cache-probe-toolchain-2' run 0 1 -tags=cacheprobe
CACHE_PROBE_TOOL_ID=' cache-probe-toolchain-2' run 0 0 -tags=cacheprobe
run 0 1 -coverprofile="$fixture/coverage-one"
run 0 0 -coverprofile="$fixture/coverage-two"
cmp "$fixture/coverage-one" "$fixture/coverage-two"
grep -q 'value.go' "$fixture/coverage-two"
CACHE_PROBE_FAIL=1 run 1 0 -coverprofile="$fixture/coverage-three"
grep -Eq 'probe_test.go:[0-9]+: cache assertion' "$fixture/output"
CACHE_PROBE_PANIC=1 run 1 0 -coverprofile="$fixture/coverage-four"
grep -q 'panic: cache panic' "$fixture/output"
grep -Eq 'probe_test.go:[0-9]+' "$fixture/output"
# CGO and race keep the native linker path, including repeated executions.
CGO_ENABLED=1 run 0 1 -race
CGO_ENABLED=1 run 0 1 -race
# Side-effecting linker flags must run even when Go's link key is unchanged.
run 0 1 -ldflags="-w -cpuprofile=$fixture/link.pprof"
[[ -s $fixture/link.pprof ]]
rm "$fixture/link.pprof"
run 0 1 -ldflags="-w -cpuprofile=$fixture/link.pprof"
[[ -s $fixture/link.pprof ]]
CACHE_PROBE_LINK_FAIL=1 run 1 1 -ldflags='-w -X linkcacheprobe.LinkValue=failure-probe'
grep -q 'linker failure evidence' "$fixture/output"
run 0 1 -ldflags='-w -X linkcacheprobe.LinkValue=failure-probe'
run 0 0 -ldflags='-w -X linkcacheprobe.LinkValue=failure-probe'
CACHE_PROBE_LINK_WARNING=1 run 0 1 -ldflags='-w -X linkcacheprobe.LinkValue=warning-probe'
grep -q 'linker warning evidence' "$fixture/output"
CACHE_PROBE_LINK_WARNING=1 run 0 1 -ldflags='-w -X linkcacheprobe.LinkValue=warning-probe'
grep -q 'linker warning evidence' "$fixture/output"
# A caller-owned build ID and an unavailable cache never become cache hits.
run 0 1 -ldflags='-w -buildid=caller-owned'
run 0 1 -ldflags='-w -buildid=caller-owned'
printf 'not-a-directory' >"$fixture/unavailable"
CACHE_PROBE_ROOT="$fixture/unavailable" run 0 1
CACHE_PROBE_ROOT="$fixture/unavailable" run 0 1
# Vet must reject a new invalid source even after a green cached link.
cat > bad.go <<'EOF'
package linkcacheprobe
import "fmt"
func bad() { fmt.Printf("%d", "not-an-integer") }
EOF
vet_exit=0
"$go_bin" test -trimpath -count=1 -toolexec="$fixture/tool" . >"$fixture/output" 2>&1 || vet_exit=$?
[[ $vet_exit != 0 ]]
grep -q 'fmt.Printf format %d' "$fixture/output"
printf 'link cache: source/test/embed/flags/tags/toolchain invalidation, fresh execution, coverage/race, diagnostics and vet passed (%s links, %s test executions)\n' "$(links)" "$(executions)"

# Exercise the full-suite wrapper with a real Go test and stub only its DB
# lifecycle. Unchanged runs must retain cmd/go's result cache as well as links.
result_fixture="$fixture/results"
mkdir -p "$result_fixture"/{backend,scripts,bin,tmp}
git -C "$result_fixture" init -q
for helper in go-test-workspace.sh test-run-id.sh go-test-link-cache.sh go-test-link-key.go; do
  cp "$repo_root/scripts/$helper" "$result_fixture/scripts/$helper"
done
printf '#!/bin/sh\necho 1\n' >"$result_fixture/scripts/go-package-workers.sh"
chmod +x "$result_fixture/scripts/go-package-workers.sh"
printf 'module resultcacheprobe\n\ngo %s\n' "$version" >"$result_fixture/backend/go.mod"
cat >"$result_fixture/backend/probe_test.go" <<'EOF'
package resultcacheprobe
import("testing"; "os"; "runtime/debug"; "strconv")
func TestGCTarget(t *testing.T) {
 want, err := strconv.Atoi(os.Getenv("GOGC"))
 if err != nil { t.Fatal(err) }
 if got := debug.SetGCPercent(-1); got != want { t.Fatalf("GOGC = %d, want %d", got, want) }
 t.Logf("GC target: %d", want)
}
EOF
cat >"$result_fixture/bin/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
if [[ ${1:-} == run ]]; then
 case ${2:-} in
  ./internal/testdb/cmd/bootstrap) echo fixture; exit 0 ;;
  ./internal/testdb/cmd/sweep) exit 0 ;;
 esac
fi
if [[ ${1:-} == tool && ${2:-} == gotestsum ]]; then exit 1; fi
exec "$RESULT_CACHE_GO" "$@"
EOF
chmod +x "$result_fixture/bin/go"
export RESULT_CACHE_GO="$go_bin"
result_run() {
  (cd "$result_fixture" && PATH="$result_fixture/bin:$PATH" TMPDIR="$result_fixture/tmp" \
    bash "$repo_root/scripts/test-backend.sh" -v "$@" .) >"$result_fixture/output" 2>&1 || {
      cat "$result_fixture/output" >&2; return 1;
    }
}
unset GOGC
result_run
grep -q 'GC target: 50' "$result_fixture/output"
result_run
grep -q '(cached)' "$result_fixture/output"
GOGC=73 result_run
grep -q 'GC target: 73' "$result_fixture/output"
GOGC=73 result_run
grep -q '(cached)' "$result_fixture/output"
result_run -count=1
if grep -q '(cached)' "$result_fixture/output"; then
  echo '-count=1 must execute the test again' >&2
  exit 1
fi
echo 'full-suite wrapper: default/explicit GOGC, cached reruns and -count=1 passed'
