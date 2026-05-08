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
import { getE2EState } from "./state";

export type E2ETwitterTenant = {
  slug: string;
  name: string;
  school_id: number;
  organization_id: number;
};

export type E2EStudentRef = {
  id: number;
  first_name: string;
  last_name: string;
  group_key: string;
  class: string;
};

export type E2EStudentPair = {
  primary: E2EStudentRef;
  secondary: E2EStudentRef;
};

export type E2EGroupPair = {
  primary: {
    id: number;
    key: string;
    display_name: string;
  };
  secondary: {
    id: number;
    key: string;
    display_name: string;
  };
};

type AppUrls = {
  primary(path?: string): string;
  secondary(path?: string): string;
  tenant(tenant: E2ETwitterTenant, path?: string): string;
  origin(tenant: E2ETwitterTenant): string;
};

type FixtureState = {
  runtime: {
    backend_url: string;
    tenant_domain: string;
    frontend_port: number;
  };
  world: {
    tenants: {
      primary: E2ETwitterTenant;
      secondary?: E2ETwitterTenant;
    };
    devices: {
      default_checkin: {
        key: string;
        api_key: string;
        pin: string;
      };
    };
  };
  fixtures: {
    students: {
      present_ready: E2EStudentRef;
      search_pair: E2EStudentPair;
    };
    groups: {
      visible_pair: E2EGroupPair;
    };
    checkin: {
      student: E2EStudentRef;
      room: {
        id: number;
        name: string;
      };
      activity: {
        id: number;
        name: string;
      };
      device_key: string;
      rfid_tag: string;
      supervisor: {
        email: string;
        password: string;
        display_name: string;
        role: string;
        staff_id: number;
      };
    };
  };
};

type Fixtures = {
  app: AppUrls;
  authSessions: AuthSetupContract;
  primaryTenant: E2ETwitterTenant;
  secondaryTenant: E2ETwitterTenant;
  presentReadyStudent: E2EStudentRef;
  studentSearchScenario: E2EStudentPair;
  groupVisibilityScenario: E2EGroupPair;
  checkinScenario: FixtureState["fixtures"]["checkin"];
  checkinDevice: {
    key: string;
    api_key: string;
    pin: string;
  };
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

function readFixtureState(): FixtureState {
  return getE2EState() as FixtureState;
}

function normalizePath(path = "/"): string {
  return path.startsWith("/") ? path : `/${path}`;
}

function tenantOrigin(state: FixtureState, tenant: E2ETwitterTenant): string {
  return `http://${tenant.slug}.${state.runtime.tenant_domain}:${state.runtime.frontend_port}`;
}

function tenantAppURL(
  state: FixtureState,
  tenant: E2ETwitterTenant,
  path = "/",
): string {
  return `${tenantOrigin(state, tenant)}${normalizePath(path)}`;
}

function getAppUrls(): AppUrls {
  const state = readFixtureState();
  return {
    primary: (path = "/") =>
      tenantAppURL(state, state.world.tenants.primary, path),
    secondary: (path = "/") =>
      tenantAppURL(state, requireSecondaryTenant(state), path),
    tenant: (tenant, path = "/") => tenantAppURL(state, tenant, path),
    origin: (tenant) => tenantOrigin(state, tenant),
  };
}

function requireSecondaryTenant(
  state: FixtureState = readFixtureState(),
): E2ETwitterTenant {
  const tenant = state.world.tenants.secondary;
  if (!tenant) {
    throw new Error(
      "e2e state secondary tenant is missing. Re-run `go run . e2e prepare` from backend/.",
    );
  }
  return tenant;
}

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
    await use(readFixtureState().world.tenants.primary); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  secondaryTenant: async ({}, use) => {
    await use(requireSecondaryTenant()); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  presentReadyStudent: async ({}, use) => {
    await use(readFixtureState().fixtures.students.present_ready); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  studentSearchScenario: async ({}, use) => {
    await use(readFixtureState().fixtures.students.search_pair); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  groupVisibilityScenario: async ({}, use) => {
    await use(readFixtureState().fixtures.groups.visible_pair); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  checkinScenario: async ({}, use) => {
    await use(readFixtureState().fixtures.checkin); // NOSONAR — Playwright fixture callback, not React Hook
  },
  // oxlint-disable-next-line no-empty-pattern
  checkinDevice: async ({}, use) => {
    await use(readFixtureState().world.devices.default_checkin); // NOSONAR — Playwright fixture callback, not React Hook
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
