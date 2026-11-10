import { defineConfig, devices } from "@playwright/test";

import { SECURE_CHROME_SETUP, secureChromeUse } from "./scripts/secure-chrome";

// Pixel-Vergleich der Kit-Stories (#6562). Nur über `pnpm run kit-pixel`
// starten: der Befehl baut Storybook und setzt die Pfade unten. Ablauf und
// Optionen: docs/agents/frontend-testing.md#kit-pixel-comparison.

function requiredEnv(name: string): string {
  const value = process.env[name];
  if (!value) {
    throw new Error(
      `${name} fehlt. Den Pixel-Vergleich mit "pnpm run kit-pixel baseline|compare" starten.`,
    );
  }
  return value;
}

const baselineDir = requiredEnv("KIT_PIXEL_BASELINE_DIR");
const workDir = requiredEnv("KIT_PIXEL_WORK_DIR");

/** Gemeinsame Einstellungen beider Breiten: Berliner Zeit, deutsch, ohne Bewegung. */
const stableContext = {
  // Rendern auf der CPU, ohne Teil-Raster und ohne Compositor-Threads: Sonst
  // zeichnet Chrome weiche Schatten und Backdrop-Blur unter Last von Bild zu
  // Bild minimal anders (gemessen bei SlideOver und ListRowLink auf dem Telefon).
  ...secureChromeUse({
    args: [
      "--disable-gpu",
      "--force-color-profile=srgb",
      "--disable-partial-raster",
      "--disable-skia-runtime-opts",
      "--run-all-compositor-stages-before-draw",
      "--disable-threaded-animation",
      "--disable-threaded-scrolling",
      "--disable-checker-imaging",
      "--font-render-hinting=none",
    ],
  }),
  locale: "de-DE",
  timezoneId: "Europe/Berlin",
  reducedMotion: "reduce",
  colorScheme: "light",
  serviceWorkers: "block",
} as const;

/** @public Loaded by the Playwright CLI rather than imported by application code. */
export default defineConfig({
  globalSetup: [
    SECURE_CHROME_SETUP,
    "./e2e/kit-pixel/browser-version.setup.ts",
  ],
  testDir: "./e2e/kit-pixel",
  outputDir: `${workDir}/results`,
  snapshotPathTemplate: `${baselineDir}/{projectName}/{arg}{ext}`,
  fullyParallel: true,
  forbidOnly: true,
  retries: 0,
  reporter: [
    ["list"],
    ["html", { outputFolder: `${workDir}/report`, open: "never" }],
  ],
  expect: {
    toHaveScreenshot: {
      // „Ohne Abweichung“ heißt: kein einziges Pixel anders.
      threshold: 0,
      maxDiffPixels: 0,
      animations: "disabled",
      caret: "hide",
      scale: "device",
    },
  },
  projects: [
    {
      name: "desktop",
      use: {
        ...devices["Desktop Chrome"],
        ...stableContext,
        viewport: { width: 1280, height: 800 },
      },
    },
    {
      name: "phone",
      use: {
        ...devices["Pixel 7"],
        ...stableContext,
      },
    },
  ],
});
