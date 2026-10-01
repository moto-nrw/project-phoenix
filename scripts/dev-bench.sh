#!/usr/bin/env bash
# dev-bench.sh: measure the local dev loop and test suites on this machine,
# so slow-laptop reports come with numbers. Every stage records wall time,
# CPU time and the peak memory of the whole process tree (sampled every
# 0.5 s, so short spikes can be missed); dev-loop stages add Docker
# container memory. Results print as a table and land in tmp/dev-bench/.
#
#   scripts/dev-bench.sh [--root DIR] [--simulate-ram-gb N] [stage...]
#
# Stages (default: typecheck vitest go-tests devloop-native):
#   typecheck        frontend typecheck, cold (no tsbuildinfo) and warm
#   vitest           full Vitest suite
#   go-tests         full backend suite via scripts/test-backend.sh
#   devloop-native   scripts/dev-native.sh up: backend ready, first page,
#                    Go hot reload, memory after visiting a few pages
#   devloop-docker   the same with backend and frontend in containers;
#                    main checkout only (see below)
#
# --root runs the stages against another checkout (for before/after
# comparisons from one copy of this script). --simulate-ram-gb makes
# scripts/total-memory-gb.sh report N GB, so a large machine runs the test
# scripts with the parallelism a 16 GB laptop would get.
#
# The hot-reload probe appends a comment to backend/main.go and restores
# the file from a copy afterwards.
#
# devloop-docker does not work inside `wt` worktrees: the server container
# listens on PORT from the worktree's .env while Compose maps
# SERVER_HOST_PORT:8080, and the frontend container fails env validation.
# Run that stage from the main checkout.
set -euo pipefail

die() { echo "dev-bench: $*" >&2; exit 1; }

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
SIM_RAM=""
STAGES=()
while [ $# -gt 0 ]; do
  case "$1" in
    --root) ROOT=$(cd "$2" && pwd); shift 2 ;;
    --simulate-ram-gb) SIM_RAM=$2; shift 2 ;;
    -h | --help) sed -n '2,/^set -euo/p' "$0" | sed -e '$d' -e 's/^# \{0,1\}//'; exit 0 ;;
    *) STAGES+=("$1"); shift ;;
  esac
done
[ ${#STAGES[@]} -gt 0 ] || STAGES=(typecheck vitest go-tests devloop-native)
[ -f "$ROOT/frontend/package.json" ] && [ -d "$ROOT/backend" ] || die "$ROOT is not a project-phoenix checkout"

OUT="$ROOT/tmp/dev-bench"
mkdir -p "$OUT"
STAMP=$(date +%Y%m%d-%H%M%S)
RESULTS="$OUT/$STAMP.tsv"
printf 'stage\twall_s\tcpu_s\tpeak_tree_mb\tdocker_mb\tnote\n' >"$RESULTS"

SHIM="" PROBE="" PROBE_BACKUP=""
cleanup() {
  # Restore the hot-reload probe file even when the run is interrupted.
  [ -z "$PROBE_BACKUP" ] || { cat "$PROBE_BACKUP" >"$PROBE"; rm -f "$PROBE_BACKUP"; }
  [ -z "$SHIM" ] || rm -rf "$SHIM"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

if [ -n "$SIM_RAM" ]; then
  SHIM=$(mktemp -d)
  # total-memory-gb.sh asks sysctl (macOS) first; answer with N GB.
  # shellcheck disable=SC2016 # $2 and $@ belong to the generated shim.
  printf '#!/bin/sh\n[ "$2" = hw.memsize ] && { echo %s; exit 0; }\nexec /usr/sbin/sysctl "$@"\n' \
    "$((SIM_RAM * 1024 * 1024 * 1024))" >"$SHIM/sysctl"
  chmod +x "$SHIM/sysctl"
  export PATH="$SHIM:$PATH"
fi

now() { perl -MTime::HiRes=time -e 'printf "%.2f", time'; }

# Sum RSS (KB) of a process and all its descendants.
tree_rss_kb() {
  ps -A -o pid=,ppid=,rss= | awk -v root="$1" '
    { rss[$1] = $3; kids[$2] = kids[$2] " " $1 }
    END {
      n = 1; queue[1] = root; total = 0
      for (i = 1; i <= n; i++) {
        p = queue[i]; total += rss[p]
        m = split(kids[p], c, " ")
        for (j = 1; j <= m; j++) queue[++n] = c[j]
      }
      print total
    }'
}

# Memory of this checkout's running Compose containers, in MB.
docker_mb() {
  local ids
  ids=$(cd "$ROOT" && docker compose ps -q 2>/dev/null) || { echo 0; return; }
  [ -n "$ids" ] || { echo 0; return; }
  # shellcheck disable=SC2086
  docker stats --no-stream --format '{{.MemUsage}}' $ids | awk '
    { split($1, a, /[A-Za-z]+/); v = a[1]; u = $1; gsub(/[0-9.]/, "", u)
      if (u == "GiB") v *= 1024; else if (u == "KiB" || u == "kB") v /= 1024; else if (u == "B") v /= 1048576
      t += v }
    END { printf "%d", t }'
}

record() { # stage wall cpu peak_mb docker_mb note
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$@" >>"$RESULTS"
  printf '  %-26s wall %7ss  cpu %7ss  peak %6s MB  docker %5s MB  %s\n' "$@"
}

# Run a command, sample its process tree memory, record one result row.
measure() { # stage dir command...
  local stage=$1 dir=$2 timef log start pid peak=0 kb status
  shift 2
  timef=$(mktemp)
  log="$OUT/$STAMP-$stage.log"
  start=$(now)
  (cd "$dir" && exec /usr/bin/time -p -o "$timef" "$@") >"$log" 2>&1 &
  pid=$!
  while kill -0 "$pid" 2>/dev/null; do
    kb=$(tree_rss_kb "$pid")
    [ "$kb" -gt "$peak" ] && peak=$kb
    sleep 0.5
  done
  status=0
  wait "$pid" || status=$?
  local wall cpu
  wall=$(awk -v s="$start" -v e="$(now)" 'BEGIN { printf "%.1f", e - s }')
  cpu=$(awk '/^user|^sys/ { t += $2 } END { printf "%.1f", t }' "$timef")
  rm -f "$timef"
  record "$stage" "$wall" "$cpu" "$((peak / 1024))" "-" "$([ "$status" -eq 0 ] && echo ok || echo "FAILED ($status), see $log")"
}

stage_typecheck() {
  local fe="$ROOT/frontend"
  rm -f "$fe"/*.tsbuildinfo
  measure typecheck-cold "$fe" pnpm run typecheck
  measure typecheck-warm "$fe" pnpm run typecheck
}

stage_vitest() {
  # chart.test.tsx formats numbers with the system locale.
  export LANG=en_US.UTF-8 VITE_CONFIG_NATIVE_IGNORE_WARNING=true
  measure vitest "$ROOT/frontend" pnpm vitest run
}

stage_go_tests() {
  measure go-tests "$ROOT" scripts/run-go-toolchain.sh scripts/test-backend.sh
}

wait_http() { # url timeout_s -> prints seconds waited or "timeout"
  local url=$1 limit=$2 start
  start=$(now)
  while ! curl -sf -o /dev/null --max-time 5 "$url"; do
    if awk -v s="$start" -v e="$(now)" -v l="$limit" 'BEGIN { exit !(e - s > l) }'; then
      echo timeout; return
    fi
    sleep 0.5
  done
  awk -v s="$start" -v e="$(now)" 'BEGIN { printf "%.1f", e - s }'
}

env_port() { sed -n "s/^$1=//p" "$ROOT/.env" | tail -1; }

# Shared dev-loop probe. $1 = mode, $2 = start command (string), $3 = stop
# command, $4 = command printing the rebuild count of the Go server.
devloop() {
  local mode=$1 start_cmd=$2 stop_cmd=$3 count_cmd=$4
  local api fe t0 ready page pages reload before
  api="http://localhost:$(env_port SERVER_HOST_PORT)"
  fe="http://localhost:$(env_port FRONTEND_HOST_PORT)"
  PROBE="$ROOT/backend/main.go"

  t0=$(now)
  (cd "$ROOT" && eval "$start_cmd") >"$OUT/$STAMP-$mode.log" 2>&1 &
  local runner=$!
  ready=$(wait_http "$api/health" 600)
  page=$(curl -s -o /dev/null -w '%{time_total}' --max-time 300 "$fe/login" || echo fail)
  pages=0
  for p in / /login /dashboard /students /activities /rooms /help /settings; do
    curl -s -o /dev/null --max-time 120 "$fe$p" && pages=$((pages + 1))
  done

  # Go hot reload: edit a comment, wait for the rebuild counter to move.
  before=$(eval "$count_cmd")
  PROBE_BACKUP=$(mktemp)
  cp "$PROBE" "$PROBE_BACKUP"
  echo "// dev-bench probe $(date +%s)" >>"$PROBE"
  local r0 n
  r0=$(now)
  reload=timeout
  for _ in $(seq 1 240); do
    n=$(eval "$count_cmd")
    if [ "${n:-0}" -gt "${before:-0}" ] && curl -sf -o /dev/null --max-time 2 "$api/health"; then
      reload=$(awk -v s="$r0" -v e="$(now)" 'BEGIN { printf "%.1f", e - s }')
      break
    fi
    sleep 0.5
  done
  cat "$PROBE_BACKUP" >"$PROBE"
  rm -f "$PROBE_BACKUP"
  PROBE_BACKUP=""

  local host_kb=0 pidf
  for pidf in "$ROOT"/tmp/dev-native/*.pid; do
    [ -f "$pidf" ] && host_kb=$((host_kb + $(tree_rss_kb "$(cat "$pidf")")))
  done
  local dmb
  dmb=$(docker_mb)
  local total
  total=$(awk -v s="$t0" -v e="$(now)" 'BEGIN { printf "%.1f", e - s }')
  record "$mode" "$total" "-" "$((host_kb / 1024))" "$dmb" \
    "backend ready ${ready}s, first page ${page}s, ${pages}/8 pages, Go reload ${reload}s"

  (cd "$ROOT" && eval "$stop_cmd") >/dev/null 2>&1 || true
  kill "$runner" 2>/dev/null || true
  wait "$runner" 2>/dev/null || true
}

stage_devloop_native() {
  [ -x "$ROOT/scripts/dev-native.sh" ] || { record devloop-native - - - - "skipped: no scripts/dev-native.sh"; return; }
  devloop devloop-native \
    "scripts/dev-native.sh up" \
    "scripts/dev-native.sh down" \
    "grep -c 'running\\.\\.\\.' '$ROOT/tmp/dev-native/backend.log' 2>/dev/null || true"
}

stage_devloop_docker() {
  # Naming the services works with and without the `full` profile.
  devloop devloop-docker \
    "docker compose up -d --wait postgres mailpit && docker compose up -d server frontend" \
    "docker compose stop server frontend" \
    "docker compose logs server 2>/dev/null | grep -c 'running\\.\\.\\.' || true"
}

echo "dev-bench: $(git -C "$ROOT" log -1 --format='%h %s' | cut -c1-70)"
echo "dev-bench: $(uname -sm), $( (sysctl -n hw.ncpu 2>/dev/null || nproc) ) CPUs, $("$ROOT/scripts/total-memory-gb.sh" 2>/dev/null || echo '?') GB reported${SIM_RAM:+ (simulated)}"
for s in "${STAGES[@]}"; do
  case "$s" in
    typecheck) stage_typecheck ;;
    vitest) stage_vitest ;;
    go-tests) stage_go_tests ;;
    devloop-native) stage_devloop_native ;;
    devloop-docker) stage_devloop_docker ;;
    *) die "unknown stage $s" ;;
  esac
done
echo "dev-bench: results in $RESULTS"
