import { expect, type Locator, type Page } from "@playwright/test";

import { loadSeedAccess, type SeedAccess } from "../scripts/seed-state";
import { loginToLocalTenant } from "./local-tenant-login";

// Gemeinsamer Tenant-Zugang der E2E-Specs. Der lokale Stack bedient die
// Tenant-App nur unter {slug}.localhost:3000 (bare localhost ist die
// Marketing-Seite); die Login-Maske liegt auf der Tenant-Wurzel `/`. Slug und
// Admin-Zugang stehen in backend/.seed-state.json; E2E_TENANT_SLUG /
// E2E_TEST_EMAIL / E2E_TEST_PASSWORD überschreiben sie. Ohne verwertbare Werte
// überspringen die Specs.

export const seedAccess: SeedAccess | null = loadSeedAccess();

// Lokal überspringen die Specs ohne Seed-Zugang. In CI (.github/workflows/
// e2e.yml) wäre ein Lauf aus lauter Skips grün, ohne etwas zu prüfen.
if (process.env.CI && seedAccess === null) {
  throw new Error(
    "E2E in CI braucht backend/.seed-state.json; der Seed-Schritt ist ausgefallen.",
  );
}

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
  await loginToLocalTenant(page, tenantBase, access);

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

/**
 * Klappt eine Gruppe der Desktop-Seitenleiste auf (etwa "Planung", #2826).
 * Die Gruppen stellen ihren gemerkten Zustand erst nach der Hydration her; ein
 * einzelner Klick kann eine gerade aufgeklappte Gruppe wieder zuklappen.
 */
export async function openSidebarGroup(page: Page, name: string) {
  const group = page
    .locator("aside")
    .getByRole("button", { name, exact: true });
  await expect(async () => {
    if ((await group.getAttribute("aria-expanded")) !== "true") {
      await group.click();
    }
    await expect(group).toHaveAttribute("aria-expanded", "true", {
      timeout: 1000,
    });
  }).toPass({ timeout: 15000 });
}

/**
 * Wartet, bis die endlichen Animationen in einem Bereich durch sind, etwa das
 * Einfahren eines SlideOvers. Ein Klick während des Einfahrens trifft die Stelle,
 * an der der Knopf gerade noch nicht ist, also den Hintergrund; das schließt den
 * Dialog ohne Aktion.
 */
export async function waitForAnimations(locator: Locator) {
  await locator.evaluate((element) =>
    Promise.all(
      element
        .getAnimations({ subtree: true })
        .filter(
          (animation) =>
            animation.effect?.getComputedTiming().endTime !== Infinity,
        )
        .map((animation) => animation.finished.catch(() => undefined)),
    ),
  );
}

export { pickDate } from "./date-picker";
