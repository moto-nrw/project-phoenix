#!/usr/bin/env bash
# Teardown that `wt remove` runs for a managed worktree (.wtconfig). Removes
# only the Docker resources of the worktree's own Compose project, the
# COMPOSE_PROJECT_NAME that wt wrote to .wt.env:
#   - every container labelled with that project, from every profile;
#   - networks and volumes labelled with that project whose name starts with
#     "<project>_", so Compose created them for this project alone.
# Resources with a fixed name (the shared Go and Next caches, fixed networks)
# stay even when this project created them: other projects use them by name.
# Test database servers (project-phoenix-testdb-<port>) belong to the test
# harness and are shared by every checkout on that port; they stay too.
#
#   scripts/worktree-teardown.sh [--dry-run]
#
# --dry-run lists what would be removed. Prints resource names only.
set -euo pipefail
cd "$(dirname "$0")/.."

dry_run=false
case "${1:-}" in
  '') ;;
  --dry-run) dry_run=true ;;
  *) echo "usage: scripts/worktree-teardown.sh [--dry-run]" >&2; exit 2 ;;
esac

project=
[ -f .wt.env ] && project=$(sed -n 's/^COMPOSE_PROJECT_NAME=//p' .wt.env | tail -n 1)
if [ -z "$project" ]; then
  echo "worktree-teardown: no COMPOSE_PROJECT_NAME in .wt.env; nothing removed" >&2
  exit 0
fi
case "$project" in
  project-phoenix-testdb-*)
    echo "worktree-teardown: $project is a shared test database server; nothing removed" >&2
    exit 1 ;;
esac
if ! [[ "$project" =~ ^[a-z0-9][a-z0-9_-]*$ ]]; then
  echo "worktree-teardown: invalid COMPOSE_PROJECT_NAME in .wt.env; nothing removed" >&2
  exit 1
fi

label="label=com.docker.compose.project=$project"
own() { awk -v prefix="${project}_" 'index($0, prefix) == 1'; }
containers=$(docker ps -a --filter "$label" --format '{{.Names}}')
networks=$(docker network ls --filter "$label" --format '{{.Name}}' | own)
volumes=$(docker volume ls --filter "$label" --format '{{.Name}}' | own)

if [ "$dry_run" = true ]; then
  for name in $containers; do echo "would remove container $name"; done
  for name in $networks; do echo "would remove network $name"; done
  for name in $volumes; do echo "would remove volume $name"; done
  exit 0
fi

# Docker names contain no whitespace, so word splitting is safe here.
status=0
[ -z "$containers" ] || docker rm --force --volumes $containers >/dev/null || status=1
for name in $networks; do docker network rm "$name" >/dev/null || status=1; done
for name in $volumes; do docker volume rm "$name" >/dev/null || status=1; done
[ "$status" -eq 0 ] && echo "worktree-teardown: removed Compose project $project"
exit "$status"
