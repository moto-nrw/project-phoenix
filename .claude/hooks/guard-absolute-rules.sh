#!/usr/bin/env bash
# guard-absolute-rules.sh - PreToolUse hook enforcing the ABSOLUTE RULES that
# were prose-only until now:
#   1. No HTTP requests against moto-app.de / moto.nrw
#      (.claude/rules/no-production-requests.md)
#   2. No hand-edits of environments/*.sops.env outside the sops CLI
#      (CLAUDE.md, Environment Management (SOPS))
#   3. No DISABLE ROW LEVEL SECURITY in migrations (CLAUDE.md rule 9)
#   4. No git commit --no-verify / -n (bypasses lefthook incl. the sops
#      encryption check)
#
# Reads the PreToolUse JSON on stdin and self-filters on tool_name, so it can
# be wired with a broad matcher. Blocks with a permissionDecision "deny" JSON
# on stdout plus exit code 2 with the reason on stderr (Claude Code honors
# the JSON, Codex the exit code). Anything it cannot parse passes through.
#
# Execution policy - the repository is the trust boundary, not the machine:
# a path that resolves INSIDE this repository (worktrees included, symlinks
# followed to their final target) may only be executed when git tracks it,
# and a *.sh file may only be executed when it is a tracked repo file, in
# any invocation form (bare, ./, bash/sh/zsh, source, via an interpreter,
# behind env/nice/timeout-style launchers). System binaries outside the
# repository (PATH lookups, /usr/bin, homebrew, the gitignored .devbox tool
# farm, whose entries resolve into /nix/store) are the developer's own
# machine and are NOT vetted here - an allowlist of the world is
# unmaintainable and this hook must never break everyday commands. Inline
# payloads (bash -c, interpreter -c/-e), eval, trap handlers, function
# definitions, dynamic executables and BASH_ENV/LD_PRELOAD-style injection
# variables stay denied because their effect cannot be read from the
# command string.
#
# Accepted residual risk, on purpose: an agent may edit a tracked script and
# run it before committing (refusing dirty scripts would block the
# legitimate edit-then-test loop), go generate directives and untracked Go
# source picked up by go run/go test are not inspected, and a launcher
# flag that consumes a value can hide the launched command from the peeler.
# lefthook, CI and review remain the backstop for what lands. The four rule
# regexes below still scan the whole command string regardless of how a
# command is invoked.
#
# Note: Codex intercepts only the shell tool in PreToolUse, so the Edit/Write
# halves of rules 2 and 3 guard the Claude side only; lefthook stays the
# backstop there. The tracked-script check resolves paths against the
# payload's cwd (Claude); Codex payloads without cwd fall back to $PWD.
#
# Kept bash-3.2 compatible (macOS /bin/bash).

set -euo pipefail

command -v jq >/dev/null 2>&1 || exit 0

input=$(cat 2>/dev/null) || exit 0
[[ -n "$input" ]] || exit 0

tool=$(printf '%s' "$input" | jq -r '.tool_name // empty' 2>/dev/null) || exit 0

deny() {
    local reason="$1"
    jq -n --arg reason "$reason" \
        '{hookSpecificOutput: {hookEventName: "PreToolUse", permissionDecision: "deny", permissionDecisionReason: $reason}}'
    printf '%s\n' "$reason" >&2
    exit 2
}

case "$tool" in
    Bash)
        cmd=$(printf '%s' "$input" | jq -r '.tool_input.command // empty' 2>/dev/null) || exit 0
        [[ -n "$cmd" ]] || exit 0

        cmd_lower=$(printf '%s' "$cmd" | tr '[:upper:]' '[:lower:]')

        # Execution vetting: segments are processed left to right; `cd`
        # segments move the resolution cwd so `cd backend && ../scripts/x.sh`
        # resolves correctly. Any resolution failure blocks (fail closed).
        curdir=$(printf '%s' "$input" | jq -r '.cwd // empty' 2>/dev/null) || curdir=""
        [[ -n "$curdir" && -d "$curdir" ]] || curdir=$PWD
        root=$(git -C "$curdir" rev-parse --show-toplevel 2>/dev/null) || root=${CLAUDE_PROJECT_DIR:-}
        if [[ -n "$root" ]]; then
            root=$(cd -P "$root" 2>/dev/null && pwd) || root=""
        fi

        # resolve_script PATH: resolve the final symlink target without relying
        # on GNU readlink -f (unavailable on macOS).
        resolve_script() {
            local path=$1 link dir depth=0
            while [[ -L "$path" ]]; do
                [[ $depth -lt 40 ]] || return 1
                depth=$((depth + 1))
                link=$(readlink "$path") || return 1
                case "$link" in
                    /*) path=$link ;;
                    *) path=$(dirname "$path")/$link ;;
                esac
            done
            dir=$(cd -P "$(dirname "$path")" 2>/dev/null && pwd) || return 1
            printf '%s/%s' "$dir" "$(basename "$path")"
        }

        # abs_of TOKEN: absolute path of TOKEN relative to the tracked cwd.
        abs_of() {
            local tok=$1 dir base
            case "$tok" in
                */*) dir=${tok%/*} base=${tok##*/} ;;
                *) dir=. base=$tok ;;
            esac
            (cd -P "$curdir" 2>/dev/null && cd -P "$dir" 2>/dev/null && printf '%s/%s' "$PWD" "$base")
        }

        # vet_script TOKEN: true iff TOKEN resolves to a git-tracked file
        # inside $root. The final symlink target is resolved too: a tracked
        # symlink must not make an untracked or out-of-repository program
        # executable.
        vet_script() {
            local tok=$1 abs target rel
            [[ -n "$root" ]] || return 1
            [[ -z "$path_assigned" ]] || deny "Blocked: a PATH assignment combined with a repository script can change what the script executes. Drop the PATH assignment."
            abs=$(abs_of "$tok") || return 1
            [[ -f "$abs" ]] || return 1
            target=$(resolve_script "$abs") || return 1
            case "$target" in
                "$root"/*) rel=${target#"$root"/} ;;
                *) return 1 ;;
            esac
            git -C "$root" ls-files --error-unmatch -- "$rel" >/dev/null 2>&1
        }

        deny_untracked() {
            deny "Blocked: only git-tracked scripts inside this repository may be executed (got: $1)."
        }

        # vet_exec_target TOKEN: executable rule for path-shaped tokens.
        # Inside the repository everything must be tracked (the gitignored
        # .devbox tool farm is exempt: its entries are symlinks into
        # /nix/store, i.e. outside). Outside the repository nothing is
        # vetted - system binaries are the developer's machine.
        vet_exec_target() {
            local tok=$1 abs target
            case "$tok" in
                *.sh) vet_script "$tok" || deny_untracked "$tok"; return 0 ;;
                */*) ;;
                *) return 0 ;; # bare token: resolves through PATH, not vetted
            esac
            abs=$(abs_of "$tok") || deny_untracked "$tok"
            case "$abs" in
                "$root"/* | "$root")
                    [[ -n "$root" ]] || deny_untracked "$tok"
                    target=$(resolve_script "$abs") || deny_untracked "$tok"
                    case "$target" in
                        "$root"/.devbox/*) return 0 ;;
                        "$root"/*) vet_script "$tok" || deny_untracked "$tok" ;;
                        *)
                            case "$abs" in
                                "$root"/.devbox/*) return 0 ;;
                                *) deny_untracked "$tok" ;;
                            esac
                            ;;
                    esac
                    ;;
            esac
            return 0
        }

        clean_token() {
            local tok=$1
            tok=${tok//\"/}
            printf '%s' "${tok//\'/}"
        }

        reject_dynamic_executable() {
            # shellcheck disable=SC2016 # match literal shell-expansion syntax
            case "$1" in
                *'${'* | *'$'* | *'`'* | *'<('* | *'>('* | *__guard_substitution__*)
                    deny "Blocked: executable paths expanded by the shell cannot be inspected by the absolute-rule guard. Write the tracked path out directly."
                    ;;
            esac
        }

        # Injection-capable environment variables. Everything else -
        # PATH included, except in combination with a repository script -
        # is the developer's business.
        vet_env_assignment() {
            local assignment=$1 name
            name=${assignment%%=*}
            case "$name" in
                BASH_ENV | ENV | ZDOTDIR)
                    deny "Blocked: $name can source an untracked shell startup file before the executable runs."
                    ;;
                LD_PRELOAD | LD_LIBRARY_PATH | DYLD_*)
                    deny "Blocked: $name injects code into the launched executable and cannot be inspected by the absolute-rule guard."
                    ;;
                GOFLAGS)
                    case "$assignment" in
                        *-exec* | *-toolexec*)
                            deny "Blocked: GOFLAGS with -exec/-toolexec can launch an untracked program through the Go toolchain."
                            ;;
                    esac
                    ;;
                PATH) path_assigned=1 ;;
            esac
        }

        # run-go-toolchain.sh executes its first argument from the pinned
        # toolchain directory. Keep that surface to the pinned Go tools and
        # tracked scripts; no deep argument inspection beyond the -exec
        # escape hatch of go test.
        vet_toolchain_request() {
            local tok=${1:-} arg
            [[ -n "$tok" ]] || return 0
            reject_dynamic_executable "$tok"
            if [[ "$tok" = */* ]]; then
                vet_script "$tok" || deny_untracked "$tok"
                return 0
            fi
            case "$tok" in
                go)
                    for arg in "$@"; do
                        case "$(clean_token "$arg")" in
                            -exec | -exec=* | -toolexec | -toolexec=*)
                                deny "Blocked: go -exec/-toolexec can launch an untracked program."
                                ;;
                        esac
                    done
                    ;;
                gofmt | gotestsum | golangci-lint | govulncheck) ;;
                *) deny "Blocked: run-go-toolchain accepts only pinned Go tools or tracked scripts." ;;
            esac
        }

        scan_segment() {
            local segment=$1 first next tok path_assigned=''
            # shellcheck disable=SC2086
            set -- $segment
            [[ $# -gt 0 ]] || return 0
            # env-assignment prefixes
            while [[ ${1:-} == *=* && ${1%%=*} != */* ]]; do
                vet_env_assignment "$(clean_token "$1")"
                shift || break
            done
            [[ $# -gt 0 ]] || return 0
            first=$(clean_token "$1")

            # These launchers leave the actual executable in their argument
            # list. Peel them before checking script paths. A slashed
            # launcher path (./env, /usr/bin/env) is itself vetted as an
            # execution target first, so an untracked repo file cannot pose
            # as a launcher.
            while :; do
                case "$first" in
                    */*)
                        case "${first##*/}" in
                            env | builtin | command | time | nohup | setsid | stdbuf | exec | nice | timeout | sudo | xargs)
                                vet_exec_target "$first"
                                ;;
                        esac
                        ;;
                esac
                case "${first##*/}" in
                    env)
                        shift
                        while [[ $# -gt 0 ]]; do
                            next=$(clean_token "$1")
                            case "$next" in
                                -C | -C* | --chdir | --chdir=*)
                                    deny "Blocked: env --chdir changes the executable resolution directory and cannot be inspected by the absolute-rule guard."
                                    ;;
                                -S | --split-string)
                                    deny "Blocked: env -S builds a command at runtime and cannot be inspected by the absolute-rule guard. Write the command out directly."
                                    ;;
                                -u | --unset) shift; shift || break ;;
                                --) shift; break ;;
                                -*) shift ;;
                                *=*) vet_env_assignment "$next"; shift ;;
                                *) break ;;
                            esac
                        done
                        ;;
                    builtin)
                        shift
                        [[ ${1:-} != -* ]] || deny "Blocked: builtin options cannot be inspected by the absolute-rule guard. Write the command out directly."
                        ;;
                    command | time | nohup | setsid | stdbuf)
                        shift
                        while [[ ${1:-} == -* ]]; do shift; done
                        ;;
                    exec)
                        shift
                        while [[ $# -gt 0 ]]; do
                            next=$(clean_token "$1")
                            case "$next" in
                                -a) shift; shift || break ;;
                                --) shift; break ;;
                                -*) shift ;;
                                *) break ;;
                            esac
                        done
                        ;;
                    nice)
                        shift
                        while [[ $# -gt 0 ]]; do
                            next=$(clean_token "$1")
                            case "$next" in
                                -n | --adjustment) shift; shift || break ;;
                                --) shift; break ;;
                                -*) shift ;;
                                *) break ;;
                            esac
                        done
                        ;;
                    timeout)
                        shift
                        while [[ $# -gt 0 ]]; do
                            next=$(clean_token "$1")
                            case "$next" in
                                -k | --kill-after) shift; shift || break ;;
                                --) shift; break ;;
                                -*) shift ;;
                                *) shift; break ;;
                            esac
                        done
                        ;;
                    sudo)
                        shift
                        while [[ $# -gt 0 ]]; do
                            next=$(clean_token "$1")
                            case "$next" in
                                -u | -g | -h | -p | -r | -t | -C | -c | --user | --group | --host | --prompt | --role | --type | --closefrom)
                                    shift; shift || break ;;
                                --) shift; break ;;
                                -*) shift ;;
                                *) break ;;
                            esac
                        done
                        ;;
                    xargs)
                        # The command xargs launches is its first non-flag
                        # argument; flag values (e.g. -n 1) can shadow it,
                        # which errs toward allowing - accepted gap.
                        shift
                        while [[ ${1:-} == -* ]]; do shift; done
                        ;;
                    *) break ;;
                esac
                [[ $# -gt 0 ]] || return 0
                first=$(clean_token "$1")
            done
            reject_dynamic_executable "$first"
            while [[ "$first" = "if" || "$first" = "elif" || "$first" = "then" || "$first" = "else" ||
                "$first" = "while" || "$first" = "until" || "$first" = "do" || "$first" = '!' ||
                "$first" = '(' || "$first" = '{' ]]; do
                shift
                [[ $# -gt 0 ]] || return 0
                first=$(clean_token "$1")
                reject_dynamic_executable "$first"
            done
            case "$first" in
                function | case)
                    deny "Blocked: shell function and case bodies cannot be inspected by the absolute-rule guard. Write the command out directly."
                    ;;
                *'()' | *'(){'*)
                    [[ ${2:-} = '{' ]] && deny "Blocked: shell function definitions cannot be inspected by the absolute-rule guard. Write the command out directly."
                    [[ "$first" = *'(){'* ]] && deny "Blocked: shell function definitions cannot be inspected by the absolute-rule guard. Write the command out directly."
                    ;;
            esac
            [[ ${2:-} = '()' && ${3:-} = '{' ]] && deny "Blocked: shell function definitions cannot be inspected by the absolute-rule guard. Write the command out directly."
            [[ ${2:-} = '(){'* ]] && deny "Blocked: shell function definitions cannot be inspected by the absolute-rule guard. Write the command out directly."
            case "$first" in
                /bin/bash | /bin/sh | /bin/zsh) first=${first##*/} ;;
            esac
            case "$first" in
                fi | 'esac' | done | '}' | ')') return 0 ;;
            esac
            case "${first##*/}" in
                cd)
                    # keep resolution honest for `cd backend && ../scripts/x.sh`
                    next=${2:-.}
                    next=$(clean_token "$next")
                    reject_dynamic_executable "$next"
                    case "$next" in
                        -*) deny "Blocked: cd options and directory-stack targets cannot be inspected by the absolute-rule guard. Write the repository path directly." ;;
                    esac
                    if moved=$(cd -P "$curdir" 2>/dev/null && cd -P "$next" 2>/dev/null && pwd); then
                        curdir=$moved
                    fi
                    return 0
                    ;;
                pushd | popd)
                    deny "Blocked: $first changes the executable resolution directory and cannot be inspected by the absolute-rule guard. Use cd with an explicit tracked path instead."
                    ;;
                eval)
                    deny "Blocked: eval builds its command at runtime and cannot be inspected by the absolute-rule guard. Write the command out directly."
                    ;;
                trap)
                    deny "Blocked: trap handlers execute later and cannot be inspected by the absolute-rule guard. Write the command out directly."
                    ;;
                bash | sh | zsh | source | .)
                    shift
                    while [[ ${1:-} == -* ]]; do
                        case "$1" in
                            -*c*) deny "Blocked: inline shell payloads (-c) cannot be inspected by the absolute-rule guard. Write the command out directly." ;;
                        esac
                        shift || break
                    done
                    [[ $# -gt 0 ]] || return 0
                    tok=$(clean_token "$1")
                    reject_dynamic_executable "$tok"
                    vet_script "$tok" || deny_untracked "$tok"
                    first=$tok
                    ;;
                python* | node | nodejs | ruby | perl)
                    shift
                    while [[ ${1:-} == -* ]]; do
                        case "$1" in
                            -c | -e | -p | --command | --eval | --print)
                                deny "Blocked: inline interpreter payloads cannot be inspected by the absolute-rule guard. Write the tracked script path out directly."
                                ;;
                            -m | --module) return 0 ;;
                        esac
                        shift || break
                    done
                    [[ $# -gt 0 ]] || return 0
                    tok=$(clean_token "$1")
                    reject_dynamic_executable "$tok"
                    vet_exec_target "$tok"
                    return 0
                    ;;
                find)
                    while [[ $# -gt 0 ]]; do
                        case "$(clean_token "$1")" in
                            -exec | -execdir)
                                shift
                                [[ $# -gt 0 ]] || deny "Blocked: find -exec needs a command."
                                tok=$(clean_token "$1")
                                reject_dynamic_executable "$tok"
                                vet_exec_target "$tok"
                                ;;
                        esac
                        shift || break
                    done
                    return 0
                    ;;
                *.sh)
                    reject_dynamic_executable "$first"
                    vet_script "$first" || deny_untracked "$first"
                    ;;
                *)
                    if [[ "$first" = */* ]]; then
                        vet_exec_target "$first"
                        first=$(abs_of "$first" 2>/dev/null) || return 0
                    else
                        # Bare tokens resolve through the developer's PATH
                        # outside the repository and are not vetted.
                        return 0
                    fi
                    ;;
            esac

            # A vetted repo script may only pass tracked scripts on; the
            # pinned-toolchain wrapper keeps its narrower surface.
            if [[ "${first##*/}" = run-go-toolchain.sh ]]; then
                shift
                [[ $# -gt 0 ]] || return 0
                vet_toolchain_request "$@"
                return 0
            fi
            for tok in "$@"; do
                tok=$(clean_token "$tok")
                case "$tok" in
                    -*) continue ;;
                    *.sh) vet_script "$tok" || deny_untracked "$tok" ;;
                esac
            done
        }

        # Recursively visit shell command substitutions, while processing
        # separators only outside quotes. This is deliberately a lexer, never
        # an evaluator: the command supplied to the hook is never executed.
        scan_commands() {
            local text=$1 segment='' quote='' ch next inner inner_quote depth i j len entry_curdir=$curdir subshell_depth=0
            local -a subshell_dirs=()
            len=${#text}
            i=0
            while ((i < len)); do
                ch=${text:i:1}
                next=${text:i+1:1}
                if [[ "$quote" = "'" ]]; then
                    segment+=$ch
                    [[ "$ch" = "'" ]] && quote=''
                    i=$((i + 1))
                    continue
                fi
                if [[ "$quote" = '"' && "$ch" = '"' ]]; then
                    segment+=$ch
                    quote=''
                    i=$((i + 1))
                    continue
                fi
                if [[ "$ch" = $'\\' ]]; then
                    segment+=$ch$next
                    i=$((i + 2))
                    continue
                fi
                if [[ "$ch" = "'" ]]; then
                    segment+=$ch
                    quote="'"
                    i=$((i + 1))
                    continue
                fi
                if [[ "$ch" = '"' ]]; then
                    segment+=$ch
                    if [[ "$quote" = '"' ]]; then quote=''; else quote='"'; fi
                    i=$((i + 1))
                    continue
                fi
                if [[ ("$ch" = '<' || "$ch" = '>') && "$next" = '(' ]]; then
                    deny "Blocked: process substitutions cannot be inspected by the absolute-rule guard. Write the command out directly."
                fi
                if [[ "$ch" = '$' && "$next" = '(' ]]; then
                    inner=''
                    inner_quote=''
                    depth=1
                    j=$((i + 2))
                    while ((j < len && depth > 0)); do
                        ch=${text:j:1}
                        if [[ -n "$inner_quote" ]]; then
                            inner+=$ch
                            [[ "$ch" = "$inner_quote" ]] && inner_quote=''
                        elif [[ "$ch" = $'\\' ]]; then
                            inner+=$ch${text:j+1:1}
                            j=$((j + 1))
                        elif [[ "$ch" = "'" || "$ch" = '"' ]]; then
                            inner+=$ch
                            inner_quote=$ch
                        elif [[ "$ch" = '(' ]]; then
                            inner+=$ch
                            depth=$((depth + 1))
                        elif [[ "$ch" = ')' ]]; then
                            depth=$((depth - 1))
                            if ((depth > 0)); then inner+=$ch; fi
                        else
                            inner+=$ch
                        fi
                        j=$((j + 1))
                    done
                    ((depth == 0)) || deny "Blocked: an unterminated command substitution cannot be inspected by the absolute-rule guard."
                    scan_commands "$inner"
                    curdir=$entry_curdir
                    segment+='__guard_substitution__'
                    i=$j
                    continue
                fi
                if [[ "$ch" = '`' ]]; then
                    inner=''
                    j=$((i + 1))
                    while ((j < len)) && [[ ${text:j:1} != '`' ]]; do
                        inner+=${text:j:1}
                        j=$((j + 1))
                    done
                    ((j < len)) || deny "Blocked: an unterminated command substitution cannot be inspected by the absolute-rule guard."
                    scan_commands "$inner"
                    curdir=$entry_curdir
                    segment+='__guard_substitution__'
                    i=$((j + 1))
                    continue
                fi
                if [[ "$ch" = '(' && "$segment" =~ ^[[:space:]]*$ ]]; then
                    subshell_dirs[${#subshell_dirs[@]}]=$curdir
                    subshell_depth=$((subshell_depth + 1))
                    i=$((i + 1))
                    continue
                fi
                if [[ "$ch" = ')' && $subshell_depth -gt 0 ]]; then
                    scan_segment "$segment"
                    segment=''
                    subshell_depth=$((subshell_depth - 1))
                    curdir=${subshell_dirs[$subshell_depth]}
                    unset 'subshell_dirs[$subshell_depth]'
                    i=$((i + 1))
                    continue
                fi
                if [[ "$quote" = '"' ]]; then
                    case "$ch" in
                        ';' | '|' | '&' | $'\n')
                            segment+=$ch
                            i=$((i + 1))
                            continue
                            ;;
                    esac
                fi
                case "$ch" in
                    ';' | $'\n')
                        scan_segment "$segment"
                        segment=''
                        ;;
                    '|')
                        scan_segment "$segment"
                        segment=''
                        curdir=$entry_curdir
                        [[ "$next" = '&' ]] && i=$((i + 1))
                        ;;
                    '&')
                        scan_segment "$segment"
                        segment=''
                        if [[ "$next" = '&' ]]; then
                            i=$((i + 1))
                        else
                            curdir=$entry_curdir
                        fi
                        ;;
                    *)
                        segment+=$ch
                        ;;
                esac
                i=$((i + 1))
            done
            ((subshell_depth == 0)) || deny "Blocked: an unterminated subshell cannot be inspected by the absolute-rule guard."
            scan_segment "$segment"
        }

        set -f # word-split segments without globbing
        scan_commands "$cmd"
        set +f
        if printf '%s' "$cmd_lower" | grep -Eq '(^|[|&;[:space:]])(curl|wget|xh|httpie|https?|nc|ncat|wscat|fetch)([[:space:]]|$)' &&
            printf '%s' "$cmd" | grep -Eq '\$[({[:alpha:]_]|<\(|`'; then
            deny "Blocked: network commands with shell expansion may construct a production hostname. Use localhost; only a human may target production."
        fi
        while IFS= read -r segment; do
            [[ -n "$segment" ]] || continue
            if printf '%s' "$segment" | grep -Eq 'moto-app\.de|moto\.nrw' &&
                { ! printf '%s' "$segment" | grep -Eq '^[[:space:]]*(rg|grep|cat|git[[:space:]]+grep)([[:space:]]|$)' ||
                    printf '%s' "$segment" | grep -Eq '\$\(|<\(|`'; }; then
                deny "Blocked: HTTP requests against moto-app.de / moto.nrw are forbidden (.claude/rules/no-production-requests.md). Use localhost; only a human may target production."
            fi
        done <<EOF
$(printf '%s' "$cmd_lower" | tr ';|&' '\n')
EOF

        # Rule 2 (shell half): validate each shell segment. A SOPS file may
        # be passed only to sops itself or to an obvious read-only command;
        # every other use could write it. This prevents an initial `sops`
        # command from exempting later writers in an &&/pipe chain.
        while IFS= read -r segment; do
            [[ -n "$segment" ]] || continue
            if printf '%s' "$segment" | grep -Eq '\.sops[^[:alnum:]]*env' &&
                printf '%s' "$segment" | grep -Eq '\$\(|<\(|`'; then
                deny "Blocked: environments/*.sops.env must only be edited with the sops CLI (sops environments/<env>.sops.env). See CLAUDE.md, Environment Management (SOPS)."
            fi
            if printf '%s' "$segment" | grep -Eq '\.sops[^[:alnum:]]*env' &&
                printf '%s' "$segment" | grep -Eq '^[[:space:]]*git[[:space:]]+diff([[:space:]]|$)' &&
                printf '%s' "$segment" | grep -Eq -- '--output(=|[[:space:]])'; then
                deny "Blocked: environments/*.sops.env must only be edited with the sops CLI (sops environments/<env>.sops.env). See CLAUDE.md, Environment Management (SOPS)."
            fi
            if printf '%s' "$segment" | grep -Eq '\.sops[^[:alnum:]]*env' &&
                ! printf '%s' "$segment" | grep -Eq '^[[:space:]]*(sops|rg|grep|cat|git[[:space:]]+(add|diff|show))([[:space:]]|$)'; then
                deny "Blocked: environments/*.sops.env must only be edited with the sops CLI (sops environments/<env>.sops.env). See CLAUDE.md, Environment Management (SOPS)."
            fi
        done <<EOF
$(printf '%s' "$cmd" | tr ';|&' '\n')
EOF

        # A redirection can overwrite a SOPS file even when the command
        # itself is on the read-only allowlist (for example `cat ... > file`).
        if printf '%s' "$cmd" | grep -Eq '>>?[[:space:]]*[^[:space:];|&]*\.sops\.env'; then
            deny "Blocked: environments/*.sops.env must only be edited with the sops CLI (sops environments/<env>.sops.env). See CLAUDE.md, Environment Management (SOPS)."
        fi

        # Rule 3 (shell half): shell writers can create migrations too. The
        # dangerous SQL must be rejected before a command writes it there.
        if printf '%s' "$cmd" | grep -Eq '(database/)?migrations([/[:space:]]|$)' &&
            printf '%s' "$cmd" | grep -Eiq 'DISABLE[[:space:]]+ROW[[:space:]]+LEVEL[[:space:]]+SECURITY'; then
            deny "Blocked: migrations must not contain DISABLE ROW LEVEL SECURITY (CLAUDE.md rule 9 - the superuser connection bypasses RLS, disabling it is unnecessary and breaks tests)."
        fi
        if printf '%s' "$cmd" | grep -Eq '(^|[|&;[:space:]])(cp|mv|install)[[:space:]]+' &&
            printf '%s' "$cmd" | grep -Eq '(database/)?migrations/'; then
            deny "Blocked: copy or move operations into migrations cannot be safely inspected for DISABLE ROW LEVEL SECURITY."
        fi

        # Rule 4: --no-verify bypasses lefthook incl. the sops encryption
        # check. Scoped to the same pipeline segment as "git commit"; a
        # literal " -n " inside a commit message is an accepted false
        # positive.
        if printf '%s' "$cmd" | grep -Eq '(^|[|;&[:space:]])([^[:space:];|&]*/)?git([[:space:]]+[^|;&[:space:]]+)*[[:space:]]+commit[^|;&]*[[:space:]](--no-verify|-n)([[:space:]]|$)'; then
            deny "Blocked: git commit --no-verify / -n skips lefthook (incl. the sops encryption guard). Commit without it; if a hook misfires, fix the hook."
        fi
        ;;
    WebFetch)
        url=$(printf '%s' "$input" | jq -r '.tool_input.url // empty' 2>/dev/null) || exit 0
        url=$(printf '%s' "$url" | tr '[:upper:]' '[:lower:]')
        authority=${url#*://}
        authority=${authority%%[/?#]*}
        host=${authority##*@}
        host=${host%%:*}
        if printf '%s' "$host" | grep -Eq '^([[:alnum:]-]+\.)*moto-app\.de\.?$|^([[:alnum:]-]+\.)*moto\.nrw\.?$'; then
            deny "Blocked: HTTP requests against moto-app.de / moto.nrw are forbidden (.claude/rules/no-production-requests.md). Use localhost; only a human may target production."
        fi
        ;;
    Edit | Write)
        file=$(printf '%s' "$input" | jq -r '.tool_input.file_path // empty' 2>/dev/null) || exit 0
        [[ -n "$file" ]] || exit 0

        # Rule 2 (editor half): SOPS env files only through the sops CLI.
        case "$file" in
            *.sops.env)
                deny "Blocked: $file is SOPS-encrypted. Edit it only with the sops CLI (sops environments/<env>.sops.env), never by hand. See CLAUDE.md, Environment Management (SOPS)."
                ;;
        esac

        # Rule 3: no RLS toggling in migrations - the migration CLI runs as
        # the postgres superuser, which bypasses RLS anyway.
        case "$file" in
            */database/migrations/*)
                content=$(printf '%s' "$input" | jq -r '(.tool_input.new_string // .tool_input.content) // empty' 2>/dev/null) || exit 0
                if printf '%s' "$content" | grep -Eiq 'DISABLE[[:space:]]+ROW[[:space:]]+LEVEL[[:space:]]+SECURITY'; then
                    deny "Blocked: migrations must not contain DISABLE ROW LEVEL SECURITY (CLAUDE.md rule 9 - the superuser connection bypasses RLS, disabling it is unnecessary and breaks tests)."
                fi
                ;;
        esac
        ;;
esac

exit 0
