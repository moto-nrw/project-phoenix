#!/usr/bin/env bash
# UserPromptSubmit: point to relevant skills without repeating the runtime catalog.
set -euo pipefail

git rev-parse --show-toplevel >/dev/null 2>&1 || exit 0

body='Use explicitly requested and task-relevant skills from the runtime catalog.
Load SKILL.md through the available skill tool, or read its file directly.
Repository skills: .agents/skills/ and .claude/skills/.
For backend or frontend work, also check that area’s .claude/skills/ and
.agents/skills/; only read skills whose scope matches the task.
Do not load the whole catalog. An unavailable Skill tool is not a blocker.'

jq -cn --arg context "<skill-reminder>$body</skill-reminder>" '{
  hookSpecificOutput: {
    hookEventName: "UserPromptSubmit",
    additionalContext: $context
  }
}'
