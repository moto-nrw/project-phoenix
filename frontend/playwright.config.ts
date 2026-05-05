import { defineConfig, devices } from "@playwright/test";
import { BASE_URL, STORAGE_STATE_PATH } from "./e2e/helpers/seed-data";

// Single source of truth for "where is the E2E backend?". Read from the
// same env var that helpers/iot.ts uses (E2E_BACKEND_URL) so HTTP-only
// specs and the spawned Next.js dev server agree on the target. When
// unset (the common local case), both fall through to localhost:8081.
const E2E_BACKEND_URL = process.env.E2E_BACKEND_URL ?? "http://localhost:8081";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  // No retries — anywhere. Retries on CI hide flakes behind green checkmarks
  // and erode trust in the suite over time. If a test is flaky, fix the
  // root cause (timing assumption, shared state, race) instead of papering
  // over it. The existing test data isolation (`Date.now()` suffixes,
  // dedicated postgres-test DB, idempotent setup) is designed to support
  // this stance.
  retries: 0,
  // Cap parallelism so multiple workers don't saturate the single backend
  // container; 4 keeps suite runtime low without making auth/UI tests flaky
  // under load. CI uses 1 worker for full determinism.
  workers: process.env.CI ? 1 : 4,
  // Per-test timeout. The default 30s is tight for UI specs that drive a
  // cold dev server: a first-hit page.goto under 4-way parallel load can
  // burn 15–25s on Next.js lazy compilation alone before the spec even
  // starts asserting, leaving too little room for the rest of the test.
  // 60s gives headroom for cold compile + form render + waitForResponse
  // chains without masking a genuine hang. CI runs with workers=1 and
  // warm caches, so it'll never come close to this ceiling.
  timeout: 60_000,
  reporter: "html",
  use: {
    baseURL: BASE_URL,
    // `retries: 0` is intentional (see comment above), so `on-first-retry`
    // would write traces never. `retain-on-failure` keeps a full trace
    // (step timeline, network log, DOM snapshots per step) on every failed
    // run and discards it on success — ~zero cost on green, gold for
    // debugging a CI red without local repro. Same logic for video.
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
    video: "retain-on-failure",
  },
  projects: [
    {
      name: "setup",
      testMatch: /auth\.setup\.ts/,
    },
    {
      name: "chromium-admin",
      use: {
        ...devices["Desktop Chrome"],
        storageState: STORAGE_STATE_PATH.admin,
      },
      // Ignore the setup project AND any *.staff.spec.ts — those run only
      // under chromium-staff with the staff storageState. Without this
      // filter, admin would also execute staff specs as the admin user
      // and the staff-session assertions would fail.
      testIgnore: [/auth\.setup\.ts/, /\.staff\.spec\.ts/],
      dependencies: ["setup"],
    },
    {
      name: "chromium-staff",
      use: {
        ...devices["Desktop Chrome"],
        storageState: STORAGE_STATE_PATH.staff,
      },
      testMatch: /\.staff\.spec\.ts/,
      dependencies: ["setup"],
    },
  ],
  webServer: {
    // E2E runs on :3030 so it can coexist with the developer's dev frontend
    // on :3000. The two stacks are fully independent: dev points at the dev
    // backend (:8080) and dev DB; E2E points at server-e2e (:8081) and
    // postgres-test. Devs can keep their dev frontend running and even open
    // both URLs side by side in the browser to compare behavior.
    // :3030 (not :3001) because :3001 is commonly taken by other dev tools
    // (Bun, secondary Next instances). :3030 is far less crowded.
    command: "pnpm run dev --port 3030",
    url: "http://localhost:3030",
    // Always spawn a fresh dev server — never reuse whatever happens to be
    // listening on :3030. Playwright's URL probe only checks "does anything
    // answer", not "is it our spawned dev server with the right env". A
    // stale Next.js process from an aborted earlier run (still pointing at
    // the dev backend on :8080 instead of server-e2e on :8081) would be
    // silently reused and tests would fail with errors that look like real
    // product bugs but are actually "wrong server". Forcing a spawn costs
    // ~6s per local run but eliminates that whole class of false failures.
    reuseExistingServer: false,
    // Default Playwright webServer timeout is 60s; a cold Next.js build on
    // a loaded machine can take ~90s. 120s gives headroom without masking
    // a genuine hang.
    timeout: 120_000,
    // Force the spawned dev server to talk to the isolated E2E backend
    // (server-e2e on :8081) instead of the dev `server` (:8080). This is
    // the second half of the E2E isolation: backend on its own DB AND
    // frontend wired to that backend, so a developer running
    // `docker compose up` in parallel doesn't poison test runs.
    env: {
      // Bind Next.js to :3030 (also passed via --port; both honored).
      PORT: "3030",
      NEXT_PUBLIC_API_URL: E2E_BACKEND_URL,
      API_URL: E2E_BACKEND_URL,
      // Override TENANT_DOMAIN for the spawned dev server so the suite gets
      // production-shaped cookie behavior (.localtest.me cookie scope) needed
      // for the tenant-switch flow. The canonical dev default is `localhost`,
      // which uses host-scoped cookies and forces re-login on tenant switch —
      // fine for everyday dev, wrong for E2E.
      TENANT_DOMAIN: "localtest.me",
      NEXT_PUBLIC_TENANT_DOMAIN: "localtest.me",
      // Pin the operator subdomain to the localtest.me + :3030 origin so
      // proxy.ts:isOperatorHost() and lib/operator-url.ts:isOperatorSubdomain()
      // recognize the operator URL in this run mode. Without this override,
      // those guards still compare against the dev default (operator.localhost
      // :3000) and a request to operator.localtest.me:3030 would be routed to
      // the tenant selector instead of the operator app — silently wrong, hard
      // to debug if a future spec exercises the operator surface.
      NEXT_PUBLIC_OPERATOR_HOSTNAME: "operator.localtest.me:3030",
    },
  },
});
