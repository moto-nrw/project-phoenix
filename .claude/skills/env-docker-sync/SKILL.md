---
name: env-docker-sync
description: Use when adding, removing, or renaming environment variables, modifying docker-compose files, or when lefthook flags env sync warnings. Triggers on .env changes, docker-compose edits, or "missing keys" errors.
metadata:
  author: moto-nrw
  version: "1.0.0"
---

# Environment & Docker Sync

Ensures all env files, docker-compose configs, and their tracked templates stay in sync.

## When to Use

- Adding/removing/renaming an environment variable
- Modifying `docker-compose.yml` or `docker-compose.example.yml`
- Seeing lefthook "missing keys" warnings after `git pull`
- Adding a new service or configuration to Docker

## File Relationships

Read the rule for the full inventory: `.claude/rules/env-docker-sync.md`

**Key principle**: Docker Compose runs all services together using the **root `.env`** for `${VAR}` substitution. `backend/dev.env` is only for local `go run main.go serve` outside Docker.

## Process: Adding a Backend Env Var

1. **Determine access pattern** in Go code:
   - `viper.GetString("key")` → works from both dev.env and docker environment
   - `os.Getenv("KEY")` → ONLY works from docker environment block or shell; does NOT read dev.env

2. **Update tracked templates** (these get committed):
   ```
   backend/dev.env.example     → add KEY=placeholder
   docker-compose.example.yml  → add KEY: ${KEY:-default} under server environment
   .env.example                → add KEY=placeholder
   ```

3. **Update local files** (these stay git-ignored):
   ```
   backend/dev.env             → add KEY=actual_value
   docker-compose.yml          → add KEY: ${KEY:-default} under server environment
   .env                        → add KEY=actual_value
   ```

4. **Verify sync**:
   ```bash
   dotenv-linter diff .env .env.example
   dotenv-linter diff backend/dev.env backend/dev.env.example
   dyff between --omit-header docker-compose.example.yml docker-compose.yml
   ```

## Process: Adding a Frontend Env Var

1. **Update tracked templates**:
   ```
   frontend/.env.example       → add KEY=placeholder
   docker-compose.example.yml  → add KEY: ${KEY:-default} under frontend environment (if needed in Docker)
   .env.example                → add KEY=placeholder (if needed in Docker)
   ```

2. **Update local files**:
   ```
   frontend/.env.local         → add KEY=actual_value
   docker-compose.yml          → add under frontend environment (if needed in Docker)
   .env                        → add actual value (if needed in Docker)
   ```

3. **Verify sync**:
   ```bash
   dotenv-linter diff frontend/.env.local frontend/.env.example
   ```

## Process: Fixing Sync Warnings

When lefthook reports `X is missing keys: Y, Z`:

1. Read the example file to find the expected keys and default values
2. Add missing keys to the local file with appropriate values
3. If the example is missing keys from the local file, update the example too (this is a tracked file — commit it)
4. Re-run the diff to confirm clean

## Verification Command (Run All)

```bash
dotenv-linter diff .env .env.example 2>/dev/null | grep "missing" || echo "OK: .env"
dotenv-linter diff backend/dev.env backend/dev.env.example 2>/dev/null | grep "missing" || echo "OK: backend/dev.env"
dotenv-linter diff frontend/.env.local frontend/.env.example 2>/dev/null | grep "missing" || echo "OK: frontend/.env.local"
dyff between --omit-header docker-compose.example.yml docker-compose.yml 2>/dev/null; echo "---"
```

## Common Mistakes

| Mistake | Consequence |
|---------|-------------|
| Add var to `dev.env` only, skip docker-compose | Migration/scheduler code using `os.Getenv()` won't see it in Docker |
| Add var to `docker-compose.yml` but not `.env` | `${VAR}` resolves to empty string |
| Add var to local files but not examples | Next developer gets sync warnings; no documentation of the var |
| Add var to `.env.example` but not `docker-compose.example.yml` | Var exists but isn't passed to any container |
