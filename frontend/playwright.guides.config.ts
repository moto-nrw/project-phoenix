import { defineConfig, devices } from "@playwright/test";

// Dedicated config for build-time guide-PDF generation (pnpm run generate:guides).
// Kept separate from playwright.config.ts so the generator never runs as part of
// the normal e2e suite (and vice versa). It renders the public /help pages via
// the same dev server the e2e config uses.
//
// Env note: the /help pages are public and host-agnostic, but proxy.ts throws
// when the operator/parents/tenant hostnames are unset. Locally those come from
// .env.local (auto-loaded by `next dev`); in CI the generate:guides step sets
// them plus SKIP_ENV_VALIDATION. No values are baked in here.
export default defineConfig({
  testDir: "./scripts",
  testMatch: "generate-guides.ts",
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
    command: "pnpm run dev",
    url: "http://localhost:3000/help",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
  },
});
