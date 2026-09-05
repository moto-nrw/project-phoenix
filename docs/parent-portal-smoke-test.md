# Parent Portal — Local Smoke Test

How to verify PR 9 end-to-end on your dev machine. Should take ~10 minutes.

---

## Prerequisites

### A. DNS — nothing to do

`parents.localhost` resolves automatically to `127.0.0.1` because `.localhost` is a reserved TLD (RFC 6761). Modern browsers + Windows handle this without any hosts-file edit. The exact same way `operator.localhost:3000` already works for you.

If for some reason it doesn't resolve (older OS, restricted network), the fallback is to add `127.0.0.1 parents.localhost` to `C:\Windows\System32\drivers\etc\hosts` (Notepad as admin). But you almost certainly won't need to.

### B. Env vars

The new env vars need to be in your local `.env`:

```bash
# In your project root .env (NOT .env.example):
NEXT_PUBLIC_PARENTS_HOSTNAME=parents.localhost:3000
PARENTS_URL=http://parents.localhost:3000
```

If they're missing, the frontend's `next build` will fail and the proxy will throw at startup.

### C. Restart Docker so the new env vars are picked up

```bash
docker compose restart server frontend
```

Watch the frontend logs once for either:
- ✅ `Ready in ...ms` — proxy started cleanly
- ❌ `NEXT_PUBLIC_PARENTS_HOSTNAME is not set` — go back to step B

---

## The smoke test

### Step 1 — Confirm a known guardian account exists

You need an account that:
- Has a password set (not still in "pending invite" state)
- Has the `guardian` role on at least one school

The simplest way: accept the existing pending guardian invite from earlier in the conversation. If you already did that, skip to step 2.

```bash
# Check what guardian invites are still open:
docker compose exec -T postgres psql -U postgres -c \
  "SELECT id, guardian_profile_id, accepted_at, expires_at FROM auth.guardian_invitations ORDER BY id DESC;"
```

If there's an unaccepted row (`accepted_at IS NULL` and `expires_at > now()`):

1. Get the token: `docker compose exec -T postgres psql -U postgres -c "SELECT token FROM auth.guardian_invitations WHERE id = <id>;"`
2. Open in browser: `http://localhost:3000/accept-guardian-invite/<token>`
3. Set a password — use something memorable like `Test1234!`

If no invite is open and you need a new one:
1. Log into the admin portal (`{tenant}.localhost:3000/login`) as an admin
2. Go to a parent's profile in **Erziehungsberechtigte** → click "Einladung senden"
3. Copy the URL from `platform.email_outbox.payload->>'invitation_url'` (since SMTP isn't sending in dev)

After this step you should know:
- An email address (e.g. `test@mail.de`)
- A password (whatever you just set)

---

### Step 2 — Try the parent login

1. In your browser, go to: `http://parents.localhost:3000/login`

   Expected: a login page with the heading "Willkommen im Eltern-Portal".

   ❌ If you see a 404 / Next.js error: the proxy isn't rewriting. Check `docker compose logs frontend` for env var errors. Most likely `NEXT_PUBLIC_PARENTS_HOSTNAME` is missing or set to a different value than what your browser is hitting.

2. Enter the email + password from step 1.
3. Click "Anmelden".

   Expected: redirect to `http://parents.localhost:3000/` showing "Übersicht Ihrer Kinder", with the child(ren) linked to that account listed under their school's name.

   ❌ If you see "Anmeldung nicht möglich. Bitte prüfen Sie Ihre Zugangsdaten…":
   - Wrong password → re-set it via the accept-invite link
   - Account has no guardian role anywhere → the policy refuses you. Check `docker compose logs server | grep "Not a guardian"` to confirm

---

### Step 3 — Confirm the children list rendered

Look at the dashboard. You should see:

- Page title: **Übersicht Ihrer Kinder**
- Subtitle: **"1 Kind angemeldet."** or similar count
- A green button top-right: **"+ Neue Anmeldung"** (clicking it 404s for now — the embedded enrollment is PR 11)
- One **white card per school** with the school's name as a heading
- Inside each card, one **row per child** with:
  - Name
  - Class (or "Anmeldung läuft")
  - A coloured pill: green = Aktiv, blue = Anmeldung läuft, gray = Beendet

❌ If the page is blank or shows an error:
- Open the browser dev tools (F12) → Network tab → reload
- Look for the request to `/api/parent/me/children`. Should be 200 with a JSON array
- If 401 → your session didn't get a `parent` scope JWT. Try logging out and back in
- If 500 → check `docker compose logs server | tail -50`

---

### Step 4 — Click into a child

Click any row. You land on `http://parents.localhost:3000/children/<id>`.

Expected:
- Page title: child's full name
- Subtitle: school name + class
- Coloured status pill on the right
- A "Betreuung" card with status, school, class, enrollment dates
- A footer note: "Weitere Funktionen ... folgen in einer kommenden Version."
- A "← Zur Übersicht" link at the top that returns to the dashboard

---

### Step 5 — Verify portal isolation (the security check)

This is the most important step — proves a parent token can't reach the staff side.

1. Stay logged into `parents.localhost:3000`. Open a **new tab** (same browser, same session).
2. Try to hit a tenant URL: `http://demo-school.localhost:3000/dashboard` (replace `demo-school` with whatever slug you use locally).

   Expected: redirected to the tenant login page. The parent session cookie is **invisible** here because it's host-only on `parents.localhost`.

3. Try the inverse — log into a tenant subdomain as a normal staff user, then try `http://parents.localhost:3000/`. Expected: redirected to the parents login page (the staff cookie is invisible here too).

4. Try a guardian-only account at the tenant login: `http://demo-school.localhost:3000/login` with `test@mail.de` + password.

   Expected: login fails with a 403 — you need to read the response in dev tools (the UI may just show "Ungültige Anmeldedaten"). Check `docker compose logs server | grep "Guardian-only"` to confirm the policy fired.

---

### Step 6 — Verify a parent token can't smuggle into a tenant API

Get a parent JWT (look in dev tools → Application → Cookies → `parent.session-token`, or just log in via the API directly):

```bash
curl -s http://localhost:8080/parent/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"test@mail.de","password":"<your-password>"}' \
  | jq -r '.data.access_token'
# Save the output — that's $TOKEN
```

Try using it on a tenant endpoint:

```bash
curl -i http://localhost:8080/api/students \
  -H "Authorization: Bearer $TOKEN"
```

Expected: **HTTP 401** with body `{"error":"unauthorized","message":"unauthorized"}`. This is `jwt.TenantMiddleware` rejecting `scope=parent`.

Try the same token on the parent endpoint:

```bash
curl -i http://localhost:8080/parent/me/children \
  -H "Authorization: Bearer $TOKEN"
```

Expected: **HTTP 200** with the same children list the dashboard shows.

---

## What to capture if something fails

If any step misbehaves, paste these into a chat message and I can diagnose:

```bash
# Last 100 lines of server logs:
docker compose logs --tail=100 server

# Last 100 lines of frontend logs:
docker compose logs --tail=100 frontend

# Current outbox state:
docker compose exec -T postgres psql -U postgres -c \
  "SELECT id, kind, status, attempts, last_error FROM platform.email_outbox ORDER BY id DESC LIMIT 5;"

# Account-tenants for the parent's email:
docker compose exec -T postgres psql -U postgres -c \
  "SELECT a.id, a.email, at.tenant_id, at.status, ar.role_id, r.name AS role_name
   FROM auth.accounts a
   LEFT JOIN auth.account_tenants at ON at.account_id = a.id
   LEFT JOIN auth.account_roles ar ON ar.account_id = a.id AND ar.tenant_id = at.tenant_id
   LEFT JOIN auth.roles r ON r.id = ar.role_id
   WHERE a.email = 'test@mail.de';"
```

---

## PR 11 — Embedded enrollment for logged-in parents

PR 11 adds the "Neue Anmeldung" path inside the parents portal: cross-tenant
school picker + the existing public enrollment form embedded under the parent
session, with autofill, captcha skipped, and the invitation queue
short-circuited when the request gets approved.

Run these steps after the PR 9 smoke test passes.

### Step 9 — School picker

1. Log in at `http://parents.localhost:3000/parents/login`.
2. From the dashboard, click **+ Neue Anmeldung** (top-right).
3. URL should change to `/parents/anmeldung`. The page lists every (school, open
   phase) pair the parent could submit a new enrollment to.
4. Schools where the parent already has a child show a green
   **"Bereits ein Kind angemeldet"** badge and appear at the top of the list.

If the list is empty, no school has an open enrollment phase right now —
either set `ph.is_active=true` and clear `enrollment_close_at`, or open a phase
in the admin UI under Anmeldungen → Phasen.

### Step 10 — Embedded form + autofill

1. Click any phase card. URL becomes `/parents/anmeldung/{slug}/{phaseId}`.
2. The same form the public path uses appears, but **guardian fields are
   prefilled** (first name, last name, email, primary phone if set on the
   guardian profile in that school).
3. Existing children already linked to the guardian appear as a panel above
   the child cards — clicking **Übernehmen** copies first/last/grade into a
   form slot.
4. The captcha widget should be **absent** — parent JWT is the trust signal.

If autofill is missing, check that `/api/parent/enrollments/{slug}/profile`
returns 200 (DevTools → Network).

### Step 11 — Authenticated submit + skip-invite

1. Fill in any missing required fields (DOB, consents) and submit.
2. The browser routes to `/anmeldung/status/{token}?submitted=1` (the public
   status URL the form generates — same as the public path).
3. Inspect the resulting `enrollment.requests` row:
   ```bash
   docker compose exec -T postgres psql -U postgres -c \
     "SELECT id, guardian_email, guardian_account_id, submitted_at
      FROM enrollment.requests ORDER BY id DESC LIMIT 1;"
   ```
   `guardian_account_id` should match the parent's account id (NOT NULL).
4. Approve the request from the admin UI (or via the decision endpoint).
5. Outbox should NOT contain a fresh `guardian_invitation` row for this
   parent — the decision service skipped the invitation because the
   `guardian_account_id` was already attached:
   ```bash
   docker compose exec -T postgres psql -U postgres -c \
     "SELECT id, kind, status, created_at FROM platform.email_outbox
      WHERE kind = 'guardian_invitation' ORDER BY id DESC LIMIT 5;"
   ```
   The most recent row should be older than the just-submitted request.
6. The new tenant should appear in `auth.account_tenants` for the parent's
   account with `status = 'active'`.

If you DO see a fresh invitation row, the by-ID attach silently fell through
to the email path — check the server logs for
`decision: linked approval to existing global account` and confirm the
`via_request_account_id=true` field is set.
