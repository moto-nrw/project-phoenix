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

const requiredWebServerEnv = [
  "API_URL",
  "NEXT_PUBLIC_API_URL",
  "NEXT_PUBLIC_OPERATOR_HOSTNAME",
  "NEXT_PUBLIC_PARENTS_HOSTNAME",
  "NEXT_PUBLIC_SCHOOL_HOSTNAME",
  "TENANT_DOMAIN",
] as const;

const webServerEnvCheckScript = `const required = ${JSON.stringify(requiredWebServerEnv)}; const missing = required.filter((key) => !process.env[key]); if (missing.length > 0) { throw new Error(missing.join(", ") + " required for Playwright E2E."); }`;
const webServerEnvCheck = [
  "node",
  "-e",
  JSON.stringify(webServerEnvCheckScript),
].join(" ");

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  workers: process.env.CI ? 1 : undefined,
  reporter: "html",
  use: {
    baseURL: "http://localhost:3000",
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
  webServer: {
    command: `${webServerEnvCheck} && pnpm run dev`,
    port: 3000,
    timeout: 120_000,
    reuseExistingServer: !process.env.CI,
  },
});
