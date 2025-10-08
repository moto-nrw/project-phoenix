---
description: Rebuild and restart backend Docker container (required after Go code changes)
allowed-tools: Bash(docker compose:*)
---

# Rebuild Backend Container

**CRITICAL**: Docker hot reload not configured for Go. Container MUST be rebuilt after Go code changes.

## What This Does

1. Rebuilds backend Docker image with latest Go code
2. Restarts backend container
3. Verifies container is healthy

## Execution

```bash
docker compose build server && docker compose up -d server
```

## Verification

Check container status:

```bash
docker compose ps server
docker compose logs --tail=20 server
```

Look for:
- Container state: "Up" with health status "healthy"
- Log message: "Starting server on port 8080"
- No error messages in logs

## When to Use

Run this command EVERY TIME you change Go code:
- Modified files in `backend/` directory
- Changed Go dependencies (`go.mod`)
- Updated environment variables in `backend/dev.env`

## Alternative: Local Development

To avoid rebuilds, run backend locally:

```bash
cd backend
go run main.go serve
```

Then stop Docker backend container:

```bash
docker compose stop server
```
