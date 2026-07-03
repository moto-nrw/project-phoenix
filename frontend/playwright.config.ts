import { defineConfig, devices } from "@playwright/test";

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(`${name} is required for Playwright E2E.`);
  }
  return value;
}

const webServerEnv = {
  API_URL: requiredEnv("API_URL"),
  NEXT_PUBLIC_API_URL: requiredEnv("NEXT_PUBLIC_API_URL"),
  NEXT_PUBLIC_OPERATOR_HOSTNAME: requiredEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME"),
  NEXT_PUBLIC_PARENTS_HOSTNAME: requiredEnv("NEXT_PUBLIC_PARENTS_HOSTNAME"),
  TENANT_DOMAIN: requiredEnv("TENANT_DOMAIN"),
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
