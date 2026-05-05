# Playwright E2E Tests

End-to-end tests run against the real Next.js dev server and Go backend, in a real Chromium browser.

## Quick start

The E2E suite runs against an **isolated backend stack** (`postgres-test` +
`server-e2e`, host port 8081) that is completely separate from the dev
stack. Your dev DB and dev backend are never touched.

```bash
# 1. ONE-TIME: add E2E hostnames to /etc/hosts so the browser can reach them.
#    Most macOS resolvers don't return *.localtest.me, even though it's a
#    public DNS wildcard. The script adds explicit 127.0.0.1 entries.
sudo ./scripts/setup-e2e-hosts.sh

# 2. Bring up the isolated E2E backend + reset + seed (single tenant +
#    second tenant for the switch flow). This script does
#    `docker compose up -d --wait postgres-test server-e2e` internally
#    and invokes the Go seeder with --with-second-tenant, so you do NOT
#    need a running dev stack and you do NOT need a second manual step.
./scripts/seed-e2e.sh

# 3. Run the tests. Playwright starts its own Next.js dev server on :3030
#    (so it can coexist with your dev frontend on :3000) and points it at
#    server-e2e (:8081). Nothing of yours needs to be stopped.
cd frontend
pnpm exec playwright install chromium      # first time only
pnpm run e2e                               # wraps playwright test + E2E_BACKEND_URL
```

> `pnpm run e2e` is a thin wrapper around `playwright test` that sets the
> required `E2E_BACKEND_URL=http://localhost:8081` env var. The bare
> `pnpm exec playwright test` will refuse to start without it (no fallback,
> per the project's "No fallbacks" rule in CLAUDE.md). If you prefer the
> bare invocation, export `E2E_BACKEND_URL` yourself.

The HTML report opens automatically on failure; otherwise: `pnpm exec playwright show-report`.

## Architecture

```
e2e/
├── helpers/
│   ├── seed-data.ts        Constants: tenant slug, base URL, credentials
│   └── auth.ts             loginViaUI() — drives the login form once
├── auth.setup.ts           Setup project: logs in per role, saves storageState
├── fixtures.ts             Custom test() with adminPage / staffPage fixtures
├── flows/                  Feature tests, organised by domain
├── .auth/                  (gitignored) Saved session cookies per role
└── screenshots/            (gitignored) Screenshots from failures / debug
```

The flow is:

1. **Setup project** runs once per test suite. It opens the login page, signs in as each role, and saves the resulting browser storage (cookies, localStorage) to `e2e/.auth/{role}.json`.
2. **Test projects** (`chromium-admin`, `chromium-staff`) declare the setup as a dependency and load the matching `storageState`. Every test in those projects starts already authenticated — no login UI is ever shown.
3. **Cross-role tests** (e.g. "admin creates X, staff sees X") use the `adminPage` / `staffPage` fixtures from `fixtures.ts`, which spawn fresh contexts backed by the same storageState files.

## Fixtures

Use the `test` export from `e2e/fixtures.ts`, not from `@playwright/test`:

```typescript
import { test, expect } from "../fixtures";

test("admin sees the dashboard", async ({ authenticatedPage: page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Übersicht" })).toBeVisible();
});

test("admin creates a group, staff sees it", async ({ adminPage, staffPage }) => {
  await adminPage.goto("/groups/new");
  await adminPage.fill('input[name="name"]', "Sterne");
  await adminPage.click('button:has-text("Anlegen")');

  await staffPage.goto("/groups");
  await expect(staffPage.getByText("Sterne")).toBeVisible();
});
```

| Fixture             | Behaviour                                                                |
| ------------------- | ------------------------------------------------------------------------ |
| `authenticatedPage` | Project default `page`, already logged in as that project's role.        |
| `adminPage`         | Fresh context as admin (`demo1@mail.de`), regardless of project.         |
| `staffPage`         | Fresh context as staff/user (`demo11@mail.de`), regardless of project.   |

For HTTP-only specs (no browser, no auth fixture — e.g. `iot-api.spec.ts`, `checkin.spec.ts`, the API halves of CRUD tests) use `apiTest` / `apiExpect` from `../fixtures`. Same Playwright `test` and `expect`, just re-exported under different names so every spec file goes through `fixtures.ts` and the README's "no `@playwright/test` imports in specs" rule holds across the whole suite.

```typescript
import { apiTest as test, apiExpect as expect } from "../fixtures";

test("returns 401 without auth", async ({ request }) => { /* ... */ });
```

## Test credentials

These come from `helpers/seed-data.ts` and must stay in sync with `scripts/seed-e2e.sh`:

| Role  | Email             | Backend role | Source            |
| ----- | ----------------- | ------------ | ----------------- |
| admin | `demo1@mail.de`   | admin        | `DemoStaff[0]`    |
| staff | `demo11@mail.de`  | user         | `DemoStaff[10]`   |

The password (`E2EPass1234!`) is set via the seeder's `--staff-password` flag and applies to all 20 demo accounts. The tenant subdomain is `demo-school.localtest.me:3030`.

## Ports

| Port | What | When |
| ---- | ---- | ---- |
| `:3000` | Your dev frontend (`docker compose up frontend`) → dev backend `:8080` → dev DB | normal development |
| `:3030` | E2E frontend, spawned by Playwright → `server-e2e` `:8081` → `postgres-test` | only while tests run |
| `:8080` | Dev backend | always |
| `:8081` | `server-e2e` (E2E backend) | always (in `e2e` profile) |

The two stacks are fully independent. You can keep your dev frontend running on `:3000` and run the E2E suite at the same time — both backends use different DBs (`postgres` vs `postgres-test`), so test runs cannot leak into your dev work, and vice versa. Open both URLs side by side in your browser if you need to compare behavior.

## How to add a test

The recipe below is the only way to add a spec — every shortcut here came from a real bug in an earlier iteration of the suite.

### 1. Decide UI vs API and pick the right `test` import

| Shape | Import | Default `page` is… |
|---|---|---|
| UI spec (drives a browser) | `import { test, expect } from "../fixtures"` | already authenticated as the project's role |
| HTTP-only spec (no browser, no auth fixture) | `import { apiTest as test, apiExpect as expect } from "../fixtures"` | n/a — use `request` instead |

Never `import { test } from "@playwright/test"`. Going through `fixtures.ts` is what gives you the factories below; bypassing it loses them silently.

### 2. Pick the right role

- Admin-only specs → file goes anywhere under `e2e/`. The `chromium-admin` project picks it up automatically.
- Staff-only specs → file MUST end in `.staff.spec.ts` (e.g. `myroom-supervision.staff.spec.ts`). The `chromium-staff` project's `testMatch` is wired to that pattern.

### 3. Use the factories — never `try/finally`

For any spec that creates rows, use `studentFactory` / `groupFactory` from the fixture:

```typescript
test("admin creates X and the row appears", async ({
  authenticatedPage: page,
  studentFactory,
}) => {
  // create + auto-cleanup
  const student = await studentFactory.create({ school_class: "3a" });

  // ... assertions ...

  // If the spec creates entities OUTSIDE the factory (e.g. via UI form),
  // hand the resulting id back so the fixture deletes it on teardown:
  // studentFactory.track(idFromUI);
});
```

The fixture deletes whatever the factory tracked at the end of the test — including on assertion failure. Do NOT add `try/finally` blocks for cleanup; that pattern was retired because it duplicated the fixture's job and broke when an `await` threw before the `finally` registered. Suffix collisions across parallel workers are also handled (factory uses `randomUUID()`, not `Date.now()`).

### 4. Use `helpers/routes.ts` for navigation, not raw paths

```typescript
import * as routes from "../helpers/routes";

await page.goto(routes.studentsList);
await page.goto(routes.studentDetail(student.id));
await page.goto(routes.groupsList);
```

This survives the kebab-case migration that's still in flight (see CLAUDE.md → URL & Route Conventions). Hardcoded `page.goto("/database/students")` works today but means a future rename has to grep every spec file.

### 5. Selector strategy (in order of preference)

1. **`getByRole({ name })`** — anything with a stable accessible name (buttons, headings, links). Works for icon-only buttons that have `aria-label` (e.g. `Schüler erstellen`).
2. **`locator('input[name="…"]')`** — form inputs that have a `name` attribute (most config-driven forms do, e.g. groups).
3. **`getByPlaceholder(…)`** — last-resort for inputs without `name` or `htmlFor` labels (e.g. the Klasse field on the Stammdaten tab). Brand copy can change without warning, so prefer 1 or 2 when available.
4. **`data-testid`** — NOT used in this codebase yet. Don't add testids to production JSX without a code-review-level discussion. If a real selector collision shows up, talk to the team first.

For modals, scope every interaction through the dialog so the modal's controls don't collide with same-named controls on the page behind it:

```typescript
const dialog = page.getByRole("dialog");
await expect(dialog).toBeVisible();
await dialog.getByPlaceholder("Max").fill(firstName);
await dialog.getByRole("button", { name: "Erstellen" }).click();
```

### 6. Wait for the right signal, not the easiest one

`useDeleteConfirmation.confirmDelete` (and many similar handlers) close the modal **synchronously** and fire the underlying request fire-and-forget. That means `await expect(dialog).toBeHidden()` resolves long before the backend has actually deleted the row — checking the API at that point will race and falsely see the row as still existing.

Wait for a domain signal instead:

```typescript
// Wait for the row to leave the list (delete + SWR revalidation = done)
await expect(page.getByText(fullName)).toHaveCount(0, { timeout: 15000 });

// THEN check the API
const res = await ctx.get(`${BACKEND_URL}/api/students/${id}`, {
  headers, failOnStatusCode: false,
});
expect(res.status()).toBe(404);
```

### 7. Run `pnpm run check` before committing

The repo's `oxlint .` and `tsc --noEmit` already include `e2e/**/*.ts`, so a typo in a helper or a stale type from a backend response shape change will fail the gate before you push. There's no separate `pnpm e2e:check` to remember.

### 8. Stable test data — never assume a seeded id

Seeded entity ids shift whenever `backend/seed/api/data.go` is reordered. Tests must read what they need through `helpers/seed-state.ts`:

```typescript
import { getStudentByIndex, getRoomId, getDeviceApiKey } from "../helpers/seed-state";

const student = getStudentByIndex(0);  // never `id: 1`
const roomId = getRoomId("OGS-Raum 1"); // never `roomId: 5`
```

Same rule for entity names: prefer `getTwoDistinctStudents()` over hardcoding `"Felix Schneider"`.

## Why we use `localtest.me` instead of `localhost`

The bare domain shows a tenant selector, not a login form (see `src/app/page.tsx`). The login lives at `{slug}.TENANT_DOMAIN/`, so `baseURL` is `http://demo-school.localtest.me:3030`.

`localtest.me` is a public DNS wildcard that resolves any subdomain to `127.0.0.1`, no `/etc/hosts` edits required. We use it instead of `*.localhost` because browsers treat `*.localhost` as separate hosts: NextAuth's session cookie ends up host-scoped, and tenant switching cannot share sessions across subdomains. With `localtest.me`, NextAuth sets the cookie on `.localtest.me` (`server/auth/tenant-config.ts:121-156`) and the switcher works locally exactly like in production.

## Troubleshooting

| Symptom                                                  | Likely cause                                                                |
| -------------------------------------------------------- | --------------------------------------------------------------------------- |
| Setup test times out on `input[name="email"]`            | Wrong subdomain — `baseURL` should be `http://demo-school.localtest.me:3030`. |
| `EADDRINUSE :3030` when starting Playwright              | Something else (a previous abandoned `pnpm run dev`?) is on :3030. Kill it: `lsof -nP -iTCP:3030 -sTCP:LISTEN`, then `kill <pid>`. |
| `DNS_PROBE_FINISHED_NXDOMAIN` for `*.localtest.me`       | No internet — `localtest.me` resolves over public DNS. Add `127.0.0.1 demo-school.localtest.me second-school.localtest.me` to `/etc/hosts` for offline use. |
| Setup test fails on "Anmelden" with "Ungültige Eingabe"  | Seeder hasn't been run, or `--staff-password` was different.                |
| `Executable doesn't exist` on first run                  | Run `pnpm exec playwright install chromium`.                                |
| Tests pass locally but fail in CI                        | Backend likely not seeded in CI yet — `scripts/seed-e2e.sh` must run first. |
| Cookies appear but tests still see the login form        | Stale `storageState` after seeder change — `rm -rf e2e/.auth` and re-run.   |
| `tenant-switch.spec.ts` fails on "TenantSwitcher visible" | Multi-tenant step in `seed-e2e.sh` did not run cleanly — re-run the seed.   |
