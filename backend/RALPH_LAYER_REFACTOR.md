# Mission: Fix Layer Violations

## Your Goal

Make the backend architecture match `TARGET_ARCHITECTURE.mmd` in this directory.

The rule is simple: **Handler → Service → Repository → Database**

Handlers must NEVER directly access repositories or database connections.

## Your Memory: LEARNINGS.md

**CRITICAL: You have a memory file at `LEARNINGS.md` in this directory.**

### At the START of each iteration:
```bash
cat LEARNINGS.md 2>/dev/null || echo "No learnings yet"
```
Read it. Learn from your past attempts.

### During your work:
When you discover something important, append it immediately:
```bash
echo "## $(date '+%Y-%m-%d %H:%M')" >> LEARNINGS.md
echo "- [discovery/mistake/insight here]" >> LEARNINGS.md
```

### What to record:
- ❌ **Mistakes**: "Forgot to update factory.go after adding service method"
- ✅ **Solutions**: "GroupSubstitution repo is used for X, moved to EducationService"
- 🔍 **Discoveries**: "api/students imports 3 different repos, all for privacy consent"
- ⚠️ **Gotchas**: "BUN ORM requires quoted aliases in ModelTableExpr"
- 📝 **Progress**: "Fixed: repoFactory.DataImport → ImportService.GetImportStatus()"

This file persists across iterations. Future you will thank past you.

---

## Your Tools

Use these commands to discover violations:

```bash
# Generate dependency graph (finds who imports what)
cd /Users/yonnock/Developer/moto/project-phoenix-refactor-layers/backend
goda graph "./..." > deps.dot

# Find API layer importing repositories (VIOLATION)
grep -E "api.*->.*repositories" deps.dot

# Find API layer importing database directly (VIOLATION)
grep -E "api.*->.*database\"" deps.dot

# Check specific package dependencies
depth ./api/...
depth ./services/...

# Search for direct repository usage in API layer
grep -r "repoFactory\." api/
grep -r "db \*bun.DB" api/
```

## Your Process

1. **Read** `LEARNINGS.md` — learn from previous iterations
2. **Read** `TARGET_ARCHITECTURE.mmd` — understand the goal
3. **Run** the analysis commands — find ALL violations yourself
4. **Understand** each violation — WHY is it there? What does the handler use it for?
5. **Fix** one violation at a time:
   - Add needed method to appropriate service
   - Update handler to use service instead
   - Remove repository/db from handler constructor
6. **Verify** after each fix: `go test ./...`
7. **Record** what you learned in `LEARNINGS.md`
8. **Repeat** until no violations remain

## Verification

You are done when:
```bash
# Only ONE occurrence (the factory creation itself)
grep -c "repoFactory\." api/base.go  # Must be 1

# Zero direct db usage in handlers
grep -r "db \*bun.DB" api/           # Must be empty

# All tests pass
go test ./...                         # Must pass
```

## Completion

When ALL violations are fixed and tests pass, output:

<promise>LAYER_VIOLATIONS_FIXED</promise>

---

**Start by reading LEARNINGS.md, then TARGET_ARCHITECTURE.mmd, then discover the violations yourself.**
