# RALPH LOOP (Codex Examples)

## Purpose
Concrete, explicit examples of a safe, repeatable loop that enforces 12‑Factor compliance first, then hexagonal migration.

---

## 0) Preconditions (explicit, not strict)

```bash
# Ensure we are in backend/ (auto-cd if possible)
repo_root=$(git rev-parse --show-toplevel 2>/dev/null || true)
if [ -n "$repo_root" ] && [ -d "$repo_root/backend" ]; then
  cd "$repo_root/backend" || exit 1
else
  echo "WARN: could not locate repo root/backend; continuing in current dir"
fi

# If working tree is dirty, continue but record it later in TASKS
DIRTY_TREE="no"
if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
  DIRTY_TREE="yes"
  echo "WARN: working tree dirty; proceed but note in TASKS"
fi

# Required tools (fallbacks when possible)
if ! command -v rg >/dev/null; then
  echo "WARN: ripgrep (rg) missing; falling back to grep where used"
  RG="grep -R -n"
else
  RG="rg -n"
fi
command -v go >/dev/null || { echo "WARN: go toolchain missing; build/tests will be skipped"; NO_GO="yes"; }
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
$RG "localhost|127\\.0\\.0\\.1|http://" --glob="*.go" --glob="!**/*_test.go" .
$RG "os\\.Create|os\\.OpenFile|ioutil\\.WriteFile" --glob="*.go" --glob="!**/*_test.go" --glob="!**/migrations/*" .
$RG "log\\.Printf|log\\.Println|log\\.Fatal" --glob="*.go" --glob="!**/*_test.go" .

# 1.4 Hardcoded defaults in config libs
$RG "SetDefault\\(" --glob="*.go" .

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
if command -v gofmt >/dev/null; then
  gofmt -w $(rg -l "" --glob="*.go" internal/core internal/adapter)
else
  echo "WARN: gofmt missing; skipping format"
fi

# Build + tests
if [ "${NO_GO}" != "yes" ]; then
  go build ./...
  go test ./... -short
else
  echo "SKIP: go build/test (Go toolchain missing)"
fi
```

If failing, fix and re-run.

---

## 6) Log iteration (explicit, must be included in same commit)

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
  if [ "${DIRTY_TREE}" = "yes" ]; then
    echo ""
    echo "**Note:** working tree was dirty at start"
  fi
  echo ""
  echo "---"
  echo ""
} >> TASKS.md
```

---

## 7) Commit (explicit, include TASKS.md)

```bash
git add -A
git commit -m "refactor: <short description>"
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
