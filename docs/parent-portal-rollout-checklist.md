# Parent Portal — Production Rollout Checklist

**Status:** PR 9 (`feat/enrollment` branch) introduces the parent portal at `parents.{TENANT_DOMAIN}`. This document lists everything that **must** happen before staging or production deploys can succeed.

Each item is something I (Claude) couldn't do locally because it requires the SOPS age key, DNS access, or a stage-only side effect.

---

## ❌ Blocks staging deploy

### 1. Add `NEXT_PUBLIC_PARENTS_HOSTNAME` to staging.sops.env

The frontend env validation (`frontend/src/env.js`) requires this var. Without it, `next build` fails inside the staging Docker build.

```bash
sops environments/staging.sops.env
# Add a line:
NEXT_PUBLIC_PARENTS_HOSTNAME=parents.staging.moto-app.de
# Save the file — sops re-encrypts on save.
```

### 2. Add `PARENTS_URL` to staging.sops.env

Backend uses this for parent-facing email links (status, accept-invite, decision emails). Falls back to `FRONTEND_URL` if missing, but the wrong URL would land parents on the staff login.

```bash
sops environments/staging.sops.env
# Add a line:
PARENTS_URL=https://parents.staging.moto-app.de
```

### 3. Verify env-check passes

```bash
./scripts/env-check.sh
```
Should print `✅` for both checks. CI runs this on every PR.

### 4. DNS for `parents.staging.moto-app.de`

Whoever manages staging DNS needs to point `parents.staging.moto-app.de` at the same server as the rest of the staging hosts (operator, tenant subdomains). Likely a wildcard `*.staging.moto-app.de` already covers it — verify with:
```bash
dig parents.staging.moto-app.de
```

### 5. Caddy / reverse proxy config for parents subdomain

If staging uses Caddy or another reverse proxy with explicit per-host blocks, add a block for `parents.staging.moto-app.de` matching the operator pattern (TLS cert, forward to the Next.js container). If it uses a wildcard cert + catch-all, no change needed.

---

## ❌ Blocks production deploy

### 6. Add both env vars to production.sops.env

Same as staging, with prod URLs:

```bash
sops environments/production.sops.env
# Add:
NEXT_PUBLIC_PARENTS_HOSTNAME=parents.moto-app.de
PARENTS_URL=https://parents.moto-app.de
```

### 7. DNS for `parents.moto-app.de`

Same as staging — likely covered by wildcard, verify.

### 8. Caddy / reverse proxy for production parents subdomain

Same as staging — explicit block or wildcard.

---

## ⚠️ Should be done before users hit the portal

### 9. Update `getting-started.md` for local dev

Local developers need:
- The hosts file entry (`127.0.0.1 parents.localhost`)
- The two new env vars (`NEXT_PUBLIC_PARENTS_HOSTNAME`, `PARENTS_URL`) added to their personal `.env`
- A note that `docker compose restart server frontend` is required after adding the vars

### 10. Communicate the URL to existing test guardians

Once PR 9 merges, every existing pending guardian invitation will continue to point at the **old** `accept-guardian-invite` URL on the staff frontend (links are baked into the email_outbox row at enqueue time). New invites will go to `parents.{TENANT_DOMAIN}`.

The accept page itself works on both — Next.js routes are unaffected. But the parent will land on the staff frontend after accept, which is wrong.

Two options:
- (a) Manually purge unsent rows: `DELETE FROM platform.email_outbox WHERE kind='guardian_invitation' AND status='pending';` then re-enqueue from the admin UI
- (b) Just live with it — old links go to staff frontend, new ones go to parents portal. Test guardians are dev-only per the user; production has none

User decision: option (b) is fine since prod has no real parent invites yet.

---

## ✅ Already done in PR 9

For reference — these don't need follow-up:

- `parents` slug reserved in both backend + frontend lists
- Proxy subdomain handling
- All three NextAuth providers (tenant, operator, parents) wired with isolated cookies
- Backend login policy + scope guards
- Cross-tenant `/parent/me/children` endpoint
- Parent app shell + dashboard + child detail
- All parent-facing emails route to `PARENTS_URL`
- CLAUDE.md + frontend/CLAUDE.md updated

---

## Suggested order for a clean deploy

1. PR 9 merges to `development` — staging deploy will fail at the `next build` step until step 1+2 are done.
2. Run steps 1+2 (sops edits) on `development`. Push. Staging deploy retries.
3. Verify staging via the smoke test (see `parent-portal-smoke-test.md`).
4. Merge `development` → `main` for production. Production deploy will fail until step 6 is done.
5. Run step 6. Push. Production deploy retries.
6. Verify production with the same smoke test against `parents.moto-app.de`.
