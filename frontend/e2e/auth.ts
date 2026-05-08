import { expect, type Page } from "@playwright/test";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import {
  getAppUrls,
  getE2EManifest,
  type SeedAdminActor,
  type SeedStaffActor,
} from "./contract";

const HERE = dirname(fileURLToPath(import.meta.url));
const FRONTEND_DIR = resolve(HERE, "..");

type TenantActor = SeedAdminActor | SeedStaffActor;

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
  const manifest = getE2EManifest();
  const app = getAppUrls();

  return {
    admin: {
      role: "admin",
      actor: manifest.actors.admin,
      email: manifest.actors.admin.email,
      password: manifest.actors.admin.password,
      displayName: manifest.actors.admin.display_name,
      storageStatePath: STORAGE_STATE_PATH.admin,
      appRoot: app.primary(),
      readyIndicator: {
        kind: "display-name",
        value: manifest.actors.admin.display_name,
      },
    },
    staff: {
      role: "staff",
      actor: manifest.actors.staff,
      email: manifest.actors.staff.email,
      password: manifest.actors.staff.password,
      displayName: manifest.actors.staff.display_name,
      storageStatePath: STORAGE_STATE_PATH.staff,
      appRoot: app.primary(),
      readyIndicator: {
        kind: "display-name",
        value: manifest.actors.staff.display_name,
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

export function verifyHarnessManifest(): void {
  const manifest = getE2EManifest();
  expect(manifest.scenario.name).toBe("e2e-multi-tenant");
  expect(manifest.scenario.mode).toBe("multi-tenant");
  expect(manifest.tenants.secondary).toBeTruthy();
  expect(manifest.switching?.verified).toBe(true);
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
