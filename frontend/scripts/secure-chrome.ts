import type { LaunchOptions } from "@playwright/test";

// #5612: Playwright 1.64 bündelt Chromium 156.0.8078.4. Das ist von
// CVE-2026-118306, -118331, -118344 und -118348 betroffen (behoben in Chrome
// 156.0.8078.73). Außerdem startet Playwright Chromium ohne Sandbox, solange
// `chromiumSandbox` nicht `true` ist. Deshalb startet jeder Lauf das
// installierte Google Chrome mit Sandbox, und `secure-chrome.setup.ts` bricht
// bei einer älteren Version ab. Übergangslösung, bis ein Playwright-Release
// einen gepatchten Browser bündelt; die Sandbox bleibt auch danach an.

/** Erste Chrome-Version, in der die CVEs aus #5612 behoben sind. */
export const MIN_CHROME_VERSION = "156.0.8078.73";

/** Startoptionen für jeden Browser, den Playwright hier öffnet. */
export function secureLaunchOptions(
  options: LaunchOptions = {},
): LaunchOptions {
  return { ...options, channel: "chrome", chromiumSandbox: true };
}

/** `use`-Block für Playwright-Configs; ergänzt vorhandene `launchOptions`. */
export function secureChromeUse(launchOptions: LaunchOptions = {}) {
  return {
    channel: "chrome",
    launchOptions: secureLaunchOptions(launchOptions),
  } as const;
}

function versionParts(version: string): number[] {
  if (!/^\d+(\.\d+){3}$/.test(version)) {
    throw new Error(`Unbekanntes Chrome-Versionsformat: "${version}"`);
  }
  return version.split(".").map(Number);
}

export function isPatchedChrome(version: string): boolean {
  const actual = versionParts(version);
  const minimum = versionParts(MIN_CHROME_VERSION);
  for (let i = 0; i < minimum.length; i++) {
    const diff = (actual[i] ?? 0) - (minimum[i] ?? 0);
    if (diff !== 0) return diff > 0;
  }
  return true;
}

export function assertPatchedChrome(version: string): void {
  if (!isPatchedChrome(version)) {
    throw new Error(
      `Google Chrome ${version} ist älter als ${MIN_CHROME_VERSION} und hat bekannte Sicherheitslücken (#5612). ` +
        "Chrome aktualisieren (lokal: Chrome > Über Google Chrome; CI: pnpm exec playwright install --with-deps --force chrome).",
    );
  }
}

/** `globalSetup` path for every Playwright config in this directory. */
export const SECURE_CHROME_SETUP = "./scripts/secure-chrome.setup.ts";
