import { defineConfig } from "@playwright/test";
import network from "./playwright.network.config";
import { networkPorts } from "./e2e/network/ports.mjs";

const ports = networkPorts("tenantSwitch");

/** @public Real Auth.js domain cookies and UI, synthetic localhost backend. */
export default defineConfig({
  ...network,
  testDir: "./e2e/network",
  testMatch: [
    "tenant-mfa-switch.spec.ts",
    "late-session-cookie.spec.ts",
    "late-portal-cookie.spec.ts",
  ],
  testIgnore: "portal-network.spec.ts",
  outputDir: "test-results/tenant-switch",
  webServer: {
    url: `http://127.0.0.1:${ports.frontend}/school-a`,
    timeout: 120_000,
    reuseExistingServer: false,
    gracefulShutdown: { signal: "SIGTERM", timeout: 10_000 },
    stdout: "ignore",
    // The server writes only its own start errors; Next's output stays off.
    stderr: "pipe",
    command:
      "TENANT_SWITCH_EVIDENCE=1 node e2e/network/server.mjs --stop-with-stdin",
  },
});
