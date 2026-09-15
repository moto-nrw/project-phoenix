#!/usr/bin/env bash
# Sourced by quality runners. Keep commands outside conditionals so Bash's
# errexit remains active inside functions and sourced scripts.
quality_step() {
  local label=$1 started=$SECONDS
  shift
  printf '[%s] starting\n' "$label"
  "$@"
  printf '[%s] passed in %ss\n' "$label" "$((SECONDS - started))"
}
