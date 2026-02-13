---
name: fix-docker-dependencies
description: Diagnose and fix "Module not found" errors caused by stale Docker dependency caches. Use when a dependency was added/removed/swapped but the Docker container still has old node_modules.
metadata:
  author: moto-nrw
  version: "1.0.0"
---

# Fix Docker Dependency Sync Issues

Use this when the frontend Docker container throws `Module not found: Can't resolve '<package>'` but `package.json` and `pnpm-lock.yaml` are correct.

## Root Cause

Docker caches the `pnpm install --frozen-lockfile` layer in the `dev-deps` build stage. When dependencies change in `package.json`/`pnpm-lock.yaml`, Docker may reuse a stale cached layer containing old `node_modules`. This is especially aggressive with `BUILDKIT_INLINE_CACHE: 1`.

`docker compose up --build` only checks file hashes — if the lockfile hasn't changed since the last build, Docker reuses the cached install. This fails when switching packages (e.g., `better-auth` to `next-auth`).

## Diagnosis Steps

### 1. Confirm the package is in package.json and lockfile

```bash
cd frontend
grep "<package>" package.json
grep "<package>" pnpm-lock.yaml | head -5
```

Both should return results. If not, run `pnpm install` first.

### 2. Check local node_modules

```bash
ls node_modules/<package>/ 2>&1 | head -5
```

If missing locally, run `pnpm install`.

### 3. Check inside the running Docker container

```bash
docker compose exec frontend ls node_modules/<package>/ 2>&1 | head -5
```

If missing inside the container but present locally, the Docker cache is stale.

## Fix Steps

### Quick Fix (usually sufficient)

```bash
# 1. Sync local node_modules
cd frontend && pnpm install

# 2. Rebuild Docker without cache
docker compose build --no-cache frontend

# 3. Recreate the container
docker compose up -d frontend

# 4. Verify
docker compose exec frontend ls node_modules/<package>/
```

### Nuclear Fix (if quick fix doesn't work)

```bash
# 1. Clean everything locally
cd frontend
rm -rf node_modules .next
pnpm install

# 2. Full Docker reset
cd ..
docker compose down
docker builder prune -f
docker compose build --no-cache frontend
docker compose up -d

# 3. Verify
docker compose exec frontend ls node_modules/<package>/
```

## Key Takeaway

- `docker compose up --build` reuses cached layers — it does NOT reinstall dependencies.
- `docker compose build --no-cache <service>` forces a fresh install.
- Always verify inside the container, not just locally: `docker compose exec frontend ls node_modules/<pkg>/`.

## Prevention

After changing dependencies in `package.json`, always rebuild with `--no-cache`:

```bash
pnpm install && docker compose build --no-cache frontend && docker compose up -d frontend
```
