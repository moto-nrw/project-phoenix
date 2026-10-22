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
#   scripts/dev-native.sh up [backend|frontend] start infra + selected apps (default: both)
#   scripts/dev-native.sh down               stop native backend/frontend (infra keeps running)
#   scripts/dev-native.sh status             show native process and infra state
#   scripts/dev-native.sh backend <cmd...>   run a command in backend/ with the native env
#   scripts/dev-native.sh frontend <cmd...>  run a command in frontend/ with the native env
#   scripts/dev-native.sh env                list exported keys (no values)
#   scripts/dev-native.sh docker             stop native processes, start the full Compose stack
#
# `devbox run dev <command>` is the same thing with the pinned toolchain.
# Every `up` replaces both managed native apps, even when selecting only one.
# Shared postgres/mailpit stay running; frontend-only skips Go and migrations.
# Backend children default to GOFLAGS=-p=4 and GOMAXPROCS=4; caller values win.
#
# Why .env and not backend/dev.env: parts of the backend read os.Getenv
# directly (CORS, rate limits, OPERATOR_*, ADMIN_*) and never see dev.env.
set -euo pipefail

die() { echo "dev-native: $*" >&2; exit 1; }
usage() { sed -n '10,22p' "$0" | sed 's/^# \{0,1\}//'; }

ROOT=$(git rev-parse --show-toplevel 2>/dev/null) || die "not inside a git repository"
[ -f "$ROOT/backend/.air.toml" ] && [ -f "$ROOT/frontend/package.json" ] ||
  die "$ROOT is not a project-phoenix checkout"
[ -f "$ROOT/.env" ] || die "$ROOT/.env missing (run scripts/setup-dev.sh)"
STATE="$ROOT/tmp/dev-native"

# Compose interpolates ${VAR} in .env values (DB_DSN carries
# ${POSTGRES_PASSWORD}); do the same so the native apps see the value the
# containers see. Single-quoted values stay literal, as in Compose. Both ${VAR}
# and $VAR are expanded; $$ is the Compose escape for a literal $.
# Defaults (${VAR:-x}) are not allowed in this repo, so any other braced form,
# and a reference to an unset variable, fails here instead of reaching the apps
# as text. Sets EXPANDED; no subshell, so die stops the script.
expand_refs() {
  local key=$1 val=$2 out='' ref char next i=0
  while [ "$i" -lt "${#val}" ]; do
    char=${val:i:1}
    if [ "$char" != '$' ]; then
      out+=$char
      i=$((i + 1))
      continue
    fi

    next=${val:i+1:1}
    if [ "$next" = '$' ]; then
      out+='$'
      i=$((i + 2))
    elif [ "$next" = '{' ]; then
      ref=${val:i+2}
      ref=${ref%%\}*}
      [[ ${val:i+2} == *\}* && $ref =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] ||
        die "$key in .env uses an unsupported \${...} form; only \${VAR} is resolved"
      [ -n "${!ref+set}" ] || die "$key in .env references \${$ref}, which is not set"
      out+=${!ref}
      i=$((i + ${#ref} + 3))
    elif [[ $next =~ [A-Za-z_] ]]; then
      ref=${val:i+1}
      ref=${ref%%[^A-Za-z0-9_]*}
      [ -n "${!ref+set}" ] || die "$key in .env references \$$ref, which is not set"
      out+=${!ref}
      i=$((i + ${#ref} + 1))
    else
      out+='$'
      i=$((i + 1))
    fi
  done
  EXPANDED=$out
}

load_env() {
  local line key val shell_env_keys=$'\n'
  while IFS= read -r key; do shell_env_keys+="$key"$'\n'; done < <(compgen -e)
  while IFS= read -r line || [ -n "$line" ]; do
    case "$line" in '' | '#'*) continue ;; esac
    line=${line#export }
    key=${line%%=*}
    val=${line#*=}
    [[ $key =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || continue
    [[ $shell_env_keys == *$'\n'"$key"$'\n'* ]] && continue
    if [[ $val == \'*\' ]]; then
      val=${val:1:${#val}-2}
    else
      if [[ $val == \"*\" ]]; then val=${val:1:${#val}-2}; fi
      expand_refs "$key" "$val"
      val=$EXPANDED
    fi
    export "$key=$val"
  done <"$ROOT/.env"

  local k
  for k in SERVER_HOST_PORT FRONTEND_HOST_PORT POSTGRES_HOST_PORT MAILPIT_HOST_PORT MAILPIT_SMTP_HOST_PORT DB_DSN; do
    [ -n "${!k:-}" ] || die "$k missing in .env"
  done

  # Host overrides: Compose hostnames -> localhost with published ports.
  if [[ $DB_DSN == *@postgres:5432/* ]]; then
    export DB_DSN=${DB_DSN/@postgres:5432\//@localhost:$POSTGRES_HOST_PORT/}
  elif [[ $shell_env_keys != *$'\nDB_DSN\n'* ]]; then
    die "DB_DSN in .env does not use host postgres:5432; adjust scripts/dev-native.sh"
  fi
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

# A replaced supervisor must not stop the new loop's processes. The arrays
# belong to cmd_up; Bash's dynamic scope keeps its signal handler on that loop.
stop_started() {
  local i name pidfile pid
  for i in "${!services[@]}"; do
    name=${services[$i]}
    pid=${started_pids[$i]:-}
    pidfile="$STATE/$name.pid"
    [ -n "$pid" ] && [ "$(cat "$pidfile" 2>/dev/null || true)" = "$pid" ] || continue
    kill_tree "$pid"
    [ "$(cat "$pidfile" 2>/dev/null || true)" != "$pid" ] || rm -f "$pidfile"
    echo "stopped $name"
  done
}

cmd_up() {
  local services=(backend frontend) started_pids=() name p i
  [ $# -le 1 ] || die "usage: scripts/dev-native.sh up [backend|frontend]"
  case "${1:-}" in
    '') ;;
    backend | frontend) services=("$1") ;;
    *) die "unknown service '$1'; use backend or frontend" ;;
  esac
  require_tools docker lsof
  for name in "${services[@]}"; do
    case "$name" in
      backend) require_tools go air ;;
      frontend) require_tools node ;;
    esac
  done
  load_env
  mkdir -p "$STATE"
  stop_native

  cd "$ROOT"
  docker compose stop server frontend >/dev/null 2>&1 || true
  docker compose up -d --wait postgres mailpit

  for name in "${services[@]}"; do
    case "$name" in
      backend) p=$SERVER_HOST_PORT ;;
      frontend) p=$FRONTEND_HOST_PORT ;;
    esac
    port_free "$p" || die "port $p is in use: $(lsof -nP -iTCP:"$p" -sTCP:LISTEN | tail -n +2 | awk '{print $1, $2}' | head -3 | tr '\n' ' ')"
  done

  trap 'echo; stop_started; exit 130' INT TERM
  # Same sequence as the Compose server command: migrate, then air. Building
  # air's binary first and migrating with it avoids a second full compile
  # (`go run` links its own copy); air's first build is then a cache hit.
  for name in "${services[@]}"; do
    case "$name" in
      backend)
        GOFLAGS=${GOFLAGS--p=4} GOMAXPROCS=${GOMAXPROCS-4} \
          start_svc backend backend bash -c 'go build -ldflags="-s -w" -o ./tmp/main . && ./tmp/main migrate && exec air -c .air.toml'
        echo "dev-native: backend http://localhost:$SERVER_HOST_PORT"
        ;;
      frontend)
        PORT=$FRONTEND_HOST_PORT start_svc frontend frontend node --max-http-header-size=65536 node_modules/next/dist/bin/next dev
        echo "dev-native: frontend http://localhost:$FRONTEND_HOST_PORT"
        ;;
    esac
    started_pids+=("$(cat "$STATE/$name.pid")")
  done

  echo "dev-native: mailpit http://localhost:$MAILPIT_HOST_PORT"
  echo "dev-native: logs in $STATE/{backend,frontend}.log"

  while true; do
    for i in "${!services[@]}"; do
      name=${services[$i]}
      if [ "$(cat "$STATE/$name.pid" 2>/dev/null || true)" != "${started_pids[$i]}" ]; then
        echo "dev-native: managed loop replaced or stopped"
        stop_started
        return
      fi
      if ! kill -0 "${started_pids[$i]}" 2>/dev/null; then
        echo "dev-native: $name exited, stopping the rest" >&2
        stop_started
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
  if [ "$dir" = frontend ]; then
    export PORT=$FRONTEND_HOST_PORT
  else
    export GOFLAGS=${GOFLAGS--p=4} GOMAXPROCS=${GOMAXPROCS-4}
  fi
  cd "$ROOT/$dir"
  exec "$@"
}

# Wrapped in main so bash parses everything before running: editing this
# file while `up` runs cannot derail the running instance.
main() {
  case "${1:-}" in
    -h | --help) usage ;;
    up) shift; cmd_up "$@" ;;
    down) stop_native ;;
    status) cmd_status ;;
    backend | frontend) cmd_in "$@" ;;
    env) load_env && export -p | sed -nE 's/^declare -x ([A-Za-z_][A-Za-z0-9_]*).*/\1/p' | sort ;;
    docker)
      stop_native
      cd "$ROOT" && docker compose --profile full up -d
      ;;
    *) usage; exit 2 ;;
  esac
}

main "$@"
exit
