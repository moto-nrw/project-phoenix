import { expect, test, type Locator, type Page } from "@playwright/test";

import { loadSeedAccess, type SeedAccess } from "../scripts/seed-state";
import { frontendPort } from "./frontend-port";
import { loginToLocalTenant } from "./local-tenant-login";

// Gemeinsamer Tenant-Zugang der E2E-Specs. Der lokale Stack bedient die
// Tenant-App nur unter {slug}.localhost:<PORT> (bare localhost ist die
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
  ? `http://${seedAccess.slug}.localhost:${frontendPort}`
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
  // Ein anderes Seed-Profil als das Standardprofil (etwa `manuell`).
  base = tenantBase,
): Promise<void> {
  // `domcontentloaded` statt `load`: der Next-Dev-Server hält über SSE
  // Verbindungen offen, das `load`-Event kann dadurch hängen.
  await loginToLocalTenant(page, base, access);
  await waitForSignedInRedirect(page);

  const res = await page.request.get(`${base}/api/auth/session`);
  expect(res.ok(), `Session laden fehlgeschlagen (${res.status()})`).toBe(true);
  const session = (await res.json()) as { user?: { id?: string } };
  const accountId = session.user?.id;
  expect(accountId, "Session ohne user.id").toBeTruthy();
  const key = `moto.tenant.notification-setup.v1.${access.slug}.${accountId}`;
  await page.evaluate((storageKey) => {
    localStorage.setItem(storageKey, JSON.stringify({ done: true }));
  }, key);
  await page.reload({ waitUntil: "domcontentloaded" });
  await waitForSignedInRedirect(page);
}

/** Längste Zeit für die Weiterleitung von `/`, samt kalter Übersetzung. */
const SIGNED_IN_REDIRECT_TIMEOUT = 60_000;

/**
 * Wartet, bis die Tenant-Wurzel `/` ein angemeldetes Konto weitergeleitet hat.
 * Die Wurzel ist die Login-Maske; mit Sitzung zeigt sie „Sie sind bereits
 * angemeldet“ und leitet erst im Browser weiter (SmartRedirect, nach /home oder
 * zur Schul-Übergabe, für jedes Konto weg von `/`). Ohne dieses Warten zerstörte
 * die Weiterleitung den Kontext von page.evaluate, oder die Spec prüfte noch die
 * Login-Maske.
 *
 * Auf einem kalten Next-Dev-Server übersetzt die erste Weiterleitung die
 * Zielseite; das kostet im CI-Job bis zu 30 s. Seit #6937 laufen die Shards in
 * eigenen Jobs mit kaltem Server, und die erste Spec eines Shards verbrauchte
 * damit ihr ganzes Zeitbudget im Login. Diese Zeit zählt deshalb nicht gegen
 * den Test: Er bekommt das Warten als Zeit dazu. Hängt die Weiterleitung,
 * meldet waitForURL das selbst, bevor das Budget endet. `commit`, nicht `load`:
 * der Dev-Server hält SSE-Verbindungen offen, `load` kann hängen.
 */
async function waitForSignedInRedirect(page: Page): Promise<void> {
  // Nur verlängern, nie kürzen: Ein Aufrufer (etwa ein beforeAll mit eigenem
  // Budget) behält seine Zeit.
  test.info().setTimeout(test.info().timeout + SIGNED_IN_REDIRECT_TIMEOUT);
  await page.waitForURL((url) => url.pathname !== "/", {
    timeout: SIGNED_IN_REDIRECT_TIMEOUT,
    waitUntil: "commit",
  });
}

/** Längste Zeit, die eine kalte Seite zum Bauen und Hochfahren bekommt. */
const WARM_PAGE_TIMEOUT = 60_000;

/**
 * Baut Seiten auf dem Next-Dev-Server vor, bevor ein Test sie misst. Der Server
 * baut eine Seite beim ersten Aufruf; auf dem kalten Server eines CI-Shards
 * (seit #6937) kostet das je Seite viele Sekunden und belegt die CPU, die der
 * Browser für Klicks braucht. Fertig ist eine Seite, wenn ihr Code im Browser
 * läuft: Sie stellt ihre erste Anfrage an `/api/…` (wie warmTourPages in
 * onboarding-tours.spec.ts). Aufrufen in einem `beforeAll` mit eigenem Budget.
 */
export async function warmPages(
  page: Page,
  paths: readonly string[],
  base = tenantBase,
): Promise<void> {
  for (const path of paths) {
    const running = page.waitForRequest(
      (request) => {
        const url = new URL(request.url());
        if (
          !url.pathname.startsWith("/api/") ||
          url.pathname === "/api/logs" ||
          !["fetch", "xhr"].includes(request.resourceType())
        ) {
          return false;
        }
        try {
          return new URL(request.frame().url()).pathname === path;
        } catch {
          // Anfragen eines Service Workers haben keinen Frame.
          return false;
        }
      },
      { timeout: WARM_PAGE_TIMEOUT },
    );
    await page.goto(`${base}${path}`, {
      waitUntil: "domcontentloaded",
      timeout: WARM_PAGE_TIMEOUT,
    });
    await running;
  }
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
