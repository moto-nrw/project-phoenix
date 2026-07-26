# Parent Portal — Production Rollout Checklist

**Status:** The parent portal is public under `eltern.{TENANT_DOMAIN}` in deployed environments. The internal Next.js namespace remains `/parents/*`; `parents.*` is kept only as a compatibility redirect alias.

Use this checklist when verifying deployed configuration, DNS, and reverse proxy routing.

---

## Staging

### 1. Verify `NEXT_PUBLIC_PARENTS_HOSTNAME` in staging.sops.env

The frontend env validation (`frontend/src/env.js`) requires this var. Without it, `next build` fails inside the staging Docker build.

```bash
sops environments/staging.sops.env
# Set:
NEXT_PUBLIC_PARENTS_HOSTNAME=eltern.staging.moto-app.de
# Save the file — sops re-encrypts on save.
```

### 2. Verify `PARENTS_URL` in staging.sops.env

Backend uses this for parent-facing email links (status, accept-invite, decision emails). The server refuses to start when it is missing, and production must use HTTPS.

```bash
sops environments/staging.sops.env
# Set:
PARENTS_URL=https://eltern.staging.moto-app.de
```

### 3. Verify env-check passes

```bash
./scripts/env-check.sh
```
Should print `✅` for both checks. CI runs this on every PR.

### 4. DNS for `eltern.staging.moto-app.de`

Whoever manages staging DNS needs to point `eltern.staging.moto-app.de` at the same server as the rest of the staging hosts (operator, tenant subdomains). Likely a wildcard `*.staging.moto-app.de` already covers it — verify with:
```bash
dig eltern.staging.moto-app.de
```

### 5. Caddy / reverse proxy config for parents subdomain

If staging uses Caddy or another reverse proxy with explicit per-host blocks, add a block for `eltern.staging.moto-app.de` matching the operator pattern (TLS cert, forward to the Next.js container). If it uses a wildcard cert + catch-all, no canonical-host block is needed.

After the frontend image/env has switched to `eltern.staging.moto-app.de`, add a compatibility redirect while old links/bookmarks are still in circulation. Do not enable this before the frontend recognizes `eltern.*`, or the old deployment can redirect back to `parents.*`.

```caddy
parents.staging.moto-app.de {
    redir https://eltern.staging.moto-app.de{uri} 302
}
```

---

## Production

### 6. Verify both env vars in production.sops.env

Same as staging, with prod URLs:

```bash
sops environments/production.sops.env
# Set:
NEXT_PUBLIC_PARENTS_HOSTNAME=eltern.moto-app.de
PARENTS_URL=https://eltern.moto-app.de
```

### 7. DNS for `eltern.moto-app.de`

Same as staging — likely covered by wildcard, verify.

### 8. Caddy / reverse proxy for production parents subdomain

Same as staging — explicit block or wildcard for `eltern.moto-app.de`, plus a compatibility redirect after the production frontend image/env has switched to `eltern.moto-app.de`:

```caddy
parents.moto-app.de {
    redir https://eltern.moto-app.de{uri} 302
}
```

---

## Local development notes

### 9. Keep local development on `parents.localhost`

Local developers need:
- `NEXT_PUBLIC_PARENTS_HOSTNAME=parents.localhost:3000`
- `PARENTS_URL=http://parents.localhost:3000`
- A note that `docker compose restart server frontend` is required after adding the vars

### 10. Communicate the URL to existing test guardians

Once PR 9 merges, every existing pending guardian invitation will continue to point at the **old** `accept-guardian-invite` URL on the staff frontend (links are baked into the email_outbox row at enqueue time). New invites will go to `eltern.{TENANT_DOMAIN}`.

The accept page itself works on both — Next.js routes are unaffected. But the parent will land on the staff frontend after accept, which is wrong.

Two options:
- (a) Manually purge unsent rows: `DELETE FROM platform.email_outbox WHERE kind='guardian_invitation' AND status='pending';` then re-enqueue from the admin UI
- (b) Just live with it — old links go to staff frontend, new ones go to parents portal. Test guardians are dev-only per the user; production has none

User decision: option (b) is fine since prod has no real parent invites yet.

---

## ✅ Already done in PR 9

For reference — these don't need follow-up:

- `parents` and `eltern` slugs reserved in both backend + frontend lists
- Proxy subdomain handling
- All three NextAuth providers (tenant, operator, parents) wired with isolated cookies
- Backend login policy + scope guards
- Cross-tenant `/parent/me/children` endpoint
- Parent app shell + dashboard + child detail
- All parent-facing emails route to `PARENTS_URL`
- CLAUDE.md + frontend/CLAUDE.md updated

---

## Suggested verification order

1. PR merges to `development`.
2. Run `./scripts/env-check.sh`.
3. Verify staging via the smoke test (see `parent-portal-smoke-test.md`).
4. Merge `development` → `main` for production.
5. Verify production with the same smoke test against `eltern.moto-app.de`, and verify `parents.moto-app.de` redirects to it.
