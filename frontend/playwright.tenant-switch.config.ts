import { defineConfig } from "@playwright/test";
import network from "./playwright.network.config";

/** @public Real Auth.js domain cookies and UI, synthetic localhost backend. */
export default defineConfig({
  ...network,
  testDir: "./e2e/network",
  testMatch: "tenant-mfa-switch.spec.ts",
  testIgnore: "portal-network.spec.ts",
  outputDir: "test-results/tenant-switch",
  webServer: {
    // TENANT_SWITCH_PORT moves the run aside, see e2e/network/server.mjs.
    url: `http://127.0.0.1:${process.env.TENANT_SWITCH_PORT ?? "13845"}/api/health`,
    timeout: 120_000,
    reuseExistingServer: false,
    stdout: "ignore",
    stderr: "ignore",
    command: "TENANT_SWITCH_EVIDENCE=1 node e2e/network/server.mjs",
  },
});
