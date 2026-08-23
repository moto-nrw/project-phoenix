import { defineConfig, devices } from "@playwright/test";

// Dedicated config for build-time guide-PDF generation (pnpm run generate:guides).
// Kept separate from playwright.config.ts so the generator never runs as part of
// the normal e2e suite (and vice versa).
//
// Renders against a *production* server (`next build && next start`), not
// `next dev`: the PDFs ship inside the prod image, so they must be generated
// from the same output users actually see , and a prod server renders
// deterministically and fast (no first-request compile / HMR), which removes
// the main flakiness source.
//
// Env note: the /help pages are public and host-agnostic, but proxy.ts throws
// when the operator/parents/tenant hostnames are unset. Locally those come from
// .env.local; in CI the generate:guides step sets them plus SKIP_ENV_VALIDATION.
// No values are baked in here.
export default defineConfig({
  testDir: "./scripts",
  testMatch: "generate-guides.ts",
  // The 91-page features guide can exceed Playwright's 30-second default on CI.
  timeout: 120_000,
  workers: 1,
  reporter: "list",
  use: {
    baseURL: "http://localhost:3000",
    headless: true,
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    command: "pnpm run build && pnpm run start",
    url: "http://localhost:3000/help",
    // Locally reuse a server you already have running; in CI always build fresh.
    reuseExistingServer: !process.env.CI,
    // build + start is far slower to become ready than `next dev`.
    timeout: 300_000,
  },
});
