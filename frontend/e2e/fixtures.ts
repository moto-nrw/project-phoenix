import {
  expect,
  test as base,
  type APIRequestContext,
  type APIResponse,
  type Browser,
  type Page,
} from "@playwright/test";
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
import {
  getAppUrls,
  getCheckinDevice,
  getCheckinScenario,
  getGroupVisibilityScenario,
  getPresentReadyStudent,
  getPrimaryTenant,
  getStudentSearchScenario,
  requireSecondaryTenant,
  type AppUrls,
  type E2ECheckinFixture,
  type E2EDevice,
  type E2EGroupPair,
  type E2EStudentPair,
  type E2EStudentRef,
  type E2ETwitterTenant,
} from "./state";

type StudentSearchProbe = {
  searchTerm: string;
  expectedVisibleName: string;
  expectedFilteredOutName: string;
};

type GroupVisibilityProbe = {
  expectedVisibleNames: [string, string];
};

type PresentReadyStudentCard = {
  fullName: string;
  markSick(): Promise<void>;
  clearSick(): Promise<void>;
};

type CheckinScan = {
  studentId: number;
  studentName?: string;
  action: string;
  visitId?: number | null;
};

type CurrentVisit = {
  id: number;
} | null;

type CheckinFlow = {
  scan(): Promise<CheckinScan>;
  readCurrentVisit(): Promise<CurrentVisit>;
  expectSeededStudent(scan: CheckinScan): void;
  expectPersistedVisitMatches(scan: CheckinScan, visit: CurrentVisit): void;
};

type IotContract = {
  getConfigWithoutAuth(): Promise<APIResponse>;
  getConfigWithInvalidApiKey(): Promise<APIResponse>;
  getConfigWithMalformedAuthorizationHeader(): Promise<APIResponse>;
  getConfigWithValidDevice(): Promise<APIResponse>;
  postCheckinWithoutPin(): Promise<APIResponse>;
  postCheckinWithWrongPin(): Promise<APIResponse>;
  postCheckinWithValidPinAndUnknownRFID(): Promise<APIResponse>;
};

type Fixtures = {
  app: AppUrls;
  authSessions: AuthSetupContract;
  primaryTenant: E2ETwitterTenant;
  secondaryTenant: E2ETwitterTenant;
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

function fullName(
  student: Pick<E2EStudentRef, "first_name" | "last_name">,
): string {
  return `${student.first_name} ${student.last_name}`;
}

async function requireOK(
  response: APIResponse,
  context: string,
): Promise<void> {
  if (response.ok()) {
    return;
  }
  throw new Error(
    `${context} (${response.status()}): ${await response.text()}`,
  );
}

function buildStudentSearchProbe(scenario: E2EStudentPair): StudentSearchProbe {
  return {
    searchTerm: scenario.primary.first_name,
    expectedVisibleName: fullName(scenario.primary),
    expectedFilteredOutName: fullName(scenario.secondary),
  };
}

function buildGroupVisibilityProbe(
  scenario: E2EGroupPair,
): GroupVisibilityProbe {
  return {
    expectedVisibleNames: [
      scenario.primary.display_name,
      scenario.secondary.display_name,
    ],
  };
}

function buildPresentReadyStudentCard(
  student: E2EStudentRef,
  adminApi: APIRequestContext,
): PresentReadyStudentCard {
  return {
    fullName: fullName(student),
    async markSick(): Promise<void> {
      const response = await adminApi.put(`/api/students/${student.id}`, {
        data: { sick: true, excused: false },
      });
      await requireOK(response, "mark present-ready student sick");
    },
    async clearSick(): Promise<void> {
      const response = await adminApi.put(`/api/students/${student.id}`, {
        data: { sick: false },
      });
      await requireOK(response, "clear present-ready student sick flag");
    },
  };
}

async function readCurrentVisit(
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

function buildCheckinFlow(
  scenario: E2ECheckinFixture,
  deviceApi: APIRequestContext,
  adminApi: APIRequestContext,
): CheckinFlow {
  return {
    async scan(): Promise<CheckinScan> {
      const response = await deviceApi.post("/api/iot/checkin", {
        data: {
          student_rfid: scenario.rfid_tag,
          action: "checkin",
          room_id: scenario.room.id,
        },
      });
      await requireOK(response, "device checkin");

      const body = (await response.json()) as {
        data: {
          student_id: number;
          student_name?: string;
          action: string;
          visit_id?: number | null;
        };
      };
      return {
        studentId: body.data.student_id,
        studentName: body.data.student_name,
        action: body.data.action,
        visitId: body.data.visit_id,
      };
    },
    async readCurrentVisit(): Promise<CurrentVisit> {
      return readCurrentVisit(adminApi, scenario.student.id);
    },
    expectSeededStudent(scan: CheckinScan): void {
      expect(scan.studentId).toBe(scenario.student.id);
      expect(scan.studentName).toContain(scenario.student.first_name);
    },
    expectPersistedVisitMatches(scan: CheckinScan, visit: CurrentVisit): void {
      if (scan.action === "checked_in") {
        expect(
          visit,
          "after a checked_in action the student must have a current open visit",
        ).not.toBeNull();
        expect(visit?.id).toBe(scan.visitId);
        return;
      }

      expect(
        visit,
        "after a checked_out action the student must have no current open visit",
      ).toBeNull();
    },
  };
}

function buildIotContract(
  backendApi: APIRequestContext,
  deviceApi: APIRequestContext,
  device: E2EDevice,
): IotContract {
  return {
    getConfigWithoutAuth(): Promise<APIResponse> {
      return backendApi.get("/api/iot/config");
    },
    getConfigWithInvalidApiKey(): Promise<APIResponse> {
      return backendApi.get("/api/iot/config", {
        headers: { Authorization: "Bearer dev_not_a_real_key" },
      });
    },
    getConfigWithMalformedAuthorizationHeader(): Promise<APIResponse> {
      return backendApi.get("/api/iot/config", {
        headers: { Authorization: device.api_key },
      });
    },
    getConfigWithValidDevice(): Promise<APIResponse> {
      return deviceApi.get("/api/iot/config");
    },
    postCheckinWithoutPin(): Promise<APIResponse> {
      return backendApi.post("/api/iot/checkin", {
        headers: {
          Authorization: `Bearer ${device.api_key}`,
        },
        data: { student_rfid: "TEST-RFID-001" },
      });
    },
    postCheckinWithWrongPin(): Promise<APIResponse> {
      return backendApi.post("/api/iot/checkin", {
        headers: {
          Authorization: `Bearer ${device.api_key}`,
          "X-Staff-PIN": "0000",
        },
        data: { student_rfid: "TEST-RFID-001" },
      });
    },
    postCheckinWithValidPinAndUnknownRFID(): Promise<APIResponse> {
      return deviceApi.post("/api/iot/checkin", {
        data: { student_rfid: "TEST-DOES-NOT-EXIST" },
      });
    },
  };
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
    await use(buildStudentSearchProbe(getStudentSearchScenario())); // NOSONAR
  },
  // oxlint-disable-next-line no-empty-pattern
  groupVisibilityProbe: async ({}, use) => {
    await use(buildGroupVisibilityProbe(getGroupVisibilityScenario())); // NOSONAR
  },
  presentReadyStudentCard: async ({ adminApi }, use) => {
    const card = buildPresentReadyStudentCard(
      getPresentReadyStudent(),
      adminApi,
    );
    await use(card); // NOSONAR — Playwright fixture callback, not React Hook
  },
  checkinFlow: async ({ adminApi, deviceApi }, use) => {
    const flow = buildCheckinFlow(getCheckinScenario(), deviceApi, adminApi);
    await use(flow); // NOSONAR — Playwright fixture callback, not React Hook
  },
  iotContract: async ({ backendApi, deviceApi }, use) => {
    const contract = buildIotContract(
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
