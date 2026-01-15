# RALPH LOOP (Codex Examples)

## Purpose
Concrete, explicit examples of a safe, repeatable loop that enforces 12‑Factor compliance first, then hexagonal migration.

---

## 0) Preconditions (HARD STOP if not met)

```bash
# Must be in backend/ directory
pwd | grep -q "/backend$" || { echo "ERROR: run from backend/"; exit 1; }

# Must be clean working tree to avoid unrelated commits
if [ -n "$(git status --porcelain)" ]; then
  echo "ERROR: working tree dirty. Commit/stash first."; exit 1
fi

# Required tools
command -v rg >/dev/null || { echo "ERROR: ripgrep (rg) missing"; exit 1; }
command -v go >/dev/null || { echo "ERROR: go toolchain missing"; exit 1; }
# deadcode is optional; handled later
```

---

## 1) Analyze (explicit, robust commands)

```bash
# 1.1 Largest files (skip tests), safe for spaces
find . -name "*.go" -type f ! -name "*_test.go" -print0 | xargs -0 wc -l | sort -nr | head -10

# 1.2 Dead code (if tool exists)
if command -v deadcode >/dev/null; then
  deadcode ./... 2>/dev/null | rg -v "test/" || true
else
  echo "deadcode not installed; skipping"
fi

# 1.3 12-Factor scans
rg -n "localhost|127\.0\.0\.1|http://" --glob="*.go" --glob="!**/*_test.go" .
rg -n "os\.Create|os\.OpenFile|ioutil\.WriteFile" --glob="*.go" --glob="!**/*_test.go" --glob="!**/migrations/*" .
rg -n "log\.Printf|log\.Println|log\.Fatal" --glob="*.go" --glob="!**/*_test.go" .

# 1.4 Hardcoded defaults in config libs
rg -n "SetDefault\(" --glob="*.go" .

# 1.5 Hex rule check (after moves)
# go list -f '{{.ImportPath}}: {{.Imports}}' ./internal/core/... | rg "adapter" || true
```

---

## 2) Research (explicit)

```bash
# Open hex/clean references (manual read)
# https://dev.to/bagashiz/building-restful-api-with-hexagonal-architecture-in-go-1mij
# https://threedots.tech/post/introducing-clean-architecture/

# 12-Factor read list
# https://12factor.net/config
# https://12factor.net/logs
# https://12factor.net/processes
```

---

## 3) Identify ONE problem (explicit criteria)

Examples:
- A file > 800 lines in core/service -> split into smaller files
- Any hardcoded URL or localhost -> replace with required ENV
- Any local filesystem writes for persistent data -> move behind port + adapter
- Any core importing adapter -> fix import boundary

Decision template:
```
Problem: <describe>
Why highest impact: <reason>
Files: <list>
```

---

## 4) Implement (example workflows)

### Example A — Remove hardcoded config
```
# Find usage
rg -n "FRONTEND_URL|http://" internal/core/service

# Implement required env + fail fast
# (edit file)
```

### Example B — Move package into core/service
```
# Move package directory
mv services/active internal/core/service/active

# Update imports
rg -l "github.com/moto-nrw/project-phoenix/services" -g"*.go" | \
  xargs perl -pi -e 's|github.com/moto-nrw/project-phoenix/services|github.com/moto-nrw/project-phoenix/internal/core/service|g'
```

### Example C — Enforce adapter boundary
```
# Identify violations
go list -f '{{.ImportPath}}: {{.Imports}}' ./internal/core/... | rg "adapter"
```

---

## 5) Verify (explicit)

```bash
# Format if needed
gofmt -w $(rg -l "" --glob="*.go" internal/core internal/adapter)

# Build + tests
go build ./...
go test ./... -short
```

If failing, fix and re-run.

---

## 6) Commit (explicit)

```bash
git add -A
git commit -m "refactor: <short description>"
```

---

## 7) Log iteration (explicit)

```bash
COMMIT_HASH=$(git rev-parse HEAD)
{
  echo "## Iteration $(date +%Y-%m-%d_%H:%M:%S)"
  echo ""
  echo "**Changed:** <what changed>"
  echo ""
  echo "**Files:** <list>"
  echo ""
  echo "**Commit:** ${COMMIT_HASH}"
  echo ""
  echo "---"
  echo ""
} >> TASKS.md
```

---

## 8) Learnings (optional)

```bash
# Add only if you learned something non-obvious
# echo "- $(date +%Y-%m-%d): <learning>" >> LEARNINGS.md
```

---

## 9) End Iteration

```
ITERATION COMPLETE
Changed: <what>
Next: <next focus>
```

---

## Notes
- Never change API contracts or DB schema in this loop.
- 12‑Factor violations have priority over hex migration.
- Keep all changes in a single iteration cohesive.
