---
name: ralph-setup
description: Use when user mentions ralph loop, wants to run iterative AI development, needs to set up autonomous coding loops, or asks about continuous iteration patterns
---

# Ralph Loop Setup

## Overview

Ralph is an external bash loop that repeatedly feeds a prompt file to an AI agent, allowing iterative improvement until completion. The agent sees its previous work in files and git history.

**Core principle:** Worktree isolation + task file + proper shell command = safe autonomous iteration.

## When to Use

- Setting up a new ralph loop
- Executing an existing ralph loop
- Troubleshooting a stuck loop
- Understanding ralph loop patterns

## Quick Reference

| Tool | Command (fish shell) |
|------|---------------------|
| Claude Code | `while true; cat TASK.md \| claude --dangerously-skip-permissions; end` |
| Codex | `while true; codex exec "$(cat TASK.md)" --dangerously-bypass-approvals-and-sandbox --enable web_search_request; end` |

## Setup Checklist

**REQUIRED BACKGROUND:** Use superpowers:using-git-worktrees skill first.

### 1. Create Worktree (MANDATORY)

Ralph loops make many commits and changes. **Never run in your main workspace.**

```bash
# Create isolated worktree
git worktree add .worktrees/ralph-task -b ralph/task-name
cd .worktrees/ralph-task
```

### 2. Create Task File

Create `RALPH_LOOP_TASK.md` with this structure:

```markdown
# RALPH LOOP: [Task Name]

## Deine Aufgabe

Du bist in einer Loop. Jede Iteration: analysiere → implementiere → committe → beende.

**KEINE VORSCHLÄGE. NUR AUSFÜHREN.**

## Ziel

[Clear, measurable goal with completion criteria]

## Iteration ausführen

### 1. Analysiere
[Commands to assess current state]

### 2. Identifiziere EIN Problem
[Decision criteria for what to fix]

### 3. Implementiere
[Make the change - no discussion]

### 4. Verifiziere
```bash
go build ./...
go test ./... -short
```

### 5. Committe
```bash
git add -A
git commit -m "refactor: [description]"
```

### 6. End Iteration
Print:
```
ITERATION COMPLETE
Changed: [what]
Next: [what should be tackled next]
```

## Regeln

- Kein API-Contract ändern
- Kein Database-Schema ändern
- Tests nicht löschen
- Immer committen bevor Iteration endet
```

### 3. Start the Loop

**Fish shell (Claude Code):**
```fish
while true; cat RALPH_LOOP_TASK.md | claude --dangerously-skip-permissions; end
```

**Fish shell (Codex):**
```fish
while true; codex exec "$(cat RALPH_LOOP_TASK.md)" --dangerously-bypass-approvals-and-sandbox --enable web_search_request; end
```

### 4. Monitor Progress

```bash
# Check commits
git log --oneline -20

# Check iteration logs (if task file creates them)
tail -50 TASKS.md
```

### 5. Stop the Loop

Press `Ctrl+C` in the terminal running the loop.

## Task File Best Practices

### Clear Completion Criteria

```markdown
## Ziel

Alle Repository-Interfaces nach internal/core/port/ verschieben.

**Fertig wenn:**
- Alle *Repository interfaces in internal/core/port/
- Keine Repository-Interfaces mehr in models/
- go build ./... erfolgreich
```

### Single Focus Per Iteration

Each iteration should:
1. Find ONE problem
2. Fix that ONE problem
3. Verify
4. Commit
5. End

### Include Verification Commands

```markdown
### Verifiziere
```bash
go build ./...
go test ./... -short
go vet ./...
```
```

### Include Logging (Optional)

```markdown
### Log to TASKS.md
```bash
echo "## Iteration $(date +%Y-%m-%d_%H:%M:%S)" >> TASKS.md
echo "**Changed:** [what]" >> TASKS.md
echo "---" >> TASKS.md
```
```

## Common Mistakes

### Running in main workspace

- **Problem:** Ralph makes many commits, can pollute history
- **Fix:** Always use worktree

### No verification step

- **Problem:** Broken code gets committed
- **Fix:** Always include build/test commands

### Vague completion criteria

- **Problem:** Loop never knows when to "complete" naturally
- **Fix:** Define measurable success criteria

### Multiple changes per iteration

- **Problem:** Hard to debug, large commits
- **Fix:** ONE problem per iteration

## Cleanup After Ralph

When done, merge or cherry-pick useful commits:

```bash
# Return to main workspace
cd /path/to/main/workspace

# Cherry-pick specific commits
git cherry-pick abc123

# Or merge branch
git merge ralph/task-name

# Remove worktree
git worktree remove .worktrees/ralph-task
```

## Integration

**REQUIRED:** superpowers:using-git-worktrees (call BEFORE starting)

**Pairs with:** superpowers:finishing-a-development-branch (for cleanup)
