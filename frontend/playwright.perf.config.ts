import { defineConfig, devices } from "@playwright/test";

import { loadAccess, perfPort, tenantBaseUrl } from "./scripts/perf/access";
import { perfServerEnv } from "./scripts/perf/env.mjs";

// Perf-Baseline gegen einen PRODUKTIONS-Server (pnpm run perf:trace, #2938).
// Wie playwright.guides.config.ts: `next build && next start`, weil nur der
// Prod-Server das misst, was Schulen bekommen (kein On-Demand-Compile, kein HMR).
//
// reuseExistingServer ist bewusst AUS: läuft auf :3000 schon das Docker-
// `next dev`, würde der Lauf sonst still gegen den Dev-Server messen. Vorher
// `docker compose stop frontend`.
//
// Env: proxy.ts und die Runtime-Validierung von `next start` verlangen die
// vollständige Server-Env; scripts/perf/env.ts liest .env.local, .env und
// ../.env und reicht sie explizit durch.
const env = perfServerEnv();
const access = loadAccess();

export default defineConfig({
  testDir: "./scripts/perf",
  // `.perf.ts`, nicht `.spec.ts`: sonst greift vitest die Datei auf.
  testMatch: "measure.perf.ts",
  timeout: 400_000,
  workers: 1,
  retries: 0,
  reporter: "list",
  outputDir: "perf-results/test-output",
  use: {
    ...devices["Desktop Chrome"],
    baseURL: tenantBaseUrl(access),
    headless: true,
  },
  webServer: {
    command: "pnpm run build && pnpm run start",
    // PORT explizit: eine Shell mit PORT=8080 (Backend-Konvention) würde den
    // Next-Server sonst auf den Backend-Port setzen.
    env: { ...env, PORT: perfPort() },
    url: `http://localhost:${perfPort()}/help`,
    reuseExistingServer: false,
    timeout: 600_000,
  },
});
