import {
  test as base,
  type APIRequestContext,
  type Browser,
  type Page,
} from "@playwright/test";
import {
  type AuthSetupContract,
  getAuthSetupContract,
  STORAGE_STATE_PATH,
} from "./auth";
import {
  createCheckinFlow,
  createBackendApiContext,
  createDeviceApiContext,
  createIotContract,
  createPresentReadyStudentCard,
  createTenantApiContext,
  type CheckinFlow,
  type IotContract,
  type PresentReadyStudentCard,
} from "./api";
import {
  makeGroupFactory,
  makeStudentFactory,
  teardownGroups,
  teardownStudents,
  type GroupFactory,
  type StudentFactory,
} from "./helpers/factories";
import {
  getAppUrls,
  getCheckinDevice,
  getCheckinScenario,
  getGroupVisibilityProbe,
  getPresentReadyStudent,
  getPrimaryTenant,
  getStudentSearchProbe,
  requireSecondaryTenant,
  type AppUrls,
  type GroupVisibilityProbe,
  type StudentSearchProbe,
  type Tenant,
} from "./state";

type Fixtures = {
  app: AppUrls;
  authSessions: AuthSetupContract;
  primaryTenant: Tenant;
  secondaryTenant: Tenant;
  presentReadyStudentCard: PresentReadyStudentCard;
  studentSearchProbe: StudentSearchProbe;
  groupVisibilityProbe: GroupVisibilityProbe;
  checkinFlow: CheckinFlow;
  iotContract: IotContract;
  backendApi: APIRequestContext;
  adminApi: APIRequestContext;
  staffApi: APIRequestContext;
  deviceApi: APIRequestContext;
  /**
   * The page provided by the active project — already authenticated as
   * whichever role the project is configured for. Use this in single-role
   * tests; it's just the standard `page` with a clearer name.
   */
  authenticatedPage: Page;
  /**
   * A page authenticated as the setup-prepared admin actor, regardless of
   * which project the test runs in. Spawns a fresh browser context backed
   * by the setup-written storageState so it does not leak into other fixtures.
   */
  adminPage: Page;
  /**
   * A page authenticated as the setup-prepared staff/user actor, in its
   * own browser context.
   */
  staffPage: Page;
  /**
   * Test-scoped factory for students. `studentFactory.create({...})`
   * returns a freshly-created student; teardown deletes everything the
   * factory created, even if assertions failed mid-test. Per-test scope
   * (not per-worker) keeps tests isolated.
   */
  studentFactory: StudentFactory;
  /**
   * Test-scoped factory for groups. Same lifecycle as `studentFactory`.
   */
  groupFactory: GroupFactory;
};

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

const baseWithFixtures = base.extend<Fixtures>({
  // oxlint-disable-next-line no-empty-pattern
  app: async ({}, use) => {
    await use(getAppUrls()); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  authSessions: async ({}, use) => {
    await use(getAuthSetupContract()); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  primaryTenant: async ({}, use) => {
    await use(getPrimaryTenant()); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  secondaryTenant: async ({}, use) => {
    await use(requireSecondaryTenant()); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  // oxlint-disable-next-line no-empty-pattern
  backendApi: async ({}, use) => {
    const ctx = await createBackendApiContext();
    try {
      await use(ctx); // NOSONAR — Playwright fixture callback, not React Hook
    } finally {
      await ctx.dispose();
    }
  },
  // oxlint-disable-next-line no-empty-pattern
  adminApi: async ({ authSessions }, use) => {
    const ctx = await createTenantApiContext(authSessions.admin);
    try {
      await use(ctx); // NOSONAR — Playwright fixture callback, not React Hook
    } finally {
      await ctx.dispose();
    }
  },
  // oxlint-disable-next-line no-empty-pattern
  staffApi: async ({ authSessions }, use) => {
    const ctx = await createTenantApiContext(authSessions.staff);
    try {
      await use(ctx); // NOSONAR — Playwright fixture callback, not React Hook
    } finally {
      await ctx.dispose();
    }
  },
  // oxlint-disable-next-line no-empty-pattern
  // oxlint-disable-next-line no-empty-pattern
  deviceApi: async ({}, use) => {
    const ctx = await createDeviceApiContext(getCheckinDevice());
    try {
      await use(ctx); // NOSONAR — Playwright fixture callback, not React Hook
    } finally {
      await ctx.dispose();
    }
  },
  authenticatedPage: async ({ page }, use) => {
    await use(page); // NOSONAR — Playwright fixture callback, not React Hook
  },
  adminPage: async ({ browser }, use) => {
    await pageForRole(browser, STORAGE_STATE_PATH.admin, use);
  },
  staffPage: async ({ browser }, use) => {
    await pageForRole(browser, STORAGE_STATE_PATH.staff, use);
  },
  // oxlint-disable-next-line no-empty-pattern
  studentSearchProbe: async ({}, use) => {
    await use(getStudentSearchProbe()); // NOSONAR
  },
  // oxlint-disable-next-line no-empty-pattern
  groupVisibilityProbe: async ({}, use) => {
    await use(getGroupVisibilityProbe()); // NOSONAR
  },
  presentReadyStudentCard: async ({ adminApi }, use) => {
    const card = createPresentReadyStudentCard(
      getPresentReadyStudent(),
      adminApi,
    );
    await use(card); // NOSONAR — Playwright fixture callback, not React Hook
  },
  checkinFlow: async ({ adminApi, deviceApi }, use) => {
    const flow = createCheckinFlow(getCheckinScenario(), deviceApi, adminApi);
    await use(flow); // NOSONAR — Playwright fixture callback, not React Hook
  },
  iotContract: async ({ backendApi, deviceApi }, use) => {
    const contract = createIotContract(
      backendApi,
      deviceApi,
      getCheckinDevice(),
    );
    await use(contract); // NOSONAR — Playwright fixture callback, not React Hook
  },
  studentFactory: async ({ adminApi }, use) => {
    const factory = makeStudentFactory(adminApi);
    await use(factory); // NOSONAR — Playwright fixture callback, not React Hook
    await teardownStudents(adminApi, factory._created());
  },
  groupFactory: async ({ adminApi }, use) => {
    const factory = makeGroupFactory(adminApi);
    await use(factory); // NOSONAR — Playwright fixture callback, not React Hook
    await teardownGroups(adminApi, factory._created());
  },
});

export const test = baseWithFixtures;
export { expect } from "@playwright/test";
export { assertSessionReady, waitForLoginFormReady } from "./auth";

export const apiTest = baseWithFixtures;
export { expect as apiExpect } from "@playwright/test";
