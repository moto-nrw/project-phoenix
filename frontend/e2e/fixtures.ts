import {
  test as base,
  type APIRequestContext,
  type Browser,
  type Page,
} from "@playwright/test";
import {
  type E2ECheckinFixture,
  type E2EGroupPair,
  type E2EStudentPair,
  type E2EStudentRef,
  type E2ETwitterTenant,
  getAppUrls,
  getE2EManifest,
  requireSecondaryTenant,
} from "./contract";
import {
  type AuthSetupContract,
  getAuthSetupContract,
  STORAGE_STATE_PATH,
} from "./auth";
import {
  createBackendApiContext,
  createDeviceApiContext,
  createTenantApiContext,
} from "./api";
import {
  makeGroupFactory,
  makeStudentFactory,
  teardownGroups,
  teardownStudents,
  type GroupFactory,
  type StudentFactory,
} from "./helpers/factories";

type Fixtures = {
  app: ReturnType<typeof getAppUrls>;
  authSessions: AuthSetupContract;
  primaryTenant: E2ETwitterTenant;
  secondaryTenant: E2ETwitterTenant;
  tenantSwitchScenario: {
    primaryTenant: E2ETwitterTenant;
    secondaryTenant: E2ETwitterTenant;
    actorDisplayName: string;
  };
  presentReadyStudent: E2EStudentRef;
  studentSearchScenario: E2EStudentPair;
  groupVisibilityScenario: E2EGroupPair;
  checkinScenario: E2ECheckinFixture;
  checkinDevice: {
    key: string;
    api_key: string;
    pin: string;
  };
  checkinHarness: CheckinHarness;
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

type CurrentVisit = {
  id: number;
} | null;

type CheckinRequestBody = {
  student_rfid: string;
  action: "checkin";
  room_id: number;
};

type CheckinScanResult = {
  student_id: number;
  student_name?: string;
  action: string;
  visit_id?: number | null;
};

type CheckinHarness = {
  scan(): Promise<CheckinScanResult>;
  currentVisit(studentId?: number): Promise<CurrentVisit>;
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

async function getCurrentVisit(
  adminApi: APIRequestContext,
  studentId: number,
): Promise<CurrentVisit> {
  const visitRes = await adminApi.get(
    `/api/students/${studentId}/current-visit`,
    {
      failOnStatusCode: false,
    },
  );
  if (visitRes.status() === 200) {
    const body = (await visitRes.json()) as {
      data?: {
        id: number;
      } | null;
    };
    return body.data ?? null;
  }

  throw new Error(
    `current-visit lookup failed for student ${studentId} (${visitRes.status()}): ${await visitRes.text()}`,
  );
}

async function scanCheckin(
  deviceApi: APIRequestContext,
  requestBody: CheckinRequestBody,
): Promise<CheckinScanResult> {
  const res = await deviceApi.post("/api/iot/checkin", {
    data: requestBody,
  });
  if (!res.ok()) {
    throw new Error(
      `device checkin failed (${res.status()}): ${await res.text()}`,
    );
  }

  const body = (await res.json()) as {
    data: CheckinScanResult;
  };
  return body.data;
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
    await use(getE2EManifest().tenants.primary); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  secondaryTenant: async ({}, use) => {
    await use(requireSecondaryTenant()); // NOSONAR — Playwright fixture callback, not React Hook
  },
  tenantSwitchScenario: async ({ primaryTenant, secondaryTenant }, use) => {
    const manifest = getE2EManifest();
    await use({
      primaryTenant,
      secondaryTenant,
      actorDisplayName: manifest.actors.admin.display_name,
    }); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  presentReadyStudent: async ({}, use) => {
    await use(getE2EManifest().fixtures.students.present_ready); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  studentSearchScenario: async ({}, use) => {
    await use(getE2EManifest().fixtures.students.search_pair); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  groupVisibilityScenario: async ({}, use) => {
    await use(getE2EManifest().fixtures.groups.visible_pair); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  checkinScenario: async ({}, use) => {
    await use(getE2EManifest().fixtures.checkin); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  checkinDevice: async ({}, use) => {
    await use(getE2EManifest().devices.default_checkin); // NOSONAR — Playwright fixture callback, not React Hook
  },
  checkinHarness: async ({ adminApi, deviceApi, checkinScenario }, use) => {
    const requestBody: CheckinRequestBody = {
      student_rfid: checkinScenario.rfid_tag,
      action: "checkin",
      room_id: checkinScenario.room.id,
    };

    await use({
      scan: async () => scanCheckin(deviceApi, requestBody),
      currentVisit: async (studentId = checkinScenario.student.id) =>
        getCurrentVisit(adminApi, studentId),
    }); // NOSONAR — Playwright fixture callback, not React Hook
  },
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
  deviceApi: async ({ checkinDevice }, use) => {
    const ctx = await createDeviceApiContext(checkinDevice);
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
