#!/usr/bin/env bash
# Source before invoking Devbox: its first profile install already uses TMPDIR.
moto_devbox_tmpdir() {
  local repo_root common_root temp filesystem pid_namespace namespace_suffix
  repo_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P) || return 1
  common_root=$(cd -- "$repo_root" && cd -- "$(git rev-parse --git-common-dir)" && pwd -P) || return 1
  temp="$common_root/moto-nix-tmp"
  # Re-scope inherited default paths when entering a sandbox. Arbitrary caller
  # paths remain untouched, including an explicit /tmp.
  if [[ -n "${TMPDIR:-}" && "$TMPDIR" != "$temp" ]]; then
    namespace_suffix=${TMPDIR#"$temp/pid-"}
    [[ "$TMPDIR" == "$temp/pid-"* && "$namespace_suffix" =~ ^[0-9]+$ ]] || return 0
  fi
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
    # PIDs are only meaningful within their creator's namespace. Keep host and
    # sandbox trees apart even when both mount the same Git common directory.
    pid_namespace=$(readlink /proc/self/ns/pid) || return 1
    if [[ ! "$pid_namespace" =~ ^pid:\[([0-9]+)\]$ ]]; then
      echo 'Cannot identify the Devbox PID namespace.' >&2
      return 1
    fi
    temp="$temp/pid-${BASH_REMATCH[1]}"
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
