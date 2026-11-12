#!/usr/bin/env bash
# Keeps a restored Go build cache to the entries one run actually uses.
#
#   go-cache-sweep.sh mark  MARKER DIR...   before the run
#   go-cache-sweep.sh sweep MARKER DIR...   after the run
#
# A CI job that restores its cache by key prefix and saves it again carries
# every older entry forward: Go deletes an entry only after five days without
# use, and a restored cache is always fresh. The backend build cache grew from
# 2.2 GB to 9.65 GB in twelve hours of development pushes, and restoring it
# took 75 s instead of 15 s in every pull request (#6784).
#
# Go refreshes an entry's mtime when it uses it, if the mtime
# is at least an hour old (markUsed in cmd/go/internal/cache). "mark" backdates
# every entry and stamps MARKER; "sweep" deletes the entries the run did not
# touch or write since. Only the entry directories 00..ff are touched, never
# the README, trim.txt or the fuzz corpus.
# Do not use this for golangci-lint's analyzer cache: cached issue results
# skip dependency analysis, leaving reusable dependency facts unread.
set -euo pipefail

usage() {
  echo "usage: $0 mark|sweep MARKER DIR..." >&2
  exit 2
}

[ "$#" -ge 3 ] || usage
command=$1
marker=$2
shift 2

entries() {
  local dir=$1
  [ -d "$dir" ] || return 0
  find "$dir" -mindepth 2 -maxdepth 2 -path "$dir/[0-9a-f][0-9a-f]/*" "${@:2}"
}

case "$command" in
  mark)
    for dir in "$@"; do
      # 2000-01-01, far past the one-hour refresh threshold.
      entries "$dir" -exec touch -m -t 200001010000 {} +
    done
    touch "$marker"
    ;;
  sweep)
    [ -f "$marker" ] || { echo "missing marker $marker; run mark first" >&2; exit 1; }
    for dir in "$@"; do
      [ -d "$dir" ] || continue
      before=$(du -sm "$dir" | cut -f1)
      entries "$dir" ! -newer "$marker" -exec rm -rf {} +
      after=$(du -sm "$dir" | cut -f1)
      echo "$dir: ${before} MB -> ${after} MB"
    done
    ;;
  *)
    usage
    ;;
esac
