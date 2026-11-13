import { chromium } from "@playwright/test";
import { existsSync, readFileSync, writeFileSync } from "node:fs";
import path from "node:path";

import { secureLaunchOptions } from "./secure-chrome";
import { browserMismatch } from "./kit-pixel-lib.mjs";

/**
 * Chrome aktualisiert sich selbst. Ein anderer Build rendert Kanten und Schrift
 * minimal anders, und der Vergleich zeigte dann Abweichungen, die nicht aus dem
 * Code kommen. Die Baseline merkt sich die Version, der Vergleich bricht bei
 * einer anderen ab, bevor ein Bild entsteht.
 *
 * @public Loaded by the Playwright CLI through `globalSetup`.
 */
export default async function checkBrowserVersion(): Promise<void> {
  const browser = await chromium.launch(secureLaunchOptions());
  const version = browser.version();
  await browser.close();

  const versionFile = path.join(
    process.env.KIT_PIXEL_BASELINE_DIR ?? "",
    "chrome-version.txt",
  );
  if (process.env.KIT_PIXEL_MODE === "baseline") {
    writeFileSync(versionFile, `${version}\n`);
    return;
  }
  if (!existsSync(versionFile)) {
    throw new Error(
      "Die Baseline nennt keine Chrome-Version. Neu erzeugen: pnpm run kit-pixel baseline --ref <commit>",
    );
  }
  const mismatch = browserMismatch(
    readFileSync(versionFile, "utf8").trim(),
    version,
  );
  if (mismatch) throw new Error(mismatch);
}
