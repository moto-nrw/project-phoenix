import { mkdirSync, rmSync } from "node:fs";
import { dirname } from "node:path";
import { test as setup } from "@playwright/test";
import {
  assertSessionReady,
  getAuthSetupContract,
  loginViaUI,
  verifyHarnessManifest,
} from "./auth";
import { getE2EManifest } from "./contract";

setup.describe.configure({ mode: "serial" });

function resetAuthArtifacts(): void {
  const sessions = getAuthSetupContract();
  mkdirSync(dirname(sessions.admin.storageStatePath), { recursive: true });
  rmSync(sessions.admin.storageStatePath, { force: true });
  rmSync(sessions.staff.storageStatePath, { force: true });
}

function authSessions() {
  return getAuthSetupContract();
}

setup("e2e manifest contract is valid", async () => {
  resetAuthArtifacts();
  getE2EManifest();
  verifyHarnessManifest();
});

setup("authenticate as admin", async ({ page }) => {
  const admin = authSessions().admin;

  await loginViaUI(page, admin.actor, admin.appRoot);
  await assertSessionReady(page, admin);
  await page.context().storageState({ path: admin.storageStatePath });
});

setup("authenticate as staff", async ({ page }) => {
  const staff = authSessions().staff;

  await loginViaUI(page, staff.actor, staff.appRoot);
  await assertSessionReady(page, staff);
  await page.context().storageState({ path: staff.storageStatePath });
});
