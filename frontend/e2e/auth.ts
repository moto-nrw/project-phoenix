import { expect, type Page } from "@playwright/test";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import {
  getAdminActor,
  getAuthSetup,
  getPrimaryTenant,
  getScenarioMode,
  getStaffActor,
  isTenantSwitchVerified,
  requireSecondaryTenant,
  tenantOrigin,
  type E2EActor,
} from "./state";

const HERE = dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = resolve(HERE, "..");

const SETUP_ROLE_ADMIN = "admin";
const SETUP_ROLE_STAFF = "staff";

export type Role = "admin" | "staff";

export const STORAGE_STATE_PATH: Record<Role, string> = {
  admin: resolve(FRONTEND_DIR, "e2e", ".auth", "admin.json"),
  staff: resolve(FRONTEND_DIR, "e2e", ".auth", "staff.json"),
};

export interface TenantSession {
  role: Role;
  actor: E2EActor;
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

function escapeRegex(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

function profileTriggerLocator(page: Page, displayName: string) {
  return page
    .getByRole("banner")
    .getByRole("button", { name: new RegExp(escapeRegex(displayName)) })
    .first();
}

function buildAuthSetupContract(): AuthSetupContract {
  const adminActor = getAdminActor();
  const staffActor = getStaffActor();
  const primaryAppRoot = tenantOrigin(getPrimaryTenant());

  return {
    admin: {
      role: "admin",
      actor: adminActor,
      email: adminActor.email,
      password: adminActor.password,
      displayName: adminActor.display_name,
      storageStatePath: STORAGE_STATE_PATH.admin,
      appRoot: primaryAppRoot,
      readyIndicator: {
        kind: "display-name",
        value: adminActor.display_name,
      },
    },
    staff: {
      role: "staff",
      actor: staffActor,
      email: staffActor.email,
      password: staffActor.password,
      displayName: staffActor.display_name,
      storageStatePath: STORAGE_STATE_PATH.staff,
      appRoot: primaryAppRoot,
      readyIndicator: {
        kind: "display-name",
        value: staffActor.display_name,
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
  verifyAuthSetup();
  expect(getScenarioMode()).toBe("multi-tenant");
}

function verifyAuthSetup(): void {
  const setup = getAuthSetup();
  expect(setup.roles).toEqual([SETUP_ROLE_ADMIN, SETUP_ROLE_STAFF]);

  if (setup.requires_secondary_tenant) {
    expect(requireSecondaryTenant()).toBeTruthy();
  }

  if (setup.requires_verified_switching) {
    expect(isTenantSwitchVerified()).toBe(true);
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
