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
#    and chains in seed-e2e-multi-tenant.sh, so you do NOT need a
#    running dev stack and you do NOT need a second manual step.
./scripts/seed-e2e.sh

# 3. Run the tests. Playwright starts its own Next.js dev server on :3030
#    (so it can coexist with your dev frontend on :3000) and points it at
#    server-e2e (:8081). Nothing of yours needs to be stopped.
cd frontend
pnpm exec playwright install chromium      # first time only
pnpm exec playwright test
```

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

1. Decide the role: most tests are admin-only → put them anywhere under `e2e/`.
2. Staff-only tests must live in a file matching `*.staff.spec.ts` so the `chromium-staff` project picks them up.
3. Use `test, expect` from `../fixtures`, not `@playwright/test`.
4. Prefer asserting on names/labels (`getByText`, `getByRole`) over IDs — the seeder's IDs are technically stable, but names are robust against future seeder changes.

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
