---
description: Validate database migrations and check dependencies
allowed-tools: Bash(cd:*), Bash(go run:*), Bash(docker compose:*)
---

# Validate Database Migrations

Check migration status, validate dependencies, and identify issues before running migrations.

## What This Does

1. Shows migration status (pending/applied)
2. Validates migration dependencies
3. Checks for circular dependencies
4. Verifies migration files are valid Go code

## Execution

### Check Status

```bash
cd backend && go run main.go migrate status
```

Shows:
- Applied migrations with timestamps
- Pending migrations not yet run
- Migration dependency tree

### Validate Dependencies

```bash
cd backend && go run main.go migrate validate
```

Checks for:
- Missing dependencies
- Circular dependency chains
- Invalid dependency versions
- Orphaned migrations

## Using Docker

If backend not running locally:

```bash
docker compose exec server ./main migrate status
docker compose exec server ./main migrate validate
```

## Before Running Migrations

Always validate first:

```bash
# 1. Check what will be applied
go run main.go migrate status

# 2. Validate dependencies
go run main.go migrate validate

# 3. If all clear, run migrations
go run main.go migrate

# 4. Verify migrations applied
go run main.go migrate status
```

## Creating New Migrations

When adding a new migration:

1. **Specify dependencies correctly**:
   ```go
   var Dependencies = []string{
       "3.0.1",  // Must reference existing migration version
   }
   ```

2. **Test rollback function**:
   ```go
   var Rollback = `DROP TABLE IF EXISTS table_name CASCADE;`
   ```

3. **Validate before committing**:
   ```bash
   go run main.go migrate validate
   ```

## Common Issues

**Issue**: "Migration X depends on Y which doesn't exist"
**Fix**: Check dependency version numbers match existing migrations

**Issue**: "Circular dependency detected"
**Fix**: Review dependency chain, remove circular reference

**Issue**: "Migration file has syntax errors"
**Fix**: Run `go build ./database/migrations` to check Go syntax
