import { defineConfig, devices } from "@playwright/test";

import { loadAccess, perfPort, tenantBaseUrl } from "./scripts/perf/access";
import { perfServerEnv } from "./scripts/perf/env.mjs";

// Render-Zählung mit react-scan gegen den DEV-Server (pnpm run perf:render,
// #2938). react-scan liest React-Fiber-Debuginfos, die nur der Dev-Build hat.
// Ein bereits laufender Dev-Server (Docker-Frontend auf :3000) wird
// wiederverwendet.
const env = perfServerEnv();
const access = loadAccess();

export default defineConfig({
  testDir: "./scripts/perf",
  testMatch: "react-scan.perf.ts",
  timeout: 240_000,
  workers: 1,
  retries: 0,
  reporter: "list",
  outputDir: "perf-results/react-scan/test-output",
  use: {
    ...devices["Desktop Chrome"],
    baseURL: tenantBaseUrl(access),
    headless: true,
  },
  webServer: {
    command: "pnpm run dev",
    env: { ...env, PORT: perfPort() },
    url: `http://localhost:${perfPort()}/help`,
    reuseExistingServer: true,
    timeout: 180_000,
  },
});
