import { chromium } from "@playwright/test";

import { assertPatchedChrome, secureLaunchOptions } from "./secure-chrome";

/**
 * Global setup of every Playwright config: fails before the first test when
 * the installed Chrome is missing or older than the patched version (#5612).
 *
 * @public Loaded by the Playwright CLI through `globalSetup`.
 */
export default async function verifySecureChrome(): Promise<void> {
  const browser = await chromium.launch(secureLaunchOptions());
  try {
    assertPatchedChrome(browser.version());
  } finally {
    await browser.close();
  }
}
