---
description: Run full quality checks (lint + typecheck) for frontend
allowed-tools: Bash(cd:*), Bash(npm run:*)
---

# Frontend Quality Check

Run comprehensive code quality checks for frontend. **MUST pass before
committing (zero warnings policy).**

## What This Checks

1. **ESLint**: Code style and TypeScript patterns
   - Unused variables must start with `_`
   - Type imports must use `import type`
   - No console.log in production code
   - All TypeScript recommended rules

2. **TypeScript**: Type checking
   - Strict mode compliance
   - No type errors
   - Proper type inference

## Execution

```bash
cd frontend && npm run check
```

This runs both checks in sequence:

- `next lint` (ESLint with 0 warnings policy)
- `tsc --noEmit` (TypeScript type checking)

## Fix Issues

Auto-fix linting issues:

```bash
cd frontend && npm run lint:fix
```

Auto-format code:

```bash
cd frontend && npm run format:write
```

## Expected Output

**Success:**

```
✓ No ESLint warnings
✓ TypeScript compilation successful
```

**Failure:**

```
❌ 3 warnings found
❌ 2 type errors
```

## Before Committing

Always run this command before creating a commit. Project enforces zero warnings
policy.

```bash
cd frontend
npm run check          # MUST pass
npm run format:write   # Auto-format
git add <files>
git commit -m "..."
```

## CI/CD Integration

This same check runs in GitHub Actions. If it fails locally, it will fail in CI.
