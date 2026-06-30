import { defineConfig, devices } from "@playwright/test";

const webServerEnv = {
  API_URL: process.env.API_URL ?? "http://localhost:8080",
  NEXT_PUBLIC_API_URL:
    process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080",
  NEXT_PUBLIC_OPERATOR_HOSTNAME:
    process.env.NEXT_PUBLIC_OPERATOR_HOSTNAME ?? "operator.localhost:3000",
  NEXT_PUBLIC_PARENTS_HOSTNAME:
    process.env.NEXT_PUBLIC_PARENTS_HOSTNAME ?? "parents.localhost:3000",
  TENANT_DOMAIN: process.env.TENANT_DOMAIN ?? "localhost:3000",
};

const webServerEnvPrefix = Object.entries(webServerEnv)
  .map(([key, value]) => `${key}=${value}`)
  .join(" ");

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 1 : undefined,
  reporter: "html",
  use: {
    baseURL: "http://localhost:3000",
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    command: `${webServerEnvPrefix} pnpm run dev`,
    port: 3000,
    timeout: 120_000,
    reuseExistingServer: !process.env.CI,
  },
});
