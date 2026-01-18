#!/usr/bin/env bash
# UserPromptSubmit hook - reminds Claude to use skills actively

set -euo pipefail

# Find all SKILL.md files and extract skill names
skills=""
while IFS= read -r skill_path; do
    # Extract plugin and skill name from path
    # Pattern: .../cache/<marketplace>/<plugin>/<version>/skills/<skill-name>/SKILL.md
    # Or: .../skills/<skill-name>/SKILL.md
    skill_name=$(basename "$(dirname "$skill_path")")

    # Get plugin name if in cache structure
    if [[ "$skill_path" == *"/cache/"* ]]; then
        plugin_name=$(echo "$skill_path" | sed -E 's|.*/cache/[^/]+/([^/]+)/.*|\1|')
        skills="${skills}${plugin_name}:${skill_name}, "
    else
        skills="${skills}${skill_name}, "
    fi
done < <(find "$HOME/.claude/plugins/cache" -name "SKILL.md" -type f 2>/dev/null | sort)

# Remove trailing comma and space
skills="${skills%, }"

# Output compact JSON reminder
cat <<EOF
{
  "hookSpecificOutput": {
    "hookEventName": "UserPromptSubmit",
    "additionalContext": "<skill-reminder>USE SKILLS ACTIVELY! Available: ${skills}. Invoke via Skill tool before acting.</skill-reminder>"
  }
}
EOF
