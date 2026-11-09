#!/usr/bin/env bash
# Source before invoking Devbox: its first profile install already uses TMPDIR.
moto_devbox_private_dir() {
  local path=$1 platform=$2 mode
  if [[ -L "$path" ]]; then
    echo 'Refusing symlinked Devbox temp directory.' >&2
    return 1
  fi
  (umask 077; mkdir -p -- "$path") || return 2
  if [[ ! -O "$path" ]]; then
    echo 'Devbox temp directory must belong to the current user.' >&2
    return 1
  fi
  if [[ "$platform" == Darwin ]]; then
    mode=$(stat -f %Lp -- "$path") || return 2
  else
    mode=$(stat -c %a -- "$path") || return 2
  fi
  # chmod on an already private directory still fails on a read-only mount.
  [[ "$mode" == 700 ]] || chmod 700 -- "$path" || return 2
}

moto_devbox_tmpdir() {
  local repo_root common_root temp filesystem pid_namespace namespace_suffix platform
  repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P) || return 1
  common_root=$(git -C "$repo_root" rev-parse --git-common-dir) || return 1
  common_root=$(cd -- "$repo_root" && cd -- "$common_root" && pwd -P) || return 1
  temp="$common_root/moto-nix-tmp"
  # Re-scope inherited default paths when entering a sandbox. Arbitrary caller
  # paths remain untouched, including an explicit /tmp.
  if [[ -n "${TMPDIR:-}" && "$TMPDIR" != "$temp" ]]; then
    namespace_suffix=${TMPDIR#"$temp/pid-"}
    [[ "$TMPDIR" == "$temp/pid-"* && "$namespace_suffix" =~ ^[0-9]+$ ]] || return 0
  fi
  platform=$(uname -s) || return 1
  moto_devbox_private_dir "$temp" "$platform" || return $?
  if [[ "$platform" == Linux ]]; then
    pid_namespace=$(readlink /proc/self/ns/pid) || return 1
    if [[ ! "$pid_namespace" =~ ^pid:\[([0-9]+)\]$ ]]; then
      echo 'Cannot identify the Devbox PID namespace.' >&2
      return 1
    fi
    temp="$temp/pid-${BASH_REMATCH[1]}"
    moto_devbox_private_dir "$temp" "$platform" || return $?
    filesystem=$(stat -f -c %T -- "$temp") || return 1
    case "$filesystem" in
      tmpfs|ramfs)
        echo 'Devbox TMPDIR requires a disk-backed checkout or an explicit TMPDIR.' >&2
        return 1
        ;;
    esac
  fi
  [[ -w "$temp" ]] || return 2
  export TMPDIR="$temp"
}

if moto_devbox_tmpdir; then
  :
else
  case $? in
    2) echo 'Devbox temp directory is not writable; keeping the original TMPDIR.' >&2 ;;
    *) return 1 ;;
  esac
fi
