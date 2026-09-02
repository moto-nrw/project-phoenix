import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: ".",
  timeout: 30 * 60_000,
  workers: 1,
  retries: 0,
  reporter: "list",
  use: {
    headless: true,
    locale: "de-DE",
    timezoneId: "Europe/Berlin",
  },
});
