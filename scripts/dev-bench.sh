#!/usr/bin/env bash
# dev-bench.sh: measure the local dev loop, test suites and a small bank of
# representative agent tasks on this machine. Every step records wall time,
# heavy-slot queue time, CPU time, the peak memory of the whole process tree
# (sampled every 0.5 s, so short spikes can be missed), the checkout's disk
# growth and its exit status; dev-loop steps add Docker container memory.
# Results print as a table and land in tmp/dev-bench/ with a metadata file
# (revision, dirty diff, toolchain, load, heavy-slot state).
#
#   scripts/dev-bench.sh [--root DIR] [--heavy] [--repeat N]
#                        [--simulate-ram-gb N] [stage...]
#   scripts/dev-bench.sh [--root DIR] [--heavy] [--repeat N] -- NAME COMMAND...
#
# Stages (default: typecheck vitest go-tests devloop-native):
#   typecheck        frontend typecheck, cold (no tsbuildinfo) and warm
#   vitest           full Vitest suite
#   go-tests         full backend suite via scripts/test-backend.sh
#   devloop-native   scripts/dev-native.sh up: backend ready, first page,
#                    Go hot reload, memory after visiting a few pages
#   devloop-docker   the same with backend and frontend in containers;
#                    main checkout only (see below)
# Task bank (each runs once after a fresh source change, then unchanged):
#   fresh-worktree   detached worktree of HEAD + scripts/bootstrap-dev.sh,
#                    then devbox doctor; removed again afterwards
#   go-leaf          probe in DEV_BENCH_GO_LEAF (few importers), then
#                    scripts/test-changed.sh --fast HEAD
#   go-central       probe in DEV_BENCH_GO_CENTRAL (many importers), same
#   fe-change        probe comment in DEV_BENCH_FE_FILE, then
#                    scripts/test-changed.sh --fast HEAD and pnpm run check
#   browser-flow     DEV_BENCH_E2E_SPEC through the seeded native stack,
#                    which must already run (scripts/dev-native.sh up)
#
# `-- NAME COMMAND...` measures one arbitrary command from the checkout root.
#
# --root runs the stages against another checkout (for before/after
# comparisons from one copy of this script). --heavy runs each measured
# command through ~/bin/agent-heavy (or $AGENT_HEAVY) and reports the time
# spent waiting for a slot separately from the run. --repeat N measures each
# stage N times for medians. --simulate-ram-gb makes
# scripts/total-memory-gb.sh report N GB, so a large machine runs the test
# scripts with the parallelism a 16 GB laptop would get; it does not create
# real memory pressure.
#
# Exit status is 1 when any measured command failed or a requested stage
# could not run, so a green exit means every row is a valid measurement.
# Change stages need a clean checkout: their test selection is the diff
# against HEAD. Probes never edit tracked files except DEV_BENCH_FE_FILE, which must be
# unmodified and is restored afterwards, also on interrupt. The Go probes and
# the hot-reload probe are new untracked files that are removed again.
#
# devloop-docker does not work inside `wt` worktrees: the server container
# listens on PORT from the worktree's .env while Compose maps
# SERVER_HOST_PORT:8080, and the frontend container fails env validation.
# Run that stage from the main checkout.
set -euo pipefail

die() { echo "dev-bench: $*" >&2; exit 1; }

ROOT=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
SIM_RAM="" HEAVY="" REPEAT=1
STAGES=() CUSTOM=()
while [ $# -gt 0 ]; do
  case "$1" in
    --root) ROOT=$(cd "$2" && pwd); shift 2 ;;
    --simulate-ram-gb) SIM_RAM=$2; shift 2 ;;
    --heavy) HEAVY=${AGENT_HEAVY:-$HOME/bin/agent-heavy}; shift ;;
    --repeat) REPEAT=$2; shift 2 ;;
    --) shift; CUSTOM=("$@"); break ;;
    -h | --help) sed -n '2,/^set -euo/p' "$0" | sed -e '$d' -e 's/^# \{0,1\}//'; exit 0 ;;
    *) STAGES+=("$1"); shift ;;
  esac
done
[[ "$REPEAT" =~ ^[1-9][0-9]*$ ]] || die "--repeat needs a positive integer"
[ -z "$HEAVY" ] || [ -x "$HEAVY" ] || die "--heavy: $HEAVY is not executable"
if [ ${#CUSTOM[@]} -gt 0 ]; then
  [ ${#CUSTOM[@]} -ge 2 ] || die "-- needs NAME COMMAND..."
  [ ${#STAGES[@]} -eq 0 ] || die "name stages or -- NAME COMMAND, not both"
  STAGES=(custom)
fi
[ ${#STAGES[@]} -gt 0 ] || STAGES=(typecheck vitest go-tests devloop-native)
[ -f "$ROOT/frontend/package.json" ] && [ -d "$ROOT/backend" ] || die "$ROOT is not a project-phoenix checkout"

GO_LEAF=${DEV_BENCH_GO_LEAF:-modules/workforce/http/timetracking}
GO_CENTRAL=${DEV_BENCH_GO_CENTRAL:-sharedkernel/calendar}
FE_FILE=${DEV_BENCH_FE_FILE:-src/lib/date-helpers.ts}
E2E_SPEC=${DEV_BENCH_E2E_SPEC:-e2e/planung-navigation.spec.ts}

OUT="$ROOT/tmp/dev-bench"
mkdir -p "$OUT"
STAMP=$(date +%Y%m%d-%H%M%S)
RESULTS="$OUT/$STAMP.tsv"
META="$OUT/$STAMP.meta"
printf 'stage\twall_s\tqueue_s\tcpu_s\tpeak_tree_mb\tdisk_mb\tdocker_mb\tstatus\tnote\n' >"$RESULTS"
FAILURES=0

SHIM="" MEASURED="" PROBE_FILES=() RESTORE_SRC="" RESTORE_BACKUP="" TSBUILD_BACKUP="" FRESH=""
# TERM a process and all its descendants (an interrupted measured command).
kill_tree() {
  local pids
  pids=$(ps -A -o pid=,ppid= | awk -v root="$1" '
    { kids[$2] = kids[$2] " " $1 }
    END { n = 1; q[1] = root; for (i = 1; i <= n; i++) { print q[i]; m = split(kids[q[i]], c, " "); for (j = 1; j <= m; j++) q[++n] = c[j] } }')
  # shellcheck disable=SC2086
  kill -TERM $pids 2>/dev/null || true
}

cleanup() {
  local f
  if [ -n "$MEASURED" ]; then kill_tree "$MEASURED"; wait "$MEASURED" 2>/dev/null || true; fi
  # Remove untracked probes, restore the edited frontend file and the
  # incremental TypeScript state even when the run is interrupted.
  for f in ${PROBE_FILES[@]+"${PROBE_FILES[@]}"}; do rm -f "$f"; done
  if [ -n "$RESTORE_BACKUP" ]; then cat "$RESTORE_BACKUP" >"$RESTORE_SRC"; rm -f "$RESTORE_BACKUP"; fi
  if [ -n "$TSBUILD_BACKUP" ]; then
    for f in "$TSBUILD_BACKUP"/*.tsbuildinfo; do
      [ -e "$f" ] && [ ! -e "$ROOT/frontend/${f##*/}" ] && mv "$f" "$ROOT/frontend/"
    done
    rm -rf "$TSBUILD_BACKUP"
  fi
  if [ -n "$FRESH" ]; then git -C "$ROOT" worktree remove --force "$FRESH" >/dev/null 2>&1 || true; fi
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
# Digits that differ on every call, for probe contents (BSD date lacks %N).
unique() { perl -MTime::HiRes=time -e 'printf "%d", time * 1e6'; }
since() { awk -v s="$1" -v e="$(now)" 'BEGIN { printf "%.1f", e - s }'; }
load1() { uptime | sed -E 's/.*load averages?: *([0-9.]+).*/\1/'; }

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

# Apparent size of a directory in KB. APFS clones and hardlinks count fully,
# so this is an upper bound for physically allocated space.
disk_kb() { du -sk "$1" 2>/dev/null | awk '{ print $1 + 0 }'; }

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

record() { # stage wall queue cpu peak_mb disk_mb docker_mb status note
  printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$@" >>"$RESULTS"
  printf '  %-24s wall %7ss  queue %5ss  cpu %7ss  peak %6s MB  disk %6s MB  %s %s\n' \
    "$1" "$2" "$3" "$4" "$5" "$6" "$8" "$9"
}

fail_row() { # stage reason
  FAILURES=$((FAILURES + 1))
  record "$1" - - - - - - FAILED "$2"
}

# Run a command, sample its process tree memory, record one result row.
# NOTE_FN, when set, names a function that prints a note from the log.
measure() { # stage dir command...
  local stage=$1 dir=$2 timef admitf log start pid peak=0 kb status disk0 load0
  shift 2
  timef=$(mktemp) admitf=$(mktemp)
  log="$OUT/$STAMP-$stage.log"
  disk0=$(disk_kb "$ROOT")
  load0=$(load1)
  start=$(now)
  if [ -n "$HEAVY" ] && [ -z "${LIGHT:-}" ]; then
    # The slot is held from the admit timestamp until the command exits.
    # shellcheck disable=SC2016 # expanded by the inner shell.
    (cd "$dir" && exec "$HEAVY" -- /bin/sh -c \
      'perl -MTime::HiRes=time -e "printf q(%.2f), time" >"$1"; shift; exec "$@"' \
      sh "$admitf" /usr/bin/time -p -o "$timef" "$@") >"$log" 2>&1 </dev/null &
  else
    printf '%s' "$start" >"$admitf"
    (cd "$dir" && exec /usr/bin/time -p -o "$timef" "$@") >"$log" 2>&1 </dev/null &
  fi
  pid=$!
  MEASURED=$pid
  while kill -0 "$pid" 2>/dev/null; do
    kb=$(tree_rss_kb "$pid")
    [ "$kb" -gt "$peak" ] && peak=$kb
    sleep 0.5
  done
  status=0
  wait "$pid" || status=$?
  MEASURED=""
  local end wall queue cpu disk note admit
  end=$(now)
  admit=$(cat "$admitf")
  [ -n "$admit" ] || admit=$end
  wall=$(awk -v s="$admit" -v e="$end" 'BEGIN { printf "%.1f", e - s }')
  queue=$(awk -v s="$start" -v a="$admit" 'BEGIN { printf "%.1f", a - s }')
  cpu=$(awk '/^user|^sys/ { t += $2 } END { printf "%.1f", t }' "$timef")
  disk=$(awk -v a="$disk0" -v b="$(disk_kb "$ROOT")" 'BEGIN { printf "%.0f", (b - a) / 1024 }')
  rm -f "$timef" "$admitf"
  note="load ${load0}"
  if [ -n "${NOTE_FN:-}" ]; then note="$note, $("$NOTE_FN" "$log")"; fi
  if [ "$status" -eq 0 ]; then
    record "$stage" "$wall" "$queue" "$cpu" "$((peak / 1024))" "$disk" - ok "$note"
  else
    FAILURES=$((FAILURES + 1))
    record "$stage" "$wall" "$queue" "$cpu" "$((peak / 1024))" "$disk" - "FAILED($status)" "$note, see $log"
  fi
}

write_meta() {
  {
    echo "stamp=$STAMP"
    echo "root=$ROOT"
    echo "revision=$(git -C "$ROOT" rev-parse HEAD)"
    echo "subject=$(git -C "$ROOT" log -1 --format=%s | cut -c1-80)"
    echo "dirty_files=$(git -C "$ROOT" status --porcelain | wc -l | tr -d ' ')"
    echo "dirty_diff_sha=$(git -C "$ROOT" diff HEAD | git hash-object --stdin)"
    echo "machine=$(uname -sm), $( (sysctl -n hw.ncpu 2>/dev/null || nproc) ) CPUs, $("$ROOT/scripts/total-memory-gb.sh" 2>/dev/null || echo '?') GB reported${SIM_RAM:+ (simulated)}"
    echo "load_start=$(load1)"
    echo "heavy=${HEAVY:-off}"
    [ -z "$HEAVY" ] || echo "heavy_status=$("$HEAVY" --status 2>/dev/null || echo unknown)"
    echo "go=$(cd "$ROOT/backend" && go version 2>/dev/null || echo missing)"
    echo "node=$(node --version 2>/dev/null || echo missing) pnpm=$(cd "$ROOT/frontend" && pnpm --version 2>/dev/null || echo missing)"
    echo "go_package_workers=$("$ROOT/scripts/go-package-workers.sh" 2>/dev/null || echo '?')"
    echo "stages=${STAGES[*]} repeat=$REPEAT"
    [ ${#CUSTOM[@]} -eq 0 ] || echo "custom=${CUSTOM[*]}"
  } >"$META"
}

stage_typecheck() {
  local fe="$ROOT/frontend" f
  # Cold means no incremental state. Keep the developer's files aside and put
  # them back if the cold run fails before writing new ones.
  TSBUILD_BACKUP=$(mktemp -d)
  for f in "$fe"/*.tsbuildinfo; do [ -e "$f" ] && mv "$f" "$TSBUILD_BACKUP/"; done
  measure typecheck-cold "$fe" pnpm run typecheck
  measure typecheck-warm "$fe" pnpm run typecheck
  rm -rf "$TSBUILD_BACKUP"
  TSBUILD_BACKUP=""
}

stage_vitest() {
  # chart.test.tsx formats numbers with the system locale.
  export LANG=en_US.UTF-8 VITE_CONFIG_NATIVE_IGNORE_WARNING=true
  measure vitest "$ROOT/frontend" pnpm vitest run
}

stage_go_tests() {
  measure go-tests "$ROOT" scripts/run-go-toolchain.sh scripts/test-backend.sh
}

# Selected and cached Go packages, frontend mode, from a test-changed log.
changed_note() {
  local pkgs ok cached fe
  pkgs=$(sed -nE 's/.*==> go test \(([0-9]+) affected packages.*/\1/p' "$1" | tail -1)
  ok=$(grep -cE '^ok[[:space:]]' "$1" || true)
  cached=$(grep -cE '^ok[[:space:]].*\(cached\)' "$1" || true)
  fe=$(sed -nE 's/^==> (vitest[^(]*|frontend: keine).*/\1/p' "$1" | head -1)
  echo "go pkgs ${pkgs:-0}, ok ${ok} (cached ${cached}), fe ${fe:-?}"
}

# Write an untracked probe into a Go package so its build and test cache keys
# change, like a real edit; the function body differs on every run.
go_probe() { # package dir relative to backend/
  local dir="$ROOT/backend/$1" pkg probe
  [ -d "$dir" ] || return 1
  for probe in "$dir"/*.go; do
    case "$probe" in *_test.go) continue ;; esac
    pkg=$(sed -nE 's/^package ([A-Za-z0-9_]+)$/\1/p' "$probe" | head -1)
    [ -z "$pkg" ] || break
  done
  [ -n "${pkg:-}" ] || return 1
  probe="$dir/zz_dev_bench_probe.go"
  PROBE_FILES+=("$probe")
  # init() keeps the value in the linked binary. Go caches test results by the
  # binary's content, and the linker drops an unreferenced function, so such a
  # probe let every package after the first run in a worktree hit the cache.
  printf 'package %s\n\n// Generated by scripts/dev-bench.sh; removed after the measurement.\nvar devBenchProbe int64\n\nfunc init() { devBenchProbe = %s }\n' \
    "$pkg" "$(unique)" >"$probe"
}

# Change stages select tests from the diff against HEAD, so any other local
# change would alter what they measure.
clean_tree() { # stage
  [ -z "$(git -C "$ROOT" status --porcelain)" ] && return 0
  fail_row "$1" "uncommitted changes would change the test selection; commit or stash them first"
  return 1
}

go_change_stage() { # stage package
  local stage=$1 dir=$2 run
  clean_tree "$stage" || return 0
  for run in change repeat; do
    if [ "$run" = change ] && ! go_probe "$dir"; then fail_row "$stage" "no Go package in backend/$dir"; return; fi
    NOTE_FN=changed_note measure "$stage-$run" "$ROOT" \
      scripts/run-go-toolchain.sh scripts/test-changed.sh --fast HEAD
  done
  rm -f "${PROBE_FILES[@]}"
}

stage_go_leaf() { go_change_stage go-leaf "$GO_LEAF"; }
stage_go_central() { go_change_stage go-central "$GO_CENTRAL"; }

stage_fe_change() {
  local src="$ROOT/frontend/$FE_FILE" run
  [ -f "$src" ] || { fail_row fe-change "missing frontend/$FE_FILE"; return; }
  git -C "$ROOT" diff --quiet HEAD -- "frontend/$FE_FILE" ||
    { fail_row fe-change "frontend/$FE_FILE has local changes; not probing it"; return; }
  clean_tree fe-change || return 0
  RESTORE_SRC=$src
  RESTORE_BACKUP=$(mktemp)
  cp "$src" "$RESTORE_BACKUP"
  for run in change repeat; do
    [ "$run" = change ] && printf '\n// dev-bench probe %s\n' "$(unique)" >>"$src"
    NOTE_FN=changed_note measure "fe-change-tests-$run" "$ROOT" \
      scripts/run-go-toolchain.sh scripts/test-changed.sh --fast HEAD
    measure "fe-change-check-$run" "$ROOT/frontend" pnpm run check
  done
  cat "$RESTORE_BACKUP" >"$src"
  rm -f "$RESTORE_BACKUP"
  RESTORE_BACKUP=""
}

fresh_note() { echo "fresh worktree $(disk_kb "$FRESH" | awk '{ printf "%d", $1 / 1024 }') MB apparent"; }

stage_fresh_worktree() {
  FRESH="$OUT/fresh-$STAMP"
  git -C "$ROOT" worktree add --detach --quiet "$FRESH" HEAD
  NOTE_FN=fresh_note measure fresh-bootstrap "$FRESH" devbox run bootstrap
  LIGHT=1 measure fresh-doctor "$FRESH" devbox run doctor
  git -C "$ROOT" worktree remove --force "$FRESH"
  FRESH=""
}

e2e_note() {
  local passed skipped
  passed=$(sed -nE 's/^ *([0-9]+) passed.*/\1/p' "$1" | tail -1)
  skipped=$(sed -nE 's/^ *([0-9]+) skipped.*/\1/p' "$1" | tail -1)
  echo "passed ${passed:-0}, skipped ${skipped:-0}"
}

stage_browser_flow() {
  local fe_port
  fe_port=$(env_port FRONTEND_HOST_PORT)
  curl -sf -o /dev/null --max-time 5 "http://localhost:${fe_port:-3000}/help" ||
    { fail_row browser-flow "no frontend on ${fe_port:-3000}; start scripts/dev-native.sh up and seed first"; return; }
  # A run in which every test skipped did not exercise the flow.
  NOTE_FN=e2e_note measure browser-flow "$ROOT" \
    scripts/dev-native.sh frontend pnpm exec playwright test "$E2E_SPEC" --reporter=list
  if [ "$(tail -1 "$RESULTS" | cut -f8)" = ok ] &&
    ! grep -qE '^ *[1-9][0-9]* passed' "$OUT/$STAMP-browser-flow.log"; then
    FAILURES=$((FAILURES + 1))
    record browser-flow - - - - - - FAILED "no test passed (missing seed state?)"
  fi
}

stage_custom() {
  local name=${CUSTOM[0]}
  measure "$name" "$ROOT" "${CUSTOM[@]:1}"
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
  since "$start"
}

env_port() { sed -n "s/^$1=//p" "$ROOT/.env" 2>/dev/null | tail -1; }

# Shared dev-loop probe. $1 = mode, $2 = start command (string), $3 = stop
# command, $4 = command printing the rebuild count of the Go server.
devloop() {
  local mode=$1 start_cmd=$2 stop_cmd=$3 count_cmd=$4
  local api fe t0 ready page pages reload before probe
  api="http://localhost:$(env_port SERVER_HOST_PORT)"
  fe="http://localhost:$(env_port FRONTEND_HOST_PORT)"

  t0=$(now)
  (cd "$ROOT" && eval "$start_cmd") >"$OUT/$STAMP-$mode.log" 2>&1 &
  local runner=$!
  ready=$(wait_http "$api/health" 600)
  page=$(curl -s -o /dev/null -w '%{time_total}' --max-time 300 "$fe/login" || echo fail)
  pages=0
  for p in / /login /dashboard /students /activities /rooms /help /settings; do
    curl -s -o /dev/null --max-time 120 "$fe$p" && pages=$((pages + 1))
  done

  # Go hot reload: add an untracked file to package main, wait for the
  # rebuild counter to move, then remove it again.
  before=$(eval "$count_cmd")
  probe="$ROOT/backend/zz_dev_bench_probe.go"
  PROBE_FILES+=("$probe")
  printf 'package main\n\n// Generated by scripts/dev-bench.sh.\nfunc devBenchProbe() int64 { return %s }\n' "$(unique)" >"$probe"
  local r0 n
  r0=$(now)
  reload=timeout
  for _ in $(seq 1 240); do
    n=$(eval "$count_cmd")
    if [ "${n:-0}" -gt "${before:-0}" ] && curl -sf -o /dev/null --max-time 2 "$api/health"; then
      reload=$(since "$r0")
      break
    fi
    sleep 0.5
  done
  rm -f "$probe"

  local host_kb=0 pidf
  for pidf in "$ROOT"/tmp/dev-native/*.pid; do
    [ -f "$pidf" ] && host_kb=$((host_kb + $(tree_rss_kb "$(cat "$pidf")")))
  done
  local dmb status=ok
  dmb=$(docker_mb)
  if [ "$ready" = timeout ] || [ "$reload" = timeout ] || [ "$page" = fail ]; then
    status=FAILED
    FAILURES=$((FAILURES + 1))
  fi
  record "$mode" "$(since "$t0")" - - "$((host_kb / 1024))" - "$dmb" "$status" \
    "backend ready ${ready}s, first page ${page}s, ${pages}/8 pages, Go reload ${reload}s"

  (cd "$ROOT" && eval "$stop_cmd") >/dev/null 2>&1 || true
  kill "$runner" 2>/dev/null || true
  wait "$runner" 2>/dev/null || true
}

stage_devloop_native() {
  [ -x "$ROOT/scripts/dev-native.sh" ] || { fail_row devloop-native "no scripts/dev-native.sh"; return; }
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

write_meta
echo "dev-bench: $(git -C "$ROOT" log -1 --format='%h %s' | cut -c1-70)"
echo "dev-bench: $(sed -n 's/^machine=//p' "$META"), load $(load1)${HEAVY:+, heavy slots via $HEAVY}"
for _ in $(seq 1 "$REPEAT"); do
  for s in "${STAGES[@]}"; do
    case "$s" in
      typecheck) stage_typecheck ;;
      vitest) stage_vitest ;;
      go-tests) stage_go_tests ;;
      devloop-native) stage_devloop_native ;;
      devloop-docker) stage_devloop_docker ;;
      fresh-worktree) stage_fresh_worktree ;;
      go-leaf) stage_go_leaf ;;
      go-central) stage_go_central ;;
      fe-change) stage_fe_change ;;
      browser-flow) stage_browser_flow ;;
      custom) stage_custom ;;
      *) die "unknown stage $s" ;;
    esac
  done
done
echo "load_end=$(load1)" >>"$META"
echo "dev-bench: results in $RESULTS (metadata $META)"
if [ "$FAILURES" -gt 0 ]; then
  echo "dev-bench: $FAILURES measurement(s) failed; the table is not a valid baseline" >&2
  exit 1
fi
