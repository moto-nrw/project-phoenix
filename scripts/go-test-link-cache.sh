#!/usr/bin/env bash
# -toolexec adapter for scripts/test-backend.sh. cmd/go still owns compilation,
# vet, test execution and reporting; only successful, silent native links are
# reused. Go 1.27 does not retain these binaries when -count=1 disables its
# result cache (cmd/go/internal/work/buildid.go).
#
# The cache holds the linker's output BEFORE cmd/go rewrites its placeholder
# build ID. Always copy, never hard-link: cmd/go must mutate only its own output.
# One action per package bounds flag/source churn; eviction races are misses.
set -euo pipefail
cache_root=$1
tool=$2
shift 2
if [[ ${tool##*/} != link || ${1:-} == -V=full || ${CGO_ENABLED:-} != 0 ]]; then
  exec "$tool" "$@"
fi
args=("$@")
output= build_id= ids=0 outputs=0 imports=0 archives=0 unsafe=false
while (($#)); do
  case $1 in
    -o) output=${2:-}; outputs=$((outputs + 1)); shift ;;
    -importcfg) imports=$((imports + 1)); shift ;;
    -s|-w|-s=*|-w=*|-buildmode=exe|-buildmode=pie|-linkmode=internal|-extld=*|-X=*) ;;
    -X|-installsuffix) shift ;;
    -buildid=*) build_id=${1#*=}; ids=$((ids + 1)) ;;
    -buildid) build_id=${2:-}; ids=$((ids + 1)); shift ;;
    -*) unsafe=true ;;
    *) archives=$((archives + 1)) ;;
  esac
  if (($#)); then shift; fi
done
# Reject caller-supplied build IDs and unknown Go build-ID formats. The first
# component is cmd/go's link action hash, not a hash of temporary file paths.
if [[ $cache_root != /* || ${GO_EXTLINK_ENABLED:-} == 1 ||
  $unsafe == true || $ids != 1 || $outputs != 1 || $imports != 1 || $archives != 1 || $output != *.test ||
  ! $build_id =~ ^([A-Za-z0-9_-]{20})/[A-Za-z0-9_-]{20}/[A-Za-z0-9_-]{20}/([A-Za-z0-9_-]{20})$ ||
  ${BASH_REMATCH[1]:-} != ${BASH_REMATCH[2]:-} ||
  ! ${TOOLEXEC_IMPORTPATH:-} =~ ^[A-Za-z0-9][A-Za-z0-9._/-]*\.test$ ||
  ${TOOLEXEC_IMPORTPATH:-} == *..* ]]; then
  exec "$tool" "${args[@]}"
fi
key=${build_id%%/*}
slot="$cache_root/$TOOLEXEC_IMPORTPATH"
cached="$slot/$key.test"
if [[ -f $cached ]] && cp "$cached" "$output" 2>/dev/null; then exit 0; fi
# A disabled/unwritable cache is an ordinary link, never a test failure.
if ! mkdir -p "$slot" 2>/dev/null; then exec "$tool" "${args[@]}"; fi
pending=$(mktemp -d "$slot/.pending.XXXXXX" 2>/dev/null) || exec "$tool" "${args[@]}"
trap 'rm -rf -- "$pending"' EXIT
status=0
"$tool" "${args[@]}" >"$pending/stdout" 2>"$pending/stderr" || status=$?
cat "$pending/stdout"
cat "$pending/stderr" >&2
# Do not cache diagnostics: every invocation must retain the linker's messages.
if [[ $status == 0 && ! -s $pending/stdout && ! -s $pending/stderr ]] &&
    cp "$output" "$pending/binary" 2>/dev/null && mv -f "$pending/binary" "$cached" 2>/dev/null; then
  # Keep one link per package. Concurrent readers with an evicted key simply
  # miss; publishers use atomic rename and never modify a cached inode.
  find "$slot" -maxdepth 1 -name '*.test' ! -name "$key.test" -type f -delete 2>/dev/null || true
fi
exit "$status"
