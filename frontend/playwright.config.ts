import { defineConfig, devices } from "@playwright/test";
import { STORAGE_STATE_PATH } from "./e2e/auth";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  // No retries — anywhere. Retries on CI hide flakes behind green checkmarks
  // and erode trust in the suite over time. If a test is flaky, fix the
  // root cause (timing assumption, shared state, race) instead of papering
  // over it. The existing test data isolation (`randomUUID()` suffixes,
  // dedicated postgres-test DB, idempotent setup) is designed to support
  // this stance.
  retries: 0,
  // Cap parallelism so multiple workers do not saturate the single backend
  // container and the one spawned Next.js dev server. With the full suite,
  // 4 local workers regularly pushed protected-page navigations and modal
  // submits into timeouts under cold-compile load; 2 is slower but stable.
  // CI still runs at 1 worker for full determinism.
  workers: process.env.CI ? 1 : 2,
  // Per-test timeout. The default 30s is tight for UI specs that drive a
  // cold dev server: a first-hit page.goto under 4-way parallel load can
  // burn 15–25s on Next.js lazy compilation alone before the spec even
  // starts asserting, leaving too little room for the rest of the test.
  // 60s gives headroom for cold compile + form render + waitForResponse
  // chains without masking a genuine hang. CI runs with workers=1 and
  // warm caches, so it'll never come close to this ceiling.
  timeout: 60_000,
  reporter: "html",
  webServer: {
    command: "pnpm run e2e:harness",
    port: 3030,
    timeout: 180_000,
    reuseExistingServer: false,
    gracefulShutdown: {
      signal: "SIGTERM",
      timeout: 15_000,
    },
    stdout: "pipe",
    stderr: "pipe",
  },
  use: {
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
});
