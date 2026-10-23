#!/usr/bin/env bash
# Deploys named Grafana provisioning files to the monitoring host and restarts
# Grafana. Only the named files change: dashboards that exist only on the host
# stay, and alert rules from the repository are not switched on by accident.
# The previous versions go to /root/monitoring/backups first.
#
# Usage: deploy-grafana-provisioning.sh [--dry-run] <ssh-host> <file>...
#   <file>  path below monitoring/grafana/provisioning,
#           e.g. dashboards/json/phoenix-error-codes.json
# 0 = deployed and Grafana healthy; 1 = invalid input or a failed check.
# The remote commands expand on this side on purpose: host and file names are
# validated below before any of them reaches ssh.
# shellcheck disable=SC2029
set -Eeuo pipefail

remote_dir=/root/monitoring
remote_provisioning=$remote_dir/grafana/provisioning
grafana_health=http://127.0.0.1:3003/api/health

usage() {
  echo 'Usage: deploy-grafana-provisioning.sh [--dry-run] <ssh-host> <file>...' >&2
  exit 1
}

dry_run=false
if [[ "${1:-}" == --dry-run ]]; then dry_run=true; shift; fi
(( $# >= 2 )) || usage
host=$1
shift
[[ "$host" =~ ^[A-Za-z0-9._@-]+$ ]] || { echo 'Invalid ssh host' >&2; exit 1; }

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
local_provisioning=$script_dir/../monitoring/grafana/provisioning
for file in "$@"; do
  if [[ ! "$file" =~ ^(alerting|dashboards|datasources)/[A-Za-z0-9._/-]+\.(json|ya?ml)$ || "$file" == *..* ]]; then
    echo "Not a provisioning file: $file" >&2
    exit 1
  fi
  [[ -f "$local_provisioning/$file" ]] || { echo "Missing in the repository: $file" >&2; exit 1; }
  if [[ "$file" == *.json ]] && ! jq empty "$local_provisioning/$file" 2>/dev/null; then
    echo "Invalid JSON: $file" >&2
    exit 1
  fi
done

for file in "$@"; do
  echo "== $file"
  if current=$(ssh "$host" "test -f $remote_provisioning/$file && cat $remote_provisioning/$file"); then
    diff -u --label "$host" --label repository \
      <(printf '%s\n' "$current") "$local_provisioning/$file" || true
  else
    echo "new file"
  fi
done

if [[ "$dry_run" == true ]]; then
  echo 'Dry run: nothing copied, Grafana not restarted.'
  exit 0
fi

backup=$remote_dir/backups/grafana-provisioning-$(date -u +%Y%m%dT%H%M%SZ)
ssh "$host" "set -e; umask 077; mkdir -p $backup; cd $remote_provisioning; for f in $*; do if [ -e \"\$f\" ]; then cp -p --parents \"\$f\" $backup/; fi; done"
echo "Backup: $host:$backup"

restore="Restore: ssh $host 'cp -a $backup/. $remote_provisioning/ && cd $remote_dir && docker compose restart grafana'; files that were new are not in the backup and must be removed by hand"

for file in "$@"; do
  target=$remote_provisioning/$file
  ssh "$host" "mkdir -p \"\$(dirname $target)\" && cat > $target.tmp && chmod 644 $target.tmp && mv $target.tmp $target" \
    < "$local_provisioning/$file"
done

since=$(ssh "$host" 'date -u +%Y-%m-%dT%H:%M:%SZ')
ssh "$host" "cd $remote_dir && docker compose restart grafana"
if ! ssh "$host" "for i in \$(seq 1 30); do curl -fsS $grafana_health >/dev/null 2>&1 && exit 0; sleep 2; done; exit 1"; then
  echo 'Grafana is not healthy 60 seconds after the restart' >&2
  echo "$restore" >&2
  exit 1
fi

if ! log=$(ssh "$host" "cd $remote_dir && docker compose logs --no-log-prefix --since $since grafana"); then
  echo 'Could not read the Grafana log after the restart; provisioning is unchecked' >&2
  echo "$restore" >&2
  exit 1
fi
# The host has no plugins provisioning directory; Grafana reports that on
# every start. Any other missing directory is an error.
errors=$(printf '%s\n' "$log" | grep 'logger=provisioning' | grep -E 'level=(error|crit)' \
  | grep -Fv 'error="open /etc/grafana/provisioning/plugins: no such file or directory"' || true)
if [[ -n "$errors" ]]; then
  echo 'Grafana reported provisioning errors:' >&2
  echo "$errors" >&2
  echo "$restore" >&2
  exit 1
fi
echo 'Grafana is healthy and provisioned without errors.'
