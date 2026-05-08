# Playwright E2E Tests

End-to-end tests run against the real Next.js dev server and Go backend, in a real Chromium browser.

## Quick start

The E2E suite runs against an **isolated backend stack** (`postgres-test` +
`server-e2e`, host port 8081) that is completely separate from the dev
stack. Your dev DB and dev backend are never touched.

```bash
# 1. ONE-TIME: install Chromium for Playwright.
cd frontend
pnpm exec playwright install chromium      # first time only

# 2. Golden path: backend reset + Go seeder scenario + Playwright run.
pnpm run e2e
```

`pnpm run e2e` is the one supported command for both local use and CI. It
delegates to the internal `../scripts/e2e.sh` orchestrator, which brings up
the isolated backend stack directly from `../docker-compose.example.yml`,
runs `go run main.go e2e prepare`, and then executes Playwright against the
emitted manifest contract. The only required local prerequisite is a valid
root `.env`.

The HTML report opens automatically on failure; otherwise: `pnpm exec playwright show-report`.

## Architecture

```
e2e/
├── contract.ts             Typed reader for the single Go-owned manifest + tenant URL builder
├── auth.ts                 Browser-auth helpers and storageState contract
├── auth.setup.ts           Setup project: verifies contract, logs in once per role, writes storageState
├── api.ts                  Session-backed API auth bridge (`storageState` -> `/api/auth/token` -> backend Bearer)
├── fixtures.ts             Playwright fixtures over scenario data, auth contexts, and a few shared harness helpers
├── flows/                  Feature tests, organised by domain
├── .auth/                  (gitignored) Saved session cookies per role
└── screenshots/            (gitignored) Screenshots from failures / debug
```

The flow is:

1. **Go-owned E2E prepare command** (`go run main.go e2e prepare`) resets the isolated DB, seeds the deterministic multi-tenant world, and writes one manifest: `backend/.e2e-manifest.json`.
2. **Manifest contract** includes both scenario data and harness runtime (`backend_url`, `tenant_domain`, `frontend_port`, operator host), so Playwright does not keep a second copy of those values in config constants.
3. **Setup project** reads that manifest, verifies the harness preconditions, signs in as each role, and saves the resulting browser `storageState` under `e2e/.auth/`.
4. **Contract layer** (`contract.ts`) is read-only: manifest validation, tenant URL building, nothing else.
5. **Auth layer** (`auth.ts` + `auth.setup.ts`) owns browser login, readiness checks, and `storageState`.
6. **API bridge** (`api.ts`) derives backend Bearer tokens from the setup-written browser session via `/api/auth/token`, so UI auth and API auth cannot drift apart.
7. **Fixtures** stay focused: `fixtures.ts` re-exposes typed scenario data, ready-to-use session-backed auth contexts, and the small reusable helpers that keep raw check-in/setup plumbing out of specs.
8. **Cross-role tests** (e.g. "admin creates X, staff sees X") use the `adminPage` / `staffPage` fixtures from `fixtures.ts`, which spawn fresh contexts backed by the same storageState files.

## Fixtures

Use the `test` export from `e2e/fixtures.ts`, not from `@playwright/test`:

```typescript
import { test, expect } from "../fixtures";
import * as routes from "../helpers/routes";

test("admin sees the dashboard", async ({ app, authenticatedPage: page }) => {
  await page.goto(app.primary(routes.root));
  await expect(page.getByRole("heading", { name: "Übersicht" })).toBeVisible();
});

test("admin creates a group, staff sees it", async ({
  app,
  adminPage,
  staffPage,
}) => {
  await adminPage.goto(app.primary(routes.groupsList));
  await adminPage.fill('input[name="name"]', "Sterne");
  await adminPage.click('button:has-text("Anlegen")');

  await staffPage.goto(app.primary(routes.groupsList));
  await expect(staffPage.getByText("Sterne")).toBeVisible();
});
```

| Fixture                   | Behaviour                                                                                                   |
| ------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `tenantSwitchScenario`    | Multi-tenant switch data: primary/secondary tenants plus canonical actor display name.                      |
| `studentSearchScenario`   | Seeded search/filter pair with stable expected names.                                                       |
| `groupVisibilityScenario` | Seeded visibility pair with stable expected group names.                                                    |
| `checkinScenario`         | Seeded RFID/check-in data: student, room, activity, RFID tag, device, supervisor.                           |
| `checkinDevice`           | Canonical seeded device credentials for raw IoT auth-path HTTP tests.                                       |
| `checkinHarness`          | High-level check-in/presence helper: canonical scan payload, current-visit lookup, isolated RFID prep/cleanup. |
| `app`                     | Tenant URL builder (`app.primary(...)`, `app.secondary(...)`, `app.tenant(...)`).                           |
| `authenticatedPage`       | Project default `page`, already logged in as that project's role.                                           |
| `backendApi`              | Raw backend API context with only `baseURL` set, for unauthenticated/negative cases.                        |
| `adminApi`                | Tenant-scoped admin API context; auth comes from the setup-written `storageState`, not a second login flow. |
| `staffApi`                | Tenant-scoped staff API context; auth comes from the setup-written `storageState`, not a second login flow. |
| `deviceApi`               | Device API context with Bearer key + `X-Staff-PIN` already set.                                             |
| `adminPage`               | Fresh context as the admin actor, regardless of project.                                                    |
| `staffPage`               | Fresh context as the staff actor, regardless of project.                                                    |

For HTTP-only specs (no browser — e.g. `iot-api.spec.ts`, `checkin.spec.ts`, the API halves of CRUD tests) use `apiTest` / `apiExpect` from `../fixtures`. Same Playwright `test` and `expect`, just re-exported under different names so every spec file goes through `fixtures.ts` and picks up the shared API fixtures. For check-in flows, prefer `checkinHarness` over raw `/rfid`, `/current-visit`, or `/api/iot/checkin` setup code inside the spec body.

```typescript
import { apiTest as test, apiExpect as expect } from "../fixtures";

test("returns config for the seeded device", async ({ deviceApi }) => {
  const res = await deviceApi.get("/api/iot/config");
  expect(res.status()).toBe(200);
});
```

## Manifest contract

The Playwright harness reads all seeded actors, tenant topology, device
credentials, scenario-specific fixtures, and harness runtime from the Go
seeder's dedicated manifest. `auth.setup.ts` verifies that canonical
contract, creates browser `storageState`, and nothing else machine-readable is
introduced on the frontend side. `contract.ts` owns the typed manifest and URL
building. `auth.setup.ts` owns browser login and session materialization.
`api.ts` turns that browser session into backend API auth when fixtures need
HTTP contexts. `fixtures.ts` exposes seeded data, ready-to-use auth contexts,
and the small shared helpers that keep repeated check-in plumbing out of spec
bodies. The Go seeder remains the only machine-readable scenario contract.
Do not hardcode emails, passwords, tenant slugs, room names, activity names,
API keys, RFID tags, or "first seeded student" assumptions in specs.
Day-to-day spec code should stay inside the fixtures from `fixtures.ts`.

## Ports

| Port    | What                                                                            | When                    |
| ------- | ------------------------------------------------------------------------------- | ----------------------- |
| `:3000` | Your dev frontend (`docker compose up frontend`) → dev backend `:8080` → dev DB | normal development      |
| `:3030` | E2E frontend, spawned by Playwright → `server-e2e` `:8081` → `postgres-test`    | only while tests run    |
| `:8080` | Backend port inside the `server` / `server-e2e` containers                      | internal container port |
| `:8081` | `server-e2e` exposed on the host for the isolated E2E backend                   | only while tests run    |

The two stacks are fully independent. You can keep your dev frontend running on
`:3000` and run the E2E suite at the same time — both backends use different
DBs (`postgres` vs `postgres-test`), so test runs cannot leak into your dev
work, and vice versa. The refactor keeps those ports intentionally separate.
Open both URLs side by side in your browser if you need to compare behavior.

## How to add a test

The recipe below is the only way to add a spec — every shortcut here came from a real bug in an earlier iteration of the suite.

### 1. Decide UI vs API and pick the right `test` import

| Shape                                        | Import                                                               | Default `page` is…                          |
| -------------------------------------------- | -------------------------------------------------------------------- | ------------------------------------------- |
| UI spec (drives a browser)                   | `import { test, expect } from "../fixtures"`                         | already authenticated as the project's role |
| HTTP-only spec (no browser, no auth fixture) | `import { apiTest as test, apiExpect as expect } from "../fixtures"` | n/a — use `request` instead                 |

Never `import { test } from "@playwright/test"`. Going through `fixtures.ts` is what gives you the factories below; bypassing it loses them silently.

### 2. Pick the right role

- Admin-only specs → file goes anywhere under `e2e/`. The `chromium-admin` project picks it up automatically.
- Staff-only specs → file MUST end in `.staff.spec.ts` (e.g. `myroom-supervision.staff.spec.ts`). The `chromium-staff` project's `testMatch` is wired to that pattern.

### 3. Use the factories and API fixtures — never open your own auth context

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

The fixture deletes whatever the factory tracked at the end of the test — including on assertion failure. Do NOT add `try/finally` blocks for cleanup or recreate login/token helpers inside specs; that pattern was retired because it duplicated the fixture's job and made the harness harder to reason about. Suffix collisions across parallel workers are also handled (factory uses `randomUUID()`, not timestamps).

### 4. Use `helpers/routes.ts` for navigation, not raw paths

```typescript
import * as routes from "../helpers/routes";

await page.goto(app.primary(routes.studentsList));
await page.goto(app.primary(routes.studentDetail(student.id)));
await page.goto(app.primary(routes.groupsList));
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
const res = await adminApi.get(`/api/students/${id}`, {
  failOnStatusCode: false,
});
expect(res.status()).toBe(404);
```

### 7. Run `pnpm run check` before committing

The repo's `oxlint .` and `tsc --noEmit` already include `e2e/**/*.ts`, so a typo in a helper or a stale type from a backend response shape change will fail the gate before you push. There's no separate `pnpm e2e:check` to remember.

### 8. Stable test data — never assume a seeded id

Seeded entity ids shift whenever `backend/seed/api/data.go` is reordered.
Specs should consume the canonical seeded refs through typed fixture data from
`fixtures.ts`, not by reaching into the manifest shape:

```typescript
import { test } from "../fixtures";

test("uses the seeded RFID fixture", async ({ checkinScenario }) => {
  const studentId = checkinScenario.student.id; // never `id: 1`
  const rfidTag = checkinScenario.rfid_tag; // never hardcode RFID tags
});
```

Same rule for names and scenario picks: prefer `studentSearchScenario`,
`groupVisibilityScenario`, and `checkinScenario` over hardcoding
`"Felix Schneider"` or `"Hausaufgaben"` in specs.

## Why we use `localtest.me` instead of `localhost`

The bare domain shows a tenant selector, not a login form (see `src/app/page.tsx`). The login lives at `{slug}.TENANT_DOMAIN/`, so the E2E contract layer builds full tenant URLs such as `http://demo-school.localtest.me:3030/` via the `app` fixture and `contract.ts`.

`localtest.me` is a public DNS wildcard that resolves any subdomain to `127.0.0.1`, no `/etc/hosts` edits required. We use it instead of `*.localhost` because browsers treat `*.localhost` as separate hosts: NextAuth's session cookie ends up host-scoped, and tenant switching cannot share sessions across subdomains. With `localtest.me`, NextAuth sets the cookie on `.localtest.me` (`server/auth/tenant-config.ts:121-156`) and the switcher works locally exactly like in production.

## Troubleshooting

| Symptom                                                   | Likely cause                                                                                                                                                |
| --------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Setup test times out on `input[name="email"]`             | Wrong tenant host — the harness should open `http://demo-school.localtest.me:3030/` via `app.primary(routes.root)`.                                         |
| `EADDRINUSE :3030` when starting Playwright               | Something else (a previous abandoned `pnpm run dev`?) is on :3030. Kill it: `lsof -nP -iTCP:3030 -sTCP:LISTEN`, then `kill <pid>`.                          |
| `DNS_PROBE_FINISHED_NXDOMAIN` for `*.localtest.me`        | No internet — `localtest.me` resolves over public DNS. Add `127.0.0.1 demo-school.localtest.me second-school.localtest.me` to `/etc/hosts` for offline use. |
| Setup test fails on "Anmelden" with "Ungültige Eingabe"   | Seeder hasn't been run, or the emitted manifest / credentials no longer match the active DB state. Re-run `pnpm run e2e`.                                   |
| `Executable doesn't exist` on first run                   | Run `pnpm exec playwright install chromium`.                                                                                                                |
| Tests pass locally but fail in CI                         | The canonical `pnpm run e2e` flow may have diverged from CI assumptions — check the uploaded seed manifest and backend logs.                                |
| Cookies appear but tests still see the login form         | Stale `storageState` after seeder change — `rm -rf e2e/.auth` and re-run.                                                                                   |
| `tenant-switch.spec.ts` fails on "TenantSwitcher visible" | The Go-owned E2E world did not materialize both tenants cleanly — re-run `pnpm run e2e` or inspect `backend/.e2e-manifest.json`.                            |

## Debug-only escape hatch

If you are working on the Playwright layer itself and need to skip the full
orchestrator, the bare runner still works as a deliberate escape hatch:

```bash
cd frontend
pnpm exec playwright test --list
```

That path is intentionally not the day-to-day entrypoint. It assumes the
isolated backend stack is already up and the canonical manifest already exists.
