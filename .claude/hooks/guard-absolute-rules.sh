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

        # Rule 1: network commands against production domains. Reading the
        # domain string in code (rg/grep/cat) stays allowed.
        if printf '%s' "$cmd" | grep -Eq 'moto-app\.de|moto\.nrw'; then
            if printf '%s' "$cmd" | grep -Eq '(^|[|&;([:space:]])(curl|wget|xh|httpie|https?|nc|ncat|wscat|fetch)([[:space:]]|$)'; then
                deny "Blocked: HTTP requests against moto-app.de / moto.nrw are forbidden (.claude/rules/no-production-requests.md). Use localhost; only a human may target production."
            fi
        fi

        # Rule 2 (shell half): writing into SOPS files outside the sops CLI.
        if printf '%s' "$cmd" | grep -q '\.sops\.env'; then
            if ! printf '%s' "$cmd" | grep -Eq '^[[:space:]]*sops([[:space:]]|$)'; then
                if printf '%s' "$cmd" | grep -Eq '(sed[[:space:]]+-i|>>?|(^|[|&;[:space:]])(tee|mv|cp)[[:space:]])'; then
                    deny "Blocked: environments/*.sops.env must only be edited with the sops CLI (sops environments/<env>.sops.env). See CLAUDE.md, Environment Management (SOPS)."
                fi
            fi
        fi

        # Rule 4: --no-verify bypasses lefthook incl. the sops encryption
        # check. Scoped to the same pipeline segment as "git commit"; a
        # literal " -n " inside a commit message is an accepted false
        # positive.
        if printf '%s' "$cmd" | grep -Eq 'git[[:space:]]+commit[^|;&]*[[:space:]](--no-verify|-n)([[:space:]]|$)'; then
            deny "Blocked: git commit --no-verify / -n skips lefthook (incl. the sops encryption guard). Commit without it; if a hook misfires, fix the hook."
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
