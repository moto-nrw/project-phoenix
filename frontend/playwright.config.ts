import { defineConfig, devices } from "@playwright/test";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

function loadLocalEnv(): void {
  for (const file of [
    ".env.development.local",
    ".env.local",
    ".env.development",
    ".env",
  ]) {
    const path = join(process.cwd(), file);
    if (!existsSync(path)) continue;
    for (const line of readFileSync(path, "utf8").split(/\r?\n/)) {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith("#")) continue;
      const separator = trimmed.indexOf("=");
      if (separator <= 0) continue;
      const key = trimmed.slice(0, separator).trim();
      let value = trimmed.slice(separator + 1).trim();
      if (
        (value.startsWith('"') && value.endsWith('"')) ||
        (value.startsWith("'") && value.endsWith("'"))
      ) {
        value = value.slice(1, -1);
      }
      process.env[key] ??= value;
    }
  }
}

loadLocalEnv();

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
  NEXT_PUBLIC_SCHOOL_HOSTNAME: requiredEnv("NEXT_PUBLIC_SCHOOL_HOSTNAME"),
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
