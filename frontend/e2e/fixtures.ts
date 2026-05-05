import { test as base, type Browser, type Page } from "@playwright/test";
import {
  makeGroupFactory,
  makeStudentFactory,
  teardownGroups,
  teardownStudents,
  type GroupFactory,
  type StudentFactory,
} from "./helpers/factories";
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
  /**
   * Test-scoped factory for students. `studentFactory.create({...})`
   * returns a freshly-created student; teardown deletes everything the
   * factory created, even if assertions failed mid-test. Per-test scope
   * (not per-worker) keeps tests isolated.
   *
   * Use instead of inline `withAdminContext + try/finally + Date.now()`.
   */
  studentFactory: StudentFactory;
  /**
   * Test-scoped factory for groups. Same lifecycle as `studentFactory`.
   * Teardown order: studentFactory runs first (Playwright tears down
   * fixtures in reverse instantiation order), so a student moved into a
   * factory-owned group is detached before the group DELETE — avoids the
   * backend's 409-when-not-empty rejection.
   */
  groupFactory: GroupFactory;
};

// SonarCloud's React rules misidentify Playwright's fixture `use(...)`
// callback as React's `use` Hook (typescript:S6440 / S6442). The two are
// unrelated: in Playwright, `use` is the standard fixture-yield function
// that signals "fixture setup done, run the test, then tear down". The
// per-line NOSONAR comments below opt those specific call sites out.
async function pageForRole(
  browser: Browser,
  storageState: string,
  use: (page: Page) => Promise<void>,
): Promise<void> {
  const context = await browser.newContext({ storageState });
  const page = await context.newPage();
  try {
    await use(page); // NOSONAR — Playwright fixture callback, not React Hook
  } finally {
    await context.close();
  }
}

const baseWithFactories = base.extend<Fixtures>({
  authenticatedPage: async ({ page }, use) => {
    await use(page); // NOSONAR — Playwright fixture callback, not React Hook
  },
  adminPage: async ({ browser }, use) => {
    await pageForRole(browser, STORAGE_STATE_PATH.admin, use);
  },
  staffPage: async ({ browser }, use) => {
    await pageForRole(browser, STORAGE_STATE_PATH.staff, use);
  },
  // Playwright requires the first arg to be an object destructuring
  // pattern; a non-destructured `_` throws at runtime. These factories
  // have no fixture deps, so the destructure is empty — disable the
  // empty-pattern lint inline.
  // oxlint-disable-next-line no-empty-pattern
  studentFactory: async ({}, use) => {
    const factory = makeStudentFactory();
    await use(factory); // NOSONAR — Playwright fixture callback, not React Hook
    await teardownStudents(factory._created());
  },
  // oxlint-disable-next-line no-empty-pattern
  groupFactory: async ({}, use) => {
    const factory = makeGroupFactory();
    await use(factory); // NOSONAR — Playwright fixture callback, not React Hook
    await teardownGroups(factory._created());
  },
});

export const test = baseWithFactories;
export { expect } from "@playwright/test";

/**
 * HTTP-only specs that do not need an authenticated `Page` use these
 * re-exports. They share the factory fixtures with `test` — the same
 * `studentFactory` / `groupFactory` are available for setup/teardown
 * even when the spec drives the backend directly via `request`. Going
 * through this module (instead of importing from `@playwright/test`)
 * keeps the README rule "no `@playwright/test` imports in specs" intact.
 */
export const apiTest = baseWithFactories;
export { expect as apiExpect } from "@playwright/test";
