#!/usr/bin/env bash
# UserPromptSubmit hook - reminds Claude to use skills actively
# Scans project-local .claude/skills and .claude/commands directories

set -euo pipefail

# Find project root
project_root=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0

skills=()

json_escape() {
    local value=$1
    value=${value//\\/\\\\}
    value=${value//\"/\\\"}
    value=${value//$'\n'/\\n}
    value=${value//$'\r'/\\r}
    value=${value//$'\t'/\\t}
    printf '%s' "$value"
}

# Scan project-local skills (.claude/skills/*/SKILL.md)
if [[ -d "$project_root/.claude/skills" ]]; then
    while IFS= read -r skill_path; do
        skill_name=$(basename "$(dirname "$skill_path")")
        skills+=("$skill_name")
    done < <(find "$project_root/.claude/skills" -name "SKILL.md" -type f 2>/dev/null | sort)
fi

# Scan project-local commands (.claude/commands/*.md)
if [[ -d "$project_root/.claude/commands" ]]; then
    while IFS= read -r cmd_path; do
        # Skip subdirectories, only process direct .md files
        if [[ -f "$cmd_path" ]]; then
            cmd_name=$(basename "$cmd_path" .md)
            skills+=("$cmd_name")
        fi
    done < <(find "$project_root/.claude/commands" -maxdepth 1 -name "*.md" -type f 2>/dev/null | sort)
fi

# Build vertical list
skill_list=""
for skill in "${skills[@]}"; do
    skill_list="${skill_list}
- ${skill}"
done

# Output compact JSON reminder
additional_context="<skill-reminder>USE SKILLS ACTIVELY! Available:${skill_list}

Invoke via Skill tool before acting.</skill-reminder>"

printf '{\n'
printf '  "hookSpecificOutput": {\n'
printf '    "hookEventName": "UserPromptSubmit",\n'
printf '    "additionalContext": "%s"\n' "$(json_escape "$additional_context")"
printf '  }\n'
printf '}\n'
