#!/usr/bin/env bash
# UserPromptSubmit hook - reminds Claude about available subagents
# Scans project-local .claude/agents directory

set -euo pipefail

# Find project root
project_root=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0

agents=()

json_escape() {
    local value=$1
    value=${value//\\/\\\\}
    value=${value//\"/\\\"}
    value=${value//$'\n'/\\n}
    value=${value//$'\r'/\\r}
    value=${value//$'\t'/\\t}
    printf '%s' "$value"
}

# Scan project-local agents (.claude/agents/*.md)
if [[ -d "$project_root/.claude/agents" ]]; then
    while IFS= read -r agent_path; do
        if [[ -f "$agent_path" ]]; then
            agent_name=$(basename "$agent_path" .md)
            agents+=("$agent_name")
        fi
    done < <(find "$project_root/.claude/agents" -maxdepth 1 -name "*.md" -type f 2>/dev/null | sort)
fi

# Exit silently if no agents found
if [[ ${#agents[@]} -eq 0 ]]; then
    exit 0
fi

# Build vertical list
agent_list=""
for agent in "${agents[@]}"; do
    agent_list="${agent_list}
- ${agent}"
done

# Output compact JSON reminder
additional_context="<subagent-reminder>Project subagents (use Task tool with subagent_type):${agent_list}

Invoke via Task tool when relevant.</subagent-reminder>"

printf '{\n'
printf '  "hookSpecificOutput": {\n'
printf '    "hookEventName": "UserPromptSubmit",\n'
printf '    "additionalContext": "%s"\n' "$(json_escape "$additional_context")"
printf '  }\n'
printf '}\n'
