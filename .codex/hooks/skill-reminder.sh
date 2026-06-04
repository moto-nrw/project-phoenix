#!/usr/bin/env bash
# UserPromptSubmit hook - reminds Claude to use skills actively
# Scans project-local .claude/skills and .claude/commands directories

set -euo pipefail

# Find project root
project_root=$(git rev-parse --show-toplevel 2>/dev/null) || exit 0

skills=()
skill_roots=(
    "$project_root/.claude/skills"
    "$project_root/.agents/skills"
    "$HOME/.codex/skills"
    "$HOME/.claude/skills"
    "$HOME/.agents/skills"
    "$HOME/repos/dotfiles/claude/.claude/skills"
)

if [[ -n "${CODEX_SKILLS_PATH:-}" ]]; then
    IFS=':' read -r -a extra_skill_roots <<< "$CODEX_SKILLS_PATH"
    skill_roots+=("${extra_skill_roots[@]}")
fi

if [[ -n "${CLAUDE_SKILLS_PATH:-}" ]]; then
    IFS=':' read -r -a extra_skill_roots <<< "$CLAUDE_SKILLS_PATH"
    skill_roots+=("${extra_skill_roots[@]}")
fi

if [[ -n "${AGENT_SKILLS_PATH:-}" ]]; then
    IFS=':' read -r -a extra_skill_roots <<< "$AGENT_SKILLS_PATH"
    skill_roots+=("${extra_skill_roots[@]}")
fi

# Scan project-local and user-global skills.
while IFS= read -r skill_path; do
    skill_name=$(basename "$(dirname "$skill_path")")
    skills+=("$skill_name")
done < <(
    for skill_root in "${skill_roots[@]}"; do
        if [[ -d "$skill_root" ]]; then
            find "$skill_root" -name "SKILL.md" -type f 2>/dev/null
        fi
    done | sort -u
)

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
while IFS= read -r skill; do
    if [[ -z "$skill" ]]; then
        continue
    fi
    skill_list="${skill_list}
- ${skill}"
done < <(printf '%s\n' "${skills[@]}" | sort -u)

# Output compact JSON reminder. Use jq so embedded newlines are escaped.
additional_context="<skill-reminder>USE SKILLS ACTIVELY! Available:${skill_list}

Invoke via Skill tool before acting.</skill-reminder>"

jq -n --arg context "$additional_context" '{
  hookSpecificOutput: {
    hookEventName: "UserPromptSubmit",
    additionalContext: $context
  }
}'
