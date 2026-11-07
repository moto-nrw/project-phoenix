import { expect, test, type Page } from "@playwright/test";

import {
  loginToTenant,
  MISSING_SEED_ACCESS,
  seedAccess as access,
  tenantBase as base,
} from "./tenant-session";

// Scrollen, über die Seitenleiste wechseln, Position prüfen (#5681). Die Regeln
// im Einzelnen prüft `portal-shell-scroll.test.tsx`; hier geht es um das, was
// nur der echte Browser zeigt: Next lässt die Position stehen, solange der
// Seitenanfang im Sichtbereich liegt, auch hinter der klebenden Kopfzeile, und
// „Zurück“ stellt der Browser selbst wieder her.
//
// Das niedrige Fenster ist Absicht: Weit gescrollt setzt oft schon die kurze
// Ladehülle der Zielseite die Position nebenbei auf 0, und der Test bliebe auch
// ohne Korrektur grün.

if (access) test.use({ baseURL: base, viewport: { width: 1280, height: 420 } });

const scrollY = (page: Page) => page.evaluate(() => window.scrollY);

// Die Aktivitätenliste lädt wenig nach. Auf einer datenreichen Seite wie `/home`
// hinge das Ergebnis von „Zurück“ an der Antwortzeit des Entwicklungsservers.
async function openLoadedSourceAt(page: Page, top: number) {
  await page.goto("/activities", { waitUntil: "domcontentloaded" });
  await expect(
    page.getByRole("heading", { level: 1, name: "Aktivitäten" }),
  ).toBeVisible();
  // Auch das Ladegerüst ist scrollbar. Erst eine echte Kachel belegt, dass
  // der spätere Listenaufbau die Ausgangsposition nicht mehr verschiebt.
  await expect(
    page
      .getByRole("main")
      .getByRole("button", { name: / öffnen$/ })
      .first(),
  ).toBeVisible();
  await expect
    .poll(async () => {
      await page.evaluate((y) => window.scrollTo(0, y), top);
      return scrollY(page);
    })
    .toBe(top);
}

async function openRoomsFromSidebar(page: Page) {
  await page.getByRole("link", { name: "Räume", exact: true }).first().click();
  await page.waitForURL((url) => url.pathname.endsWith("/rooms"));
  await expect(
    page.getByRole("heading", { level: 1, name: "Räume" }),
  ).toBeVisible();
}

test.describe("Seitenwechsel über die Seitenleiste", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(access === null, MISSING_SEED_ACCESS);
    if (!access) return;
    await loginToTenant(page, access);
  });

  test("beginnt die neue Seite oben", async ({ page }) => {
    await openLoadedSourceAt(page, 40);

    await openRoomsFromSidebar(page);

    expect(await scrollY(page)).toBe(0);
  });

  test("Zurück stellt die vorige Position wieder her", async ({ page }) => {
    await openLoadedSourceAt(page, 40);
    await openRoomsFromSidebar(page);

    await page.goBack();
    await page.waitForURL((url) => url.pathname.endsWith("/activities"));

    await expect.poll(() => scrollY(page)).toBe(40);
  });

  // Die kurze Zielseite ist der schwere Fall: Steht das Dokument dort oben,
  // begrenzt der Browser die gespeicherte Position beim Zurück auf deren Höhe.
  test("Zurück von einer kurzen Seite stellt auch eine weite Position wieder her", async ({
    page,
  }) => {
    await openLoadedSourceAt(page, 400);

    // Im niedrigen Fenster liegt der Eintrag am Rand der Leiste, die ihn beim
    // Zeigen aus dem Weg schiebt; das Ereignis trifft denselben Link.
    await page
      .getByRole("link", { name: "Umfragen", exact: true })
      .first()
      .dispatchEvent("click");
    await page.waitForURL((url) => url.pathname.endsWith("/umfragen"));
    await expect(
      page.getByRole("heading", { level: 1, name: "Umfragen" }),
    ).toBeVisible();
    expect(await scrollY(page)).toBe(0);

    await page.goBack();
    await page.waitForURL((url) => url.pathname.endsWith("/activities"));

    await expect.poll(() => scrollY(page)).toBe(400);
  });
});
