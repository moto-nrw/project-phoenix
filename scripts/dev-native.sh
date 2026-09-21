#!/usr/bin/env bash
# dev-native.sh: run the backend (air) and frontend (next dev) on the host
# while postgres and mailpit keep running in Docker Compose.
#
# Both processes read the same root .env that Compose uses, so there is one
# source of configuration. Only values that name Compose hostnames are
# rewritten to localhost with the published ports. In a wt worktree the
# copied .env already carries that worktree's ports, so several checkouts
# can run side by side.
#
#   scripts/dev-native.sh up                 start infra + backend + frontend (foreground)
#   scripts/dev-native.sh down               stop native backend/frontend (infra keeps running)
#   scripts/dev-native.sh status             show native process and infra state
#   scripts/dev-native.sh backend <cmd...>   run a command in backend/ with the native env
#   scripts/dev-native.sh frontend <cmd...>  run a command in frontend/ with the native env
#   scripts/dev-native.sh env                list exported keys (no values)
#   scripts/dev-native.sh docker             stop native processes, start the full Compose stack
#
# `devbox run dev <command>` is the same thing with the pinned toolchain.
#
# Why .env and not backend/dev.env: parts of the backend read os.Getenv
# directly (CORS, rate limits, OPERATOR_*, ADMIN_*) and never see dev.env.
set -euo pipefail

die() { echo "dev-native: $*" >&2; exit 1; }

ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || die "not inside a git repository"
[ -f "$ROOT/backend/.air.toml" ] && [ -f "$ROOT/frontend/package.json" ] ||
  die "$ROOT is not a project-phoenix checkout"
[ -f "$ROOT/.env" ] || die "$ROOT/.env missing (run scripts/setup-dev.sh)"
STATE="$ROOT/tmp/dev-native"

load_env() {
  local line key val
  while IFS= read -r line || [ -n "$line" ]; do
    case "$line" in '' | '#'*) continue ;; esac
    line=${line#export }
    key=${line%%=*}
    val=${line#*=}
    [[ $key =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || continue
    if [[ $val == \"*\" || $val == \'*\' ]]; then val=${val:1:${#val}-2}; fi
    export "$key=$val"
  done <"$ROOT/.env"

  local k
  for k in SERVER_HOST_PORT FRONTEND_HOST_PORT POSTGRES_HOST_PORT MAILPIT_HOST_PORT MAILPIT_SMTP_HOST_PORT DB_DSN; do
    [ -n "${!k:-}" ] || die "$k missing in .env"
  done

  # Host overrides: Compose hostnames -> localhost with published ports.
  [[ $DB_DSN == *@postgres:5432/* ]] || die "DB_DSN in .env does not use host postgres:5432; adjust scripts/dev-native.sh"
  export DB_DSN=${DB_DSN/@postgres:5432\//@localhost:$POSTGRES_HOST_PORT/}
  export EMAIL_SMTP_HOST=localhost
  export EMAIL_SMTP_PORT=$MAILPIT_SMTP_HOST_PORT
  export MAILPIT_URL=http://localhost:$MAILPIT_HOST_PORT
  export API_URL=http://localhost:$SERVER_HOST_PORT
  export PORT=$SERVER_HOST_PORT
}

require_tools() {
  local t
  for t in "$@"; do
    command -v "$t" >/dev/null || die "$t not found; run inside the Devbox environment (devbox run dev ... or direnv)"
  done
}

kill_tree() {
  local pid=$1 child
  for child in $(pgrep -P "$pid" 2>/dev/null); do kill_tree "$child"; done
  kill "$pid" 2>/dev/null || true
}

stop_native() {
  local name pidfile
  for name in backend frontend; do
    pidfile="$STATE/$name.pid"
    [ -f "$pidfile" ] || continue
    kill_tree "$(cat "$pidfile")"
    rm -f "$pidfile"
    echo "stopped $name"
  done
}

port_free() {
  ! lsof -nP -iTCP:"$1" -sTCP:LISTEN >/dev/null 2>&1
}

start_svc() {
  local name=$1 dir=$2
  shift 2
  : >"$STATE/$name.log"
  (cd "$ROOT/$dir" && exec "$@") \
    > >(tee -a "$STATE/$name.log" | sed -u "s/^/[$name] /") 2>&1 &
  echo $! >"$STATE/$name.pid"
}

cmd_up() {
  require_tools go air pnpm docker lsof
  load_env
  mkdir -p "$STATE"
  stop_native

  cd "$ROOT"
  docker compose stop server frontend >/dev/null 2>&1 || true
  docker compose up -d --wait postgres mailpit

  local p
  for p in "$SERVER_HOST_PORT" "$FRONTEND_HOST_PORT"; do
    port_free "$p" || die "port $p is in use: $(lsof -nP -iTCP:"$p" -sTCP:LISTEN | tail -n +2 | awk '{print $1, $2}' | head -3 | tr '\n' ' ')"
  done

  # Same sequence as the Compose server command: migrate, then air.
  start_svc backend backend bash -c 'go run . migrate && exec air -c .air.toml'
  PORT=$FRONTEND_HOST_PORT start_svc frontend frontend pnpm dev

  trap 'echo; stop_native; exit 130' INT TERM
  echo "dev-native: backend http://localhost:$SERVER_HOST_PORT  frontend http://localhost:$FRONTEND_HOST_PORT  mailpit http://localhost:$MAILPIT_HOST_PORT"
  echo "dev-native: logs in $STATE/{backend,frontend}.log"

  local name
  while true; do
    for name in backend frontend; do
      if ! kill -0 "$(cat "$STATE/$name.pid" 2>/dev/null || echo 0)" 2>/dev/null; then
        echo "dev-native: $name exited, stopping the rest" >&2
        stop_native
        exit 1
      fi
    done
    sleep 2
  done
}

cmd_status() {
  local name pid
  for name in backend frontend; do
    pid=$(cat "$STATE/$name.pid" 2>/dev/null || true)
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      echo "$name: running (pid $pid)"
    else
      echo "$name: stopped"
    fi
  done
  (cd "$ROOT" && docker compose ps --format '{{.Service}}: {{.State}}')
}

cmd_in() {
  local dir=$1
  shift
  [ $# -gt 0 ] || die "usage: scripts/dev-native.sh $dir <command...>"
  load_env
  [ "$dir" = frontend ] && export PORT=$FRONTEND_HOST_PORT
  cd "$ROOT/$dir"
  exec "$@"
}

# Wrapped in main so bash parses everything before running: editing this
# file while `up` runs cannot derail the running instance.
main() {
  case "${1:-}" in
    up) cmd_up ;;
    down) stop_native ;;
    status) cmd_status ;;
    backend | frontend) cmd_in "$@" ;;
    env) load_env && export -p | sed -nE 's/^declare -x ([A-Za-z_][A-Za-z0-9_]*).*/\1/p' | sort ;;
    docker)
      stop_native
      cd "$ROOT" && docker compose up -d
      ;;
    *) sed -n '10,17p' "$0" | sed 's/^# \{0,1\}//'; exit 2 ;;
  esac
}

main "$@"
exit
