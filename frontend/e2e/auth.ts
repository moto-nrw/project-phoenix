import { expect, type Page } from "@playwright/test";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { getE2EState } from "./state";

const HERE = dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = resolve(HERE, "..");

const SCENARIO_MODE_MULTI_TENANT = "multi-tenant";
const SETUP_ROLE_ADMIN = "admin";
const SETUP_ROLE_STAFF = "staff";

type E2ETwitterTenant = {
  slug: string;
};

type TenantActor = {
  email: string;
  password: string;
  display_name: string;
  role: string;
  staff_id: number;
};

type E2EAuthSetup = {
  roles: string[];
  requires_secondary_tenant: boolean;
  requires_verified_switching: boolean;
};

type AuthState = {
  runtime: {
    tenant_domain: string;
    frontend_port: number;
  };
  world: {
    scenario: {
      mode: string;
    };
    tenants: {
      primary: E2ETwitterTenant;
      secondary?: E2ETwitterTenant;
    };
    actors: {
      admin: TenantActor;
      staff: TenantActor;
    };
  };
  setup: {
    auth: E2EAuthSetup;
  };
  assertions: {
    switching: {
      verified: boolean;
    };
  };
};

export type Role = "admin" | "staff";

export const STORAGE_STATE_PATH: Record<Role, string> = {
  admin: resolve(FRONTEND_DIR, "e2e", ".auth", "admin.json"),
  staff: resolve(FRONTEND_DIR, "e2e", ".auth", "staff.json"),
};

export interface TenantSession {
  role: Role;
  actor: TenantActor;
  email: string;
  password: string;
  displayName: string;
  storageStatePath: string;
  appRoot: string;
  readyIndicator: {
    kind: "display-name";
    value: string;
  };
}

export interface AuthSetupContract {
  admin: TenantSession;
  staff: TenantSession;
}

let cachedAuthSetup: AuthSetupContract | undefined;

function readAuthState(): AuthState {
  return getE2EState() as AuthState;
}

function escapeRegex(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function profileTriggerLocator(page: Page, displayName: string) {
  return page
    .getByRole("banner")
    .getByRole("button", { name: new RegExp(escapeRegex(displayName)) })
    .first();
}

function tenantOrigin(state: AuthState, tenant: E2ETwitterTenant): string {
  return `http://${tenant.slug}.${state.runtime.tenant_domain}:${state.runtime.frontend_port}`;
}

function buildAuthSetupContract(): AuthSetupContract {
  const state = readAuthState();
  const primaryAppRoot = tenantOrigin(state, state.world.tenants.primary);

  return {
    admin: {
      role: "admin",
      actor: state.world.actors.admin,
      email: state.world.actors.admin.email,
      password: state.world.actors.admin.password,
      displayName: state.world.actors.admin.display_name,
      storageStatePath: STORAGE_STATE_PATH.admin,
      appRoot: primaryAppRoot,
      readyIndicator: {
        kind: "display-name",
        value: state.world.actors.admin.display_name,
      },
    },
    staff: {
      role: "staff",
      actor: state.world.actors.staff,
      email: state.world.actors.staff.email,
      password: state.world.actors.staff.password,
      displayName: state.world.actors.staff.display_name,
      storageStatePath: STORAGE_STATE_PATH.staff,
      appRoot: primaryAppRoot,
      readyIndicator: {
        kind: "display-name",
        value: state.world.actors.staff.display_name,
      },
    },
  };
}

export function getAuthSetupContract(): AuthSetupContract {
  if (!cachedAuthSetup) {
    cachedAuthSetup = buildAuthSetupContract();
  }
  return cachedAuthSetup;
}

export function verifyHarnessState(): void {
  const state = readAuthState();
  verifyAuthSetup(state.setup.auth, state);
  expect(state.world.scenario.mode).toBe(SCENARIO_MODE_MULTI_TENANT);
}

function verifyAuthSetup(setup: E2EAuthSetup, state = readAuthState()): void {
  expect(setup.roles).toEqual([SETUP_ROLE_ADMIN, SETUP_ROLE_STAFF]);

  if (setup.requires_secondary_tenant) {
    expect(state.world.tenants.secondary).toBeTruthy();
  }

  if (setup.requires_verified_switching) {
    expect(state.assertions.switching.verified).toBe(true);
  }
}

export async function assertSessionReady(
  page: Page,
  session: TenantSession,
): Promise<void> {
  await expect(
    profileTriggerLocator(page, session.readyIndicator.value),
  ).toBeVisible({ timeout: 30000 });
}

export async function waitForLoginFormReady(page: Page): Promise<void> {
  const emailInput = page.getByRole("textbox", { name: "E-Mail-Adresse" });
  const submitButton = page.getByRole("button", { name: "Anmelden" });

  await expect(
    page.getByText(/Sitzung wird überprüft…|Sie werden weitergeleitet…/),
  ).toHaveCount(0, {
    timeout: 20000,
  });
  await expect(emailInput).toBeVisible({ timeout: 20000 });
  await expect(submitButton).toBeVisible({ timeout: 20000 });
}

export async function loginViaUI(
  page: Page,
  credentials: { email: string; password: string },
  startUrl: string,
): Promise<void> {
  const invalidCredentials = page.getByText("Ungültige E-Mail oder Passwort");

  for (let attempt = 1; attempt <= 2; attempt++) {
    await page.goto(startUrl);
    await waitForLoginFormReady(page);

    await page
      .getByRole("textbox", { name: "E-Mail-Adresse" })
      .fill(credentials.email);
    await page
      .getByRole("textbox", { name: "Passwort" })
      .fill(credentials.password);
    await page.getByRole("button", { name: "Anmelden" }).click();

    try {
      await page.waitForSelector('input[name="email"]', {
        state: "detached",
        timeout: 5000,
      });
      return;
    } catch {
      if (await invalidCredentials.isVisible()) {
        if (attempt === 2) {
          throw new Error(
            "loginViaUI failed twice with 'Ungültige E-Mail oder Passwort'",
          );
        }
        continue;
      }
    }

    await page.waitForURL((url) => !/\/login(\/|$)/.test(url.pathname), {
      timeout: 15000,
    });
    await page.waitForSelector('input[name="email"]', {
      state: "detached",
      timeout: 15000,
    });
    return;
  }
}

export function _resetAuthSetupCacheForTesting(): void {
  cachedAuthSetup = undefined;
}
