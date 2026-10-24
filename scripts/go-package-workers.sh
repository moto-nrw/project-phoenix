#!/usr/bin/env bash
# Shared package-build/test budget. Reserve CPU and RAM for the editor, native
# dev loop and nested Go tools; macOS also needs a lower clone-I/O ceiling.
set -euo pipefail
script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)

cpu_count=$(getconf _NPROCESSORS_ONLN)
if ! [[ "$cpu_count" =~ ^[1-9][0-9]*$ ]]; then
  echo "getconf returned an invalid CPU count: $cpu_count" >&2
  exit 1
fi
memory_gb=$("$script_dir/total-memory-gb.sh")
if ! [[ "$memory_gb" =~ ^[0-9]+$ ]]; then
  echo "total-memory-gb.sh returned invalid memory: $memory_gb" >&2
  exit 1
fi

max_workers=4
if [ "$(uname -s)" = Darwin ]; then max_workers=2; fi
workers=$((cpu_count / 2))
ram_workers=$((memory_gb / 8))
if [ "$workers" -gt "$ram_workers" ]; then workers=$ram_workers; fi
if [ "$workers" -gt "$max_workers" ]; then workers=$max_workers; fi
if [ "$workers" -lt 1 ]; then workers=1; fi
echo "$workers"
