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
# Note: Codex intercepts only the shell tool in PreToolUse, so the Edit/Write
# halves of rules 2 and 3 guard the Claude side only; lefthook stays the
# backstop there.
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
        if printf '%s' "$cmd_lower" | grep -Eq '(^|[|&;[:space:]])(bash|sh)[[:space:]]+[^-[:space:]][^[:space:]]*\.sh([[:space:]]|$)|(^|[|&;[:space:]])\./[^[:space:]]+'; then
            deny "Blocked: script execution cannot be safely inspected by the absolute-rule guard. Run the command directly."
        fi
        if printf '%s' "$cmd_lower" | grep -Eq '(^|[|&;[:space:]])(python([0-9.]*)?|node|go[[:space:]]+run)[[:space:]]+'; then
            deny "Blocked: interpreter execution cannot be safely inspected by the absolute-rule guard. Run the command directly."
        fi
        if printf '%s' "$cmd_lower" | grep -Eq '(^|[|&;[:space:]])(eval|source|\.)([[:space:]]|$)'; then
            deny "Blocked: dynamic shell execution cannot be safely inspected by the absolute-rule guard. Run the command directly."
        fi
        if printf '%s' "$cmd_lower" | grep -Eq '(^|[|&;[:space:]])(bash|sh)[[:space:]]+-c([[:space:]]|$)'; then
            deny "Blocked: shell wrappers cannot be safely inspected by the absolute-rule guard. Run the command directly."
        fi
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
