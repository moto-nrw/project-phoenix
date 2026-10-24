import { defineConfig } from "@playwright/test";
import network from "./playwright.network.config";

/** @public Real Auth.js domain cookies and UI, synthetic localhost backend. */
export default defineConfig({
  ...network,
  testMatch: "tenant-mfa-switch.spec.ts",
  testIgnore: [],
  outputDir: "test-results/tenant-switch",
  webServer: {
    url: "http://127.0.0.1:13845/api/health",
    timeout: 120_000,
    reuseExistingServer: false,
    stdout: "ignore",
    stderr: "ignore",
    command: "TENANT_SWITCH_EVIDENCE=1 node e2e/network/server.mjs",
  },
});
