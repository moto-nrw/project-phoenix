---
name: migrate-reset
description: Use when resetting databases, running migrations, or encountering migration errors. Triggers on "migrate reset", "database reset", "clean slate", "betterauth migrate", "migration order", or migration failures.
metadata:
  author: moto-nrw
  version: "1.0.0"
---

# Database Migration Reset

This skill documents the correct order for resetting both BetterAuth and Go backend databases.

## Critical: Migration Order

**Go backend depends on BetterAuth tables.** Always follow this order:

```
1. Drop BetterAuth tables (public schema)
       ↓
2. BetterAuth migrate (creates tables Go needs)
       ↓
3. Go migrate reset (can now reference BetterAuth tables)
```

## Quick Commands

### Full Reset (Development)

```bash
# Step 1: Drop BetterAuth tables
docker compose exec postgres psql -U postgres -d postgres -c \
  "DROP TABLE IF EXISTS public.member, public.invitation, public.organization, public.session, public.account, public.verification, public.\"user\" CASCADE;"

# Step 2: BetterAuth migrate (requires env vars)
cd betterauth && \
DATABASE_URL="postgres://postgres:postgres@localhost:5432/postgres?sslmode=require" \
BASE_DOMAIN="localhost:3000" \
NODE_TLS_REJECT_UNAUTHORIZED=0 \
pnpm run migrate

# Step 3: Go migrate reset
cd ../backend && go run main.go migrate reset
```

### BetterAuth Only

```bash
cd betterauth && \
DATABASE_URL="postgres://postgres:postgres@localhost:5432/postgres?sslmode=require" \
BASE_DOMAIN="localhost:3000" \
NODE_TLS_REJECT_UNAUTHORIZED=0 \
pnpm run migrate
```

### Go Backend Only

```bash
cd backend && go run main.go migrate reset
```

## Environment Variables for BetterAuth

| Variable | Value | Why |
|----------|-------|-----|
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/postgres?sslmode=require` | PostgreSQL connection with SSL |
| `BASE_DOMAIN` | `localhost:3000` | Required by email utilities |
| `NODE_TLS_REJECT_UNAUTHORIZED` | `0` | Accept self-signed SSL certs |

## Common Errors

### "BASE_DOMAIN environment variable is required"
**Fix:** Pass `BASE_DOMAIN="localhost:3000"` inline with the command.

### "self-signed certificate in certificate chain"
**Fix:** Add `NODE_TLS_REJECT_UNAUTHORIZED=0` before the command.

### "pg_hba.conf rejects connection... no encryption"
**Fix:** Use `sslmode=require` in DATABASE_URL (not `sslmode=disable`).

### "relation does not exist" during Go migration
**Cause:** BetterAuth tables missing. Go migrations depend on them.
**Fix:** Run BetterAuth migrate first.

## Database Schema Overview

| System | Schema | Tables |
|--------|--------|--------|
| BetterAuth | `public` | user, session, account, verification, organization, member, invitation |
| Go Backend | `auth`, `users`, `education`, `facilities`, `activities`, `active`, `schedule`, `iot`, `feedback`, `config`, `meta`, `audit`, `tenant` | Various domain tables |

## After Reset

Don't forget to seed test data:
```bash
cd backend && go run main.go seed
```
