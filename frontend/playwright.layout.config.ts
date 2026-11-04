import { defineConfig, devices } from "@playwright/test";
import { SECURE_CHROME_SETUP, secureChromeUse } from "./scripts/secure-chrome";

// Layouttests im echten Browser (pnpm run test:layout). Sie kompilieren
// `src/styles/globals.css` selbst und rendern nachgebautes Markup mit
// `page.setContent`: kein Next-Server, kein Login, keine Umgebungsvariablen.
// jsdom und happy-dom rechnen kein Layout; Fehler wie #3328 (eine Karte, die
// nicht mehr schrumpft) sieht nur ein Browser.

/** @public Loaded by the Playwright CLI rather than imported by application code. */
export default defineConfig({
  globalSetup: SECURE_CHROME_SETUP,
  use: secureChromeUse(),
  testDir: "./e2e/layout",
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  reporter: process.env.CI ? "list" : "html",
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
