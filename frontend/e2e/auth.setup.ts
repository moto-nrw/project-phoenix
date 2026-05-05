import { test as setup } from "@playwright/test";
import { loginViaUI } from "./helpers/auth";
import { ADMIN, STAFF, STORAGE_STATE_PATH } from "./helpers/seed-data";

setup("authenticate as admin", async ({ page }) => {
  await loginViaUI(page, ADMIN);
  await page.context().storageState({ path: STORAGE_STATE_PATH.admin });
});

setup("authenticate as staff", async ({ page }) => {
  await loginViaUI(page, STAFF);
  await page.context().storageState({ path: STORAGE_STATE_PATH.staff });
});
