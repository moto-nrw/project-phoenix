#!/usr/bin/env bash
# Rolls out a new Caddyfile on the app server (#5098). The host Caddy serves
# production, staging, Grafana and PyrePortal, so every step either leaves the
# running config untouched or restores it.
#
# Usage: deploy-caddy-remote.sh NEW_CADDYFILE PREVIOUS_CADDYFILE KNOWN_SHA256S
#   PREVIOUS_CADDYFILE  repository version before this commit, for the drift diff
#   KNOWN_SHA256S       sha256 of every version of the file on main, one per line
# 0=deployed or already current, 1=aborted before the live file changed,
# 10=new config failed and the backup is live again, 11=restoring failed.
set -Eeuo pipefail
(( $# == 3 )) || { echo 'Usage: deploy-caddy-remote.sh NEW_CADDYFILE PREVIOUS_CADDYFILE KNOWN_SHA256S' >&2; exit 1; }
new=$1 previous=$2 known=$3
caddy_dir=${CADDY_DIR:-/etc/caddy}
live="$caddy_dir/Caddyfile"
env_file="$caddy_dir/cloudflare.env"
[ -s "$new" ] && [ -f "$previous" ] && [ -s "$known" ] || { echo 'New Caddyfile, previous version or known hashes missing' >&2; exit 1; }
[ -f "$live" ] && [ -f "$env_file" ] || { echo "$live or $env_file missing" >&2; exit 1; }

if cmp -s "$new" "$live"; then
  echo 'Caddyfile is already current; nothing to do'
  exit 0
fi

# A live file that matches no repository version was edited on the server.
# Overwriting it would silently drop that edit, so it has to reach the
# repository first.
live_sha=$(sha256sum "$live" | cut -d' ' -f1)
if ! grep -qxF "$live_sha" "$known"; then
  echo "$live differs from every repository version; it was edited on the server. Diff against the previous repository version:" >&2
  diff -u --label repository "$previous" --label "$live" "$live" >&2 || true
  echo 'Not deployed. Bring the server edit into deploy/caddy/Caddyfile first.' >&2
  exit 1
fi

# The Cloudflare DNS module needs its token to provision, as under systemd.
# shellcheck disable=SC1090 # server-only file, never in the repository
if ! (set -a; . "$env_file"; set +a; caddy validate --config "$new" --adapter caddyfile); then
  echo 'caddy validate rejected the new Caddyfile; the live file was not touched' >&2
  exit 1
fi

backup="$live.bak-$(date -u +%Y%m%dT%H%M%SZ)"
cp -p "$live" "$backup"

healthy() {
  systemctl is-active --quiet caddy &&
    curl -fsS --max-time 5 -o /dev/null http://localhost:2019/config/
}

restore() {
  trap - ERR
  echo "Rollout failed; restoring $backup" >&2
  if install -m 644 "$backup" "$live" && systemctl reload caddy && healthy; then
    echo 'Previous Caddyfile restored and Caddy is healthy' >&2
    exit 10
  fi
  echo "Restoring $backup failed; check systemctl status caddy and journalctl -u caddy" >&2
  exit 11
}
trap restore ERR

install -m 644 "$new" "$live"
# caddy reload exits non-zero when the new config does not load.
systemctl reload caddy
healthy
trap - ERR
echo "Caddyfile deployed; previous version: $backup"
