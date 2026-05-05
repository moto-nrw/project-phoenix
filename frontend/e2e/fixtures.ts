import { test as base, type Browser, type Page } from "@playwright/test";
import { STORAGE_STATE_PATH } from "./helpers/seed-data";

type Fixtures = {
  /**
   * The page provided by the active project — already authenticated as
   * whichever role the project is configured for. Use this in single-role
   * tests; it's just the standard `page` with a clearer name.
   */
  authenticatedPage: Page;
  /**
   * A page authenticated as the admin role (demo1@mail.de), regardless of
   * which project the test runs in. Spawns a fresh browser context backed
   * by the admin storageState so it does not leak into other fixtures.
   */
  adminPage: Page;
  /**
   * A page authenticated as the staff/user role (demo11@mail.de), in its
   * own browser context.
   */
  staffPage: Page;
};

async function pageForRole(
  browser: Browser,
  storageState: string,
  use: (page: Page) => Promise<void>,
): Promise<void> {
  const context = await browser.newContext({ storageState });
  const page = await context.newPage();
  try {
    await use(page);
  } finally {
    await context.close();
  }
}

export const test = base.extend<Fixtures>({
  authenticatedPage: async ({ page }, use) => {
    await use(page);
  },
  adminPage: async ({ browser }, use) => {
    await pageForRole(browser, STORAGE_STATE_PATH.admin, use);
  },
  staffPage: async ({ browser }, use) => {
    await pageForRole(browser, STORAGE_STATE_PATH.staff, use);
  },
});

export { expect } from "@playwright/test";
