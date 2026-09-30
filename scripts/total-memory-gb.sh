#!/usr/bin/env bash
# Print total physical memory in whole GB (macOS and Linux). Used to size
# parallel Go builds so 16 GB laptops do not swap.
set -euo pipefail
if bytes=$(sysctl -n hw.memsize 2>/dev/null); then
  echo $((bytes / 1024 / 1024 / 1024))
elif [ -r /proc/meminfo ]; then
  awk '/^MemTotal:/ { printf "%d\n", $2 / 1024 / 1024 }' /proc/meminfo
else
  echo 16
fi
