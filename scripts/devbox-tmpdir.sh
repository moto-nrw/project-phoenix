#!/usr/bin/env bash
# Source before invoking Devbox: its first profile install already uses TMPDIR.
moto_devbox_tmpdir() {
  [[ -z "${TMPDIR:-}" ]] || return 0
  local repo_root common_root temp filesystem
  repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P) || return 1
  common_root=$(cd -- "$repo_root" && cd -- "$(git rev-parse --git-common-dir)" && pwd -P) || return 1
  temp="$common_root/moto-nix-tmp"
  if [[ -L "$temp" ]]; then
    echo 'Refusing symlinked Devbox temp directory.' >&2
    return 1
  fi
  (umask 077; mkdir -p -- "$temp") || return 1
  if [[ ! -O "$temp" ]]; then
    echo 'Devbox temp directory must belong to the current user.' >&2
    return 1
  fi
  chmod 700 -- "$temp" || return 1
  if [[ $(uname -s) == Linux ]]; then
    filesystem=$(stat -f -c %T -- "$temp") || return 1
    case "$filesystem" in
      tmpfs|ramfs)
        echo 'Devbox TMPDIR requires a disk-backed checkout or an explicit TMPDIR.' >&2
        return 1
        ;;
    esac
  fi
  export TMPDIR="$temp"
}
moto_devbox_tmpdir
