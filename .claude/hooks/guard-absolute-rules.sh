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
# Script execution: a file may be executed (bare, ./, bash/sh/zsh, source)
# only when it resolves inside this repository (worktrees included) and git
# tracks it - to get a script tracked it must pass lefthook, CI and review.
# Untracked or out-of-repo scripts, `bash -c` wrappers and `eval` stay
# blocked because their payload cannot be inspected here. Accepted residual
# risk, on purpose: an agent may edit a tracked script and run it before
# committing; refusing dirty scripts would block the legitimate
# edit-then-test loop, so lefthook/CI/review remain the backstop for what
# lands. The four rule regexes below still scan the whole command string
# regardless of how a command is invoked.
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

        # Rule 1: a production hostname in a shell segment is a request
        # unless that segment is plainly reading source text. This covers
        # wrapped clients (python/node/bash -c) without maintaining a
        # bypassable list of network binaries.
        cmd_lower=$(printf '%s' "$cmd" | tr '[:upper:]' '[:lower:]')

        # Script execution: only git-tracked files inside this repository may
        # be run. Segments are processed left to right; `cd` segments move
        # the resolution cwd so `cd backend && ../scripts/x.sh` resolves
        # correctly. Any resolution failure blocks (fail closed).
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

        # vet_script TOKEN: true iff TOKEN resolves to a git-tracked file
        # inside $root. Resolve the final target too: a tracked symlink must
        # not make an untracked or out-of-repository program executable.
        vet_script() {
            local tok=$1 dir base abs target rel
            [[ -n "$root" ]] || return 1
            case "$tok" in
                */*) dir=${tok%/*} base=${tok##*/} ;;
                *) dir=. base=$tok ;;
            esac
            abs=$(cd -P "$curdir" 2>/dev/null && cd -P "$dir" 2>/dev/null && printf '%s/%s' "$PWD" "$base") || return 1
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

        # System-tool directories may supply compiled tools, but never shell
        # scripts. Other paths remain subject to repository script vetting.
        is_script_file() {
            local first=''
            IFS= read -r first < "$1" || true
            [[ "$first" = '#!'* ]]
        }

        vet_trusted_binary() {
            [[ -x "$1" && ! -d "$1" ]] && ! is_script_file "$1"
        }

        # pnpm is a Node script installed by Homebrew. Its interpreter must
        # be a vetted binary too.
        vet_homebrew_pnpm() {
            local target=$1 first
            target=$(resolve_script "$target") || return 1
            [[ -x "$target" && ! -d "$target" ]] || return 1
            IFS= read -r first < "$target" || return 1
            [[ "$target:$first" = /opt/homebrew/Cellar/pnpm/*/libexec/lib/node_modules/pnpm/bin/pnpm.mjs:'#!/usr/bin/env node' ]] || return 1
            vet_bare_executable node
        }

        # Devbox pnpm is a declared project dependency. Only its generated
        # launcher may be a shebang script; direct Nix-store scripts are not
        # execution targets.
        vet_devbox_pnpm() {
            local source=$1 target first interpreter
            [[ "$source" = "$root/.devbox/nix/profile/default/bin/pnpm" ]] || return 1
            git -C "$root" ls-files --error-unmatch -- devbox.json >/dev/null 2>&1 || return 1
            jq -e 'any(.packages[]; startswith("pnpm@"))' "$root/devbox.json" >/dev/null || return 1
            target=$(resolve_script "$source") || return 1
            [[ -x "$target" && ! -d "$target" ]] || return 1
            IFS= read -r first < "$target" || return 1
            [[ "$target:$first" = /nix/store/*-pnpm-*/libexec/pnpm/bin/pnpm.mjs:'#!'/nix/store/*/bin/node ]] || return 1
            interpreter=${first#\#!}
            vet_trusted_binary "$interpreter"
        }

        vet_executable() {
            local target
            case "$1" in
                /opt/homebrew/bin/pnpm | /opt/homebrew/Cellar/pnpm/*/bin/pnpm)
                    vet_homebrew_pnpm "$1"
                    ;;
                /bin/* | /sbin/* | /usr/bin/* | /usr/sbin/* | /usr/local/bin/* | /opt/homebrew/bin/* | /opt/hostedtoolcache/go/*/bin/go | */node_modules/@openai/codex-*/vendor/*/codex-path/rg)
                    vet_trusted_binary "$1"
                    ;;
                /nix/store/*)
                    vet_trusted_binary "$1"
                    ;;
                */.devbox/nix/profile/default/bin/*)
                    target=$(resolve_script "$1") || return 1
                    [[ "$target" = /nix/store/* ]] || return 1
                    vet_devbox_pnpm "$1" || vet_trusted_binary "$target"
                    ;;
                *) vet_script "$1" ;;
            esac
        }

        vet_bare_executable() {
            local tok=$1 resolved
            resolved=$(command -v "$tok" 2>/dev/null) || deny_untracked "$tok"
            case "$resolved" in
                "$tok") return 0 ;; # shell builtin
                /*) vet_executable "$resolved" || deny_untracked "$tok" ;;
                *) deny_untracked "$tok" ;;
            esac
        }

        vet_launcher() {
            reject_dynamic_executable "$1"
            if [[ "$1" = */* ]]; then
                vet_executable "$1" || deny_untracked "$1"
            else
                vet_bare_executable "$1"
            fi
        }

        clean_token() {
            local tok=$1
            tok=${tok//\"/}
            printf '%s' "${tok//\'/}"
        }

        reject_dynamic_executable() {
            # shellcheck disable=SC2016 # match literal shell-expansion syntax
            case "$1" in
                *'${'*|*'$'*|*'`'*|*'<('*|*'>('*|*__guard_substitution__*)
                    deny "Blocked: executable paths expanded by the shell cannot be inspected by the absolute-rule guard. Write the tracked path out directly."
                    ;;
            esac
        }

        vet_go_dependencies() {
            local tok=$1 go_path files go_file rel
            go_path=$(command -v go 2>/dev/null) || deny_untracked go
            vet_executable "$go_path" || deny_untracked go
            files=$(cd "$curdir" && "$go_path" list -deps -test -json "$tok" |
                jq -r --arg root "$root" '
                    select(.Dir == $root or (.Dir | startswith($root + "/"))) |
                    .Dir as $dir |
                    (.GoFiles[]?, .CgoFiles[]?, .CFiles[]?, .CXXFiles[]?, .MFiles[]?, .HFiles[]?, .SFiles[]?, .SysoFiles[]?, .EmbedFiles[]?, .TestGoFiles[]?, .XTestGoFiles[]?) |
                    $dir + "/" + .
                ') || deny_untracked "$tok"
            [[ -n "$files" ]] || deny_untracked "$tok"
            while IFS= read -r go_file; do
                [[ -n "$go_file" ]] || continue
                rel=${go_file#"$root"/}
                git -C "$root" ls-files --error-unmatch -- "$rel" >/dev/null 2>&1 || deny_untracked "$go_file"
            done <<EOF
$files
EOF
        }

        vet_go_run() {
            local tok source
            tok=${1:-}
            source=$tok
            [[ -n "$tok" ]] || deny "Blocked: go run needs a tracked package or Go source file."
            reject_dynamic_executable "$tok"
            case "$tok" in
                -*) deny "Blocked: go run flags cannot be inspected by the absolute-rule guard. Write the tracked package path directly." ;;
                *.go)
                    vet_script "$tok" || deny_untracked "$tok"
                    shift
                    while [[ $# -gt 0 && ${1:-} != -- ]]; do
                        tok=$(clean_token "$1")
                        [[ "$tok" = *.go ]] || break
                        vet_script "$tok" || deny_untracked "$tok"
                        shift
                    done
                    vet_go_dependencies "$source"
                    return 0
                    ;;
                .|./*|../*|/*) ;;
                *) deny "Blocked: go run may execute only a tracked local package or Go source file." ;;
            esac
            vet_go_dependencies "$tok"
        }

        vet_go_command() {
            local tok has_package=false
            while [[ $# -gt 0 ]]; do
                case "$(clean_token "$1")" in
                    -C|-C*) deny "Blocked: go -C changes the executable resolution directory and cannot be inspected by the absolute-rule guard." ;;
                    run) shift; vet_go_run "$@"; return 0 ;;
                    test)
                        shift
                        while [[ $# -gt 0 ]]; do
                            tok=$(clean_token "$1")
                            case "$tok" in
                                -exec|-exec=*) deny "Blocked: go test -exec can launch an untracked program." ;;
                                --) deny "Blocked: go test arguments after -- cannot be inspected by the absolute-rule guard." ;;
                                -bench|-benchtime|-count|-coverprofile|-cpu|-list|-p|-parallel|-run|-shuffle|-timeout|-vet)
                                    shift
                                    [[ $# -gt 0 ]] || deny "Blocked: go test flag $tok needs an argument."
                                    ;;
                                -a|-asan|-cover|-failfast|-fullpath|-json|-msan|-race|-short|-trimpath|-v|-work|-x) ;;
                                -*) deny "Blocked: go test flag $tok can change the executed source set and cannot be inspected by the absolute-rule guard." ;;
                                .|./*|../*|/*)
                                    vet_go_dependencies "$tok"
                                    has_package=true
                                    ;;
                                *) deny "Blocked: go test may test only tracked local packages." ;;
                            esac
                            shift
                        done
                        "$has_package" || vet_go_dependencies .
                        return 0
                        ;;
                    *) return 0 ;;
                esac
            done
        }

        vet_toolchain_request() {
            local tok=${1:-}
            reject_dynamic_executable "$tok"
            if [[ "$tok" = */* ]]; then
                vet_script "$tok" || deny_untracked "$tok"
                return 0
            fi
            case "$tok" in
                go) shift; vet_go_command "$@" ;;
                golangci-lint|govulncheck) ;;
                *) deny "Blocked: run-go-toolchain accepts only pinned Go tools or tracked scripts." ;;
            esac
        }

        vet_env_assignment() {
            local assignment=$1 name
            name=${assignment%%=*}
            case "$name" in
                BASH_ENV|ENV|ZDOTDIR)
                    deny "Blocked: $name can source an untracked shell startup file before the executable runs."
                    ;;
                PATH)
                    deny "Blocked: PATH can change which executable runs. Use the configured toolchain path directly."
                    ;;
                GO*|CGO_*|CC|CXX|AR|AS|LD|RANLIB|PKG_CONFIG)
                    deny "Blocked: $name can alter Go toolchain execution. Set toolchain configuration only inside a tracked wrapper."
                    ;;
            esac
        }

        scan_segment() {
            local segment=$1 first next tok
            # shellcheck disable=SC2086
            set -- $segment
            [[ $# -gt 0 ]] || return 0
            # skip FOO=1 env-assignment prefixes
            while [[ ${1:-} == *=* && ${1%%=*} != */* ]]; do
                vet_env_assignment "$(clean_token "$1")"
                shift || break
            done
            [[ $# -gt 0 ]] || return 0
            first=$(clean_token "$1")

            # These launchers leave the actual executable in their argument
            # list. Peel them before checking shell options or script paths.
            while :; do
                case "${first##*/}" in
                    env|builtin|command|time|nohup|setsid|exec|nice|timeout|sudo|stdbuf)
                        vet_launcher "$first"
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
                                -S|--split-string)
                                    deny "Blocked: env -S builds a command at runtime and cannot be inspected by the absolute-rule guard. Write the command out directly."
                                    ;;
                                -u|--unset) shift; shift || break ;;
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
                    command|time|nohup|setsid)
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
                                -n|--adjustment) shift; shift || break ;;
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
                                -k|--kill-after) shift; shift || break ;;
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
                                -u|-g|-h|-p|-r|-t|-C|-c|--user|--group|--host|--prompt|--role|--type|--closefrom)
                                    shift; shift || break ;;
                                --) shift; break ;;
                                -*) shift ;;
                                *) break ;;
                            esac
                        done
                        ;;
                    stdbuf)
                        deny "Blocked: stdbuf dispatches another command and cannot be inspected by the absolute-rule guard. Write the command out directly."
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
                function|case)
                    deny "Blocked: shell function definitions cannot be inspected by the absolute-rule guard. Write the command out directly."
                    ;;
                *'()'|*'(){'*)
                    [[ ${2:-} = '{' ]] && deny "Blocked: shell function definitions cannot be inspected by the absolute-rule guard. Write the command out directly."
                    [[ "$first" = *'(){'* ]] && deny "Blocked: shell function definitions cannot be inspected by the absolute-rule guard. Write the command out directly."
                    ;;
            esac
            [[ ${2:-} = '()' && ${3:-} = '{' ]] && deny "Blocked: shell function definitions cannot be inspected by the absolute-rule guard. Write the command out directly."
            [[ ${2:-} = '(){'* ]] && deny "Blocked: shell function definitions cannot be inspected by the absolute-rule guard. Write the command out directly."
            case "$first" in
                /bin/bash|/bin/sh|/bin/zsh) first=${first##*/} ;;
            esac
            case "$first" in
                fi|'esac'|done|'}'|')') return 0 ;;
            esac
            if [[ "$first" = */* ]]; then
                vet_executable "$first" || deny_untracked "$first"
            else
                vet_bare_executable "$first"
            fi
            case "${first##*/}" in
                cd)
                    # keep resolution honest for `cd backend && ../scripts/x.sh`
                    next=${2:-.}
                    next=$(clean_token "$next")
                    if moved=$(cd -P "$curdir" 2>/dev/null && cd -P "$next" 2>/dev/null && pwd); then
                        curdir=$moved
                    fi
                    return 0
                    ;;
                export | readonly | declare | typeset | read)
                    for next in "$@"; do
                        case "$(clean_token "$next")" in
                            PATH|PATH=*|PATH+=*)
                                deny "Blocked: PATH can change which executable runs. Use the configured toolchain path directly."
                                ;;
                        esac
                    done
                    ;;
                printf)
                    shift
                    while [[ $# -gt 0 ]]; do
                        case "$(clean_token "$1")" in
                            -v) shift; [[ ${1:-} != PATH ]] || deny "Blocked: PATH can change which executable runs. Use the configured toolchain path directly." ;;
                        esac
                        shift || break
                    done
                    ;;
                eval)
                    deny "Blocked: eval builds its command at runtime and cannot be inspected by the absolute-rule guard. Write the command out directly."
                    ;;
                trap)
                    deny "Blocked: trap handlers execute later and cannot be inspected by the absolute-rule guard. Write the command out directly."
                    ;;
                pushd|popd)
                    deny "Blocked: $first changes the executable resolution directory and cannot be inspected by the absolute-rule guard. Use cd with an explicit tracked path instead."
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
                    if [[ "${tok##*/}" = run-go-toolchain.sh ]]; then
                        shift
                        [[ $# -gt 0 ]] || return 0
                        vet_toolchain_request "$@"
                    fi
                    ;;
                python | python3 | node | nodejs | ruby | perl)
                    shift
                    while [[ ${1:-} == -* ]]; do
                        case "$1" in
                            -c|-e|--command|--eval)
                                deny "Blocked: inline interpreter payloads cannot be inspected by the absolute-rule guard. Write the tracked script path out directly."
                                ;;
                            -p|--print)
                                deny "Blocked: inline interpreter payloads cannot be inspected by the absolute-rule guard. Write the tracked script path out directly."
                                ;;
                            -m|--module)
                                shift
                                [[ ${1:-} = venv ]] || deny "Blocked: interpreter modules cannot be inspected by the absolute-rule guard. Write the tracked script path out directly."
                                return 0
                                ;;
                            -V|-v|-h|--version|--help) return 0 ;;
                        esac
                        shift || break
                    done
                    [[ $# -gt 0 ]] || deny "Blocked: an interpreter needs a tracked script path; stdin and REPL payloads cannot be inspected by the absolute-rule guard."
                    tok=$(clean_token "$1")
                    reject_dynamic_executable "$tok"
                    vet_script "$tok" || deny_untracked "$tok"
                    ;;
                xargs)
                    deny "Blocked: xargs builds commands from runtime input and cannot be inspected by the absolute-rule guard. Write the command out directly."
                    ;;
                find)
                    for next in "$@"; do
                        case "$(clean_token "$next")" in
                            -exec|-execdir)
                                deny "Blocked: find -exec builds commands from runtime arguments and cannot be inspected by the absolute-rule guard. Write the command out directly."
                                ;;
                        esac
                    done
                    ;;
                go)
                    shift
                    vet_go_command "$@"
                    ;;
                *.sh)
                    vet_script "$first" || deny_untracked "$first"
                    ;;
                *)
                    return 0
                    ;;
            esac

            # run-go-toolchain executes a slash-containing requested command;
            # vet that executable position, not arbitrary .sh-looking input.
            if [[ "${first##*/}" = run-go-toolchain.sh ]]; then
                shift
                [[ $# -gt 0 ]] || return 0
                vet_toolchain_request "$@"
            fi
        }

        # Recursively visit shell command substitutions, while processing
        # separators only outside quotes. This is deliberately a lexer, never
        # an evaluator: the command supplied to the hook is never executed.
        scan_commands() {
            local text=$1 segment='' quote='' ch next inner inner_quote depth i j len entry_curdir=$curdir
            len=${#text}
            i=0
            while (( i < len )); do
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
                if [[ ( "$ch" = '<' || "$ch" = '>' ) && "$next" = '(' ]]; then
                    deny "Blocked: process substitutions cannot be inspected by the absolute-rule guard. Write the command out directly."
                fi
                if [[ "$ch" = '$' && "$next" = '(' ]]; then
                    inner=''
                    inner_quote=''
                    depth=1
                    j=$((i + 2))
                    while (( j < len && depth > 0 )); do
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
                            if (( depth > 0 )); then inner+=$ch; fi
                        else
                            inner+=$ch
                        fi
                        j=$((j + 1))
                    done
                    (( depth == 0 )) || deny "Blocked: an unterminated command substitution cannot be inspected by the absolute-rule guard."
                    scan_commands "$inner"
                    curdir=$entry_curdir
                    segment+='__guard_substitution__'
                    i=$j
                    continue
                fi
                if [[ "$ch" = '`' ]]; then
                    inner=''
                    j=$((i + 1))
                    while (( j < len )) && [[ ${text:j:1} != '`' ]]; do
                        inner+=${text:j:1}
                        j=$((j + 1))
                    done
                    (( j < len )) || deny "Blocked: an unterminated command substitution cannot be inspected by the absolute-rule guard."
                    scan_commands "$inner"
                    curdir=$entry_curdir
                    segment+='__guard_substitution__'
                    i=$((j + 1))
                    continue
                fi
                if [[ "$quote" = '"' ]]; then
                    case "$ch" in
                        ';'|'|'|'&'|$'\n')
                            segment+=$ch
                            i=$((i + 1))
                            continue
                            ;;
                    esac
                fi
                case "$ch" in
                    ';'|$'\n')
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
