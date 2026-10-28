import { defineConfig, devices } from "@playwright/test";

// Installed Playwright otherwise writes the DOM (including form values) into
// error-context.md even when screenshots and traces are disabled.
process.env.PLAYWRIGHT_NO_COPY_PROMPT = "1";

/** @public Local, synthetic network evidence. No seeded database or real keys. */
export default defineConfig({
  testDir: "./e2e/network",
  testMatch: "*.spec.ts",
  // The shared-cookie scenario needs its own domain and fixture server mode.
  testIgnore: ["tenant-mfa-switch.spec.ts", "late-session-cookie.spec.ts"],
  workers: 1,
  retries: 0,
  timeout: 90_000,
  expect: { timeout: 20_000 },
  forbidOnly: !!process.env.CI,
  reporter: "list",
  outputDir: "test-results/network",
  use: {
    ...devices["Desktop Chrome"],
    locale: "de-DE",
    serviceWorkers: "block",
    // Never save credentials, form contents, session cookies or raw requests.
    trace: "off",
    screenshot: "off",
    video: "off",
    actionTimeout: 15_000,
    navigationTimeout: 30_000,
    launchOptions: {
      args: [
        "--host-resolver-rules=MAP * ~NOTFOUND, EXCLUDE localhost, EXCLUDE *.localhost",
        "--disable-background-networking",
      ],
    },
  },
  webServer: {
    command: "node e2e/network/server.mjs",
    // Node, unlike Chromium, cannot assume wildcard .localhost resolution.
    url: "http://127.0.0.1:13843/api/health",
    reuseExistingServer: false,
    timeout: 120_000,
    stdout: "ignore",
    stderr: "ignore",
  },
});
