#!/usr/bin/env bash
# UserPromptSubmit hook - reminds the agent to use skills, scoped to the area
# being worked on.
#
# Repo-root and user-global skills are always listed. Area skills (frontend/,
# backend/) are listed only when that area looks active:
#   - the shell cwd is inside it, or
#   - the working tree has changes under it, or
#   - the prompt mentions it.
#
# Areas are discovered, not hardcoded: any top-level directory holding
# .claude/skills or .agents/skills counts.
#
# Kept bash-3.2 compatible (macOS /bin/bash).

set -euo pipefail

project_root=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0

# --- hook input -------------------------------------------------------------
# Optional. The bounded read means a caller that never closes stdin cannot hang
# the prompt, and a caller that sends no JSON just leaves the fields empty.
hook_input=""
if [[ ! -t 0 ]]; then
    IFS= read -r -d '' -t 1 hook_input || true
fi

prompt=""
cwd="$PWD"
if [[ -n "$hook_input" ]]; then
    parsed=$(printf '%s' "$hook_input" | jq -r '.prompt // empty' 2>/dev/null || true)
    if [[ -n "$parsed" ]]; then prompt="$parsed"; fi
    parsed=$(printf '%s' "$hook_input" | jq -r '.cwd // empty' 2>/dev/null || true)
    if [[ -n "$parsed" ]]; then cwd="$parsed"; fi
fi

# --- skill collection -------------------------------------------------------
# find(1) does not descend into symlinked directories, so a skill mirrored as
# .agents/skills/x -> ../../.claude/skills/x is counted once, whichever side
# holds the real files.
collect_skills() {
    local root
    for root in "$@"; do
        if [[ -d "$root" ]]; then
            find "$root" -name "SKILL.md" -type f 2>/dev/null
        fi
    done | while IFS= read -r skill_path; do
        # dirname + basename via parameter expansion - one fork per skill adds up
        skill_dir="${skill_path%/*}"
        printf '%s\n' "${skill_dir##*/}"
    done | sort -u
}

global_roots=(
    "$project_root/.claude/skills"
    "$project_root/.agents/skills"
    "$HOME/.codex/skills"
    "$HOME/.claude/skills"
    "$HOME/.agents/skills"
    "$HOME/repos/dotfiles/claude/.claude/skills"
)

for extra_path in "${CODEX_SKILLS_PATH:-}" "${CLAUDE_SKILLS_PATH:-}" "${AGENT_SKILLS_PATH:-}"; do
    if [[ -n "$extra_path" ]]; then
        IFS=':' read -r -a extra_roots <<< "$extra_path"
        global_roots+=("${extra_roots[@]}")
    fi
done

global_skills=$(collect_skills "${global_roots[@]}")

# Project-local commands are global too - they are not area-scoped.
if [[ -d "$project_root/.claude/commands" ]]; then
    commands=$(find "$project_root/.claude/commands" -maxdepth 1 -name "*.md" -type f 2>/dev/null |
        while IFS= read -r cmd_path; do
            cmd_file="${cmd_path##*/}"
            printf '%s\n' "${cmd_file%.md}"
        done | sort -u)
    global_skills=$(printf '%s\n%s\n' "$global_skills" "$commands" | sed '/^$/d' | sort -u)
fi

# --- area detection ---------------------------------------------------------
changed_files=$(git -C "$project_root" status --porcelain 2>/dev/null | cut -c4- || true)

# Extra prompt keywords per area; the directory name always matches too.
keywords_for() {
    case "$1" in
        frontend) echo 'ui|ux|component|komponente|react|next|tailwind|css|tsx|modal|dialog|button|screen|storybook|oberfläche|ansicht|seite|page|dashboard|formular|form|tabelle|table|icon|farbe|color|schrift|font|responsive|mobil|styling|layout|design|browser|screenshot' ;;
        backend)  echo 'golang|handler|service|repository|migration|sql|postgres|datenbank|database|endpoint|endpunkt|orm|bun|scheduler|rls|tenant|seeder' ;;
        *)        echo '' ;;
    esac
}

# Prints the reasons this area counts as active; returns 1 when it does not.
area_reasons() {
    local area="$1"
    local reasons=""
    local pattern extra

    if [[ "$cwd" == "$project_root/$area" || "$cwd" == "$project_root/$area/"* ]]; then
        reasons="cwd"
    fi

    if printf '%s\n' "$changed_files" | grep -q "^$area/"; then
        if [[ -n "$reasons" ]]; then reasons="$reasons, changed files"; else reasons="changed files"; fi
    fi

    if [[ -n "$prompt" ]]; then
        pattern="$area"
        extra=$(keywords_for "$area")
        if [[ -n "$extra" ]]; then pattern="$area|$extra"; fi
        if printf '%s' "$prompt" | grep -qiE "$pattern"; then
            if [[ -n "$reasons" ]]; then reasons="$reasons, prompt"; else reasons="prompt"; fi
        fi
    fi

    if [[ -z "$reasons" ]]; then return 1; fi
    echo "$reasons"
}

# --- output -----------------------------------------------------------------
render_list() {
    printf '%s\n' "$1" | sed '/^$/d' | sed 's/^/- /'
}

body="USE SKILLS ACTIVELY!

Available now (repo root + global):
$(render_list "$global_skills")"

for area_dir in "$project_root"/*/; do
    area=$(basename "$area_dir")
    if [[ ! -d "$area_dir.claude/skills" && ! -d "$area_dir.agents/skills" ]]; then continue; fi

    reasons=$(area_reasons "$area") || continue
    area_skills=$(collect_skills "$area_dir.claude/skills" "$area_dir.agents/skills")
    if [[ -z "$area_skills" ]]; then continue; fi

    body="$body

Skills for $area/ (active: $reasons) - they live in $area/.claude/skills and load once you touch a file under $area/; until then, read the SKILL.md directly:
$(render_list "$area_skills")"
done

body="$body

Invoke via Skill tool before acting."

# Output compact JSON. Use jq so embedded newlines are escaped.
jq -n --arg context "<skill-reminder>$body</skill-reminder>" '{
  hookSpecificOutput: {
    hookEventName: "UserPromptSubmit",
    additionalContext: $context
  }
}'
