import { expect, type Page } from "@playwright/test";

import { loadSeedAccess, type SeedAccess } from "../scripts/seed-state";

// Gemeinsamer Tenant-Zugang der E2E-Specs. Der lokale Stack bedient die
// Tenant-App nur unter {slug}.localhost:3000 (bare localhost ist die
// Marketing-Seite); die Login-Maske liegt auf der Tenant-Wurzel `/`. Slug und
// Admin-Zugang stehen in backend/.seed-state.json; E2E_TENANT_SLUG /
// E2E_TEST_EMAIL / E2E_TEST_PASSWORD überschreiben sie. Ohne verwertbare Werte
// überspringen die Specs.

export const seedAccess: SeedAccess | null = loadSeedAccess();

export const tenantBase = seedAccess
  ? `http://${seedAccess.slug}.localhost:3000`
  : "";

export const MISSING_SEED_ACCESS =
  "backend/.seed-state.json (oder E2E_TENANT_SLUG/E2E_TEST_EMAIL/E2E_TEST_PASSWORD) erforderlich";

/**
 * Meldet den Seed-Admin an und blendet den Benachrichtigungs-Einstieg aus.
 *
 * Der geführte Benachrichtigungs-Einstieg (#2831, PortalNotificationSetup im
 * Tenant-Layout) öffnet sich nach dem Login asynchron als Modal (Headless-
 * Chromium meldet Notification.permission "denied") und fängt Klicks ab. Ein
 * Locator-Handler, der ihn schließt, kann mitten in einem offenen Editor
 * feuern. Deshalb wird die Entscheidung "erledigt" unter dem Schlüssel aus
 * notification-setup-decision.ts (setupStorageKey) im localStorage hinterlegt
 * und die Seite neu geladen, bevor die Spec weiterarbeitet.
 */
export async function loginToTenant(
  page: Page,
  access: SeedAccess,
): Promise<void> {
  // `domcontentloaded` statt `load`: der Next-Dev-Server hält über SSE
  // Verbindungen offen, das `load`-Event kann dadurch hängen.
  await page.goto(`${tenantBase}/`, { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[type="email"]', { timeout: 20000 });
  await page.fill('input[type="email"]', access.email);
  await page.fill('input[type="password"]', access.password);
  await page.click('button:has-text("Anmelden")');
  await page.waitForSelector('input[type="email"]', {
    state: "detached",
    timeout: 20000,
  });

  const res = await page.request.get(`${tenantBase}/api/auth/session`);
  expect(res.ok(), `Session laden fehlgeschlagen (${res.status()})`).toBe(true);
  const session = (await res.json()) as { user?: { id?: string } };
  const accountId = session.user?.id;
  expect(accountId, "Session ohne user.id").toBeTruthy();
  const key = `moto.tenant.notification-setup.v1.${access.slug}.${accountId}`;
  await page.evaluate((storageKey) => {
    localStorage.setItem(storageKey, JSON.stringify({ done: true }));
  }, key);
  await page.reload({ waitUntil: "domcontentloaded" });
}
