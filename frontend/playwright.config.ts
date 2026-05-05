import { defineConfig, devices } from "@playwright/test";
import { BASE_URL, STORAGE_STATE_PATH } from "./e2e/helpers/seed-data";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  // Cap parallelism so multiple workers don't saturate the single backend
  // container; 4 keeps suite runtime low without making auth/UI tests flaky
  // under load. CI uses 1 worker for full determinism.
  workers: process.env.CI ? 1 : 4,
  reporter: "html",
  use: {
    baseURL: BASE_URL,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
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
      testIgnore: /auth\.setup\.ts/,
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
    command: "pnpm run dev",
    url: "http://localhost:3000",
    reuseExistingServer: !process.env.CI,
    // Force the dev server to talk to the isolated E2E backend (server-e2e
    // on :8081) instead of the dev `server` (:8080). This is the second
    // half of the E2E isolation: backend on its own DB AND frontend wired
    // to that backend, so a developer running `docker compose up` in
    // parallel doesn't poison test runs.
    env: {
      NEXT_PUBLIC_API_URL: "http://localhost:8081",
      API_URL: "http://localhost:8081",
      // The dev frontend's .env.local may still have TENANT_DOMAIN=localhost
      // from before the localtest.me migration. Override here so the suite
      // is self-contained: any *.localtest.me request resolves the tenant.
      TENANT_DOMAIN: "localtest.me",
      NEXT_PUBLIC_TENANT_DOMAIN: "localtest.me",
    },
  },
});
