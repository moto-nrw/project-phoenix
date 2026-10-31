import { expect, test, type Locator, type Page } from "@playwright/test";

import {
  SETUP_TOUR_BRANCHES,
  SETUP_TOURS,
  type SetupTour,
  type SetupTourStop,
} from "../src/components/school-setup/setup-tours";
import { SCHOOL_SETUP_STEP_CONTENT } from "../src/components/school-setup/school-setup-steps";
import {
  STAFF_ONBOARDING_STEP_CONTENT,
  staffOnboardingTours,
} from "../src/components/staff-onboarding/staff-onboarding-steps";
import { STAFF_ONBOARDING_STEP_KEYS } from "../src/lib/staff-onboarding-api";
import { loadSeedAccess } from "../scripts/seed-state";
import { frontendPort } from "./frontend-port";
import {
  loginToTenant,
  MISSING_SEED_ACCESS,
  seedAccess,
  tenantBase,
} from "./tenant-session";

// Die geführten Touren der ersten Schritte (#2832, #3748) in der echten App.
// Die Unit-Tests der Touren zeigen auf nachgebaute Stellen; benennt eine
// Seite ihre Stelle um oder baut den Knopf um (#5142: „+ Personal“ wurde ein
// Menü), bleiben sie grün, und die Tour steht in der App mit „nicht zu
// sehen“. Diese Spec startet jede Tour, die der Seed anbietet, über „Zeig es
// mir“ und geht sie Station für Station durch: Die Sprechblase heißt wie die
// Station, der grüne Rahmen liegt auf einer Stelle, die der Selektor der
// Station trifft, und die Spec klickt genau diese Stelle („click“) oder
// „Weiter“ („next“).
//
// Seed-Zustände (backend/.seed-state.json):
// - Einrichtung: der schuleigene Admin von `manuell` (nicht `demo1`, dessen
//   MFA-Codes schon die übrigen Specs anfordern). Die Schule ist nicht
//   fertig eingerichtet. Offen
//   sind „Team“ und „Eltern“; Räume und Gruppen gelten dort nicht (binäre
//   Anwesenheit, offene Betreuung), Kinder sind erledigt. Erledigte Schritte
//   bieten keine Tour an. Jede geseedete Schule hat Räume, Gruppen und
//   Kinder; die Touren „rooms“, „groups“ und „students“ (samt Import) prüft
//   hier deshalb nur der Selektor-Test setup-tour-targets.test.ts.
// - Erste Schritte im Team: die erste Betreuungskraft des Standardprofils
//   `vollbetrieb` (Admins haben stattdessen die Einrichtung). Der Seed blendet
//   ihre Checkliste aus; vor jeder Tour setzt die Spec Ausblenden und
//   Schritte über die Routen der App zurück.
//
// Ohne Nebenwirkung: Die Touren der Einrichtung enden am Knopf, der eine
// Einladung verschickt oder etwas speichert. Die Spec prüft, dass er markiert
// ist, und beendet die Tour dort, statt Mails zu verschicken oder Daten
// anzulegen (SIDE_EFFECT_TARGETS).

if (seedAccess) test.use({ baseURL: tenantBase });
// Nacheinander in einem Worker: Computer und Handy setzen dieselbe Checkliste
// zurück. Anders als „serial“ läuft nach einem Fehler der Rest weiter.
test.describe.configure({ mode: "default" });

const SETUP_PROFILE = "manuell";
const setupAccess = seedAccess
  ? loadSeedAccess({ profile: SETUP_PROFILE, role: "school_admin" })
  : null;
const setupBase = setupAccess
  ? `http://${setupAccess.slug}.localhost:${frontendPort}`
  : "";
const staffAccess = seedAccess ? loadSeedAccess({ role: "betreuer" }) : null;

/** Stellen, deren Klick eine Mail verschickt oder Daten anlegt. */
const SIDE_EFFECT_TARGETS: ReadonlySet<string> = new Set([
  '[data-setup-tour="invite-submit"]',
  '[data-setup-tour="form-submit"]',
  '[data-setup-tour="student-submit"]',
  '[data-setup-tour="guardian-submit"]',
  '[data-setup-tour="student-import-submit"]',
  // Das Menü neben einer Person: „Einladen“ verschickt die Einladung.
  '[role="menu"]',
]);

/** Texte der Sprechblase, wenn die Stelle fehlt (Wizards, setup-tours.ts). */
const TARGET_MISSING = "Diese Stelle ist gerade nicht zu sehen.";

/** Die Sprechblase der Tour, auch solange sie die nächste Stelle sucht. */
function tourBubble(page: Page): Locator {
  return page.locator('[role="dialog"]', { hasText: /Station \d+ von \d+/ });
}

async function tourPosition(
  page: Page,
): Promise<{ index: number; total: number } | null> {
  const bubble = tourBubble(page);
  if ((await bubble.count()) === 0) return null;
  const text = (await bubble.first().textContent()) ?? "";
  const match = /Station (\d+) von (\d+)/.exec(text);
  if (!match) return null;
  return { index: Number(match[1]) - 1, total: Number(match[2]) };
}

/** Die Tour, in der die Station steht: die Tour selbst oder ein Zweig. */
function stopAt(
  tour: SetupTour,
  position: { index: number; total: number },
): SetupTourStop {
  const definition =
    position.total === tour.stops.length
      ? tour
      : Object.values(SETUP_TOUR_BRANCHES).find(
          (branch) => branch.stops.length === position.total,
        );
  const stop = definition?.stops[position.index];
  if (!stop) {
    throw new Error(
      `Station ${position.index + 1} von ${position.total} gehört zu keiner bekannten Tour.`,
    );
  }
  return stop;
}

/**
 * Markiert die Stelle unter dem grünen Rahmen, wenn sie den Selektor der
 * Station trifft. Der Rahmen steht 6 px um die Stelle (coach-mark.tsx);
 * `endBefore` kürzt nur seine Höhe.
 */
async function markHighlightedTarget(
  page: Page,
  selector: string,
): Promise<boolean> {
  return page.evaluate((targetSelector) => {
    for (const marked of document.querySelectorAll("[data-e2e-tour-target]")) {
      marked.removeAttribute("data-e2e-tour-target");
    }
    const ring = document.querySelector("div.ring-2.ring-moto-green");
    if (!ring) return false;
    const box = ring.getBoundingClientRect();
    for (const element of document.querySelectorAll(targetSelector)) {
      const rect = element.getBoundingClientRect();
      if (rect.width === 0 || rect.height === 0) continue;
      const close = (a: number, b: number) => Math.abs(a - b) <= 2;
      if (
        close(rect.left - 6, box.left) &&
        close(rect.top - 6, box.top) &&
        close(rect.width + 12, box.width)
      ) {
        element.setAttribute("data-e2e-tour-target", "");
        return true;
      }
    }
    return false;
  }, selector);
}

/** Die besuchten Stationen stehen im Bericht jedes Tests. */
function annotateStations(visited: readonly string[]): void {
  test.info().annotations.push({
    type: "Stationen",
    description: visited.join(" → "),
  });
}

/**
 * Am Computer führt jede Tour durch die Seitenleiste: Ihre letzte Station
 * dort entfällt nie. Fehlt sie, hat die Tour die Seite selbst geöffnet, weil
 * sie die Seitenleiste nicht fand.
 */
function expectSidebarPath(
  tour: SetupTour,
  visited: readonly string[],
  mobile: boolean,
): void {
  const lastNav = tour.stops.filter((stop) => stop.nav).at(-1);
  if (!mobile && lastNav) expect(visited).toContain(lastNav.title);
}

/**
 * Geht die laufende Tour durch, bis sie endet oder eine Station mit
 * Nebenwirkung erreicht. Gibt die Titel der besuchten Stationen zurück.
 */
async function walkTour(
  page: Page,
  tour: SetupTour,
): Promise<{ visited: string[]; stoppedAt: SetupTourStop | null }> {
  const visited: string[] = [];
  const bubble = tourBubble(page);
  for (let guard = 0; guard < 40; guard++) {
    await expect(bubble, "Die Tour zeigt keine Sprechblase.").toBeVisible({
      timeout: 30_000,
    });
    const position = await tourPosition(page);
    if (!position) throw new Error("Die Sprechblase zeigt keine Station.");
    const stop = stopAt(tour, position);
    const label = `Station ${position.index + 1} von ${position.total} „${stop.title}“`;
    visited.push(stop.title);

    await expect(bubble, label).toHaveAttribute("aria-label", stop.title);
    await expect(
      page.locator("div.ring-2.ring-moto-green"),
      `${label}: kein grüner Rahmen, die Stelle ${stop.target} fehlt.`,
    ).toBeVisible();
    await expect(bubble, label).not.toContainText(TARGET_MISSING);
    if (stop.missingText) {
      await expect(bubble, label).not.toContainText(stop.missingText);
    }
    // Die Tour scrollt die Stelle weich ins Bild, und der Rahmen folgt ihr.
    // Erst klicken, wenn er still steht, wie ein Mensch: Ein Menü
    // (OverflowMenu) schließt sich beim Scrollen gleich wieder.
    let previousBox = "";
    await expect
      .poll(
        async () => {
          const box = JSON.stringify(
            await page.locator("div.ring-2.ring-moto-green").boundingBox(),
          );
          const still = box === previousBox;
          previousBox = box;
          return still;
        },
        {
          message: `${label}: Der Rahmen kommt nicht zur Ruhe.`,
          intervals: [250],
        },
      )
      .toBe(true);
    // Eine Station darf von selbst weitergehen, etwa „Eltern eintragen“,
    // sobald die geladene Liste schon jemanden mit Menü zeigt
    // (skipWhenVisible). Dann gilt die nächste Station.
    const here = JSON.stringify(position);
    let moved = false;
    await expect
      .poll(
        async () => {
          moved = JSON.stringify(await tourPosition(page)) !== here;
          return moved || (await markHighlightedTarget(page, stop.target));
        },
        {
          message: `${label}: Der Rahmen liegt auf keiner Stelle zu ${stop.target}.`,
          timeout: 10_000,
        },
      )
      .toBe(true);
    if (moved) {
      // Die Station ist entfallen, nicht besucht.
      visited.pop();
      if ((await tourPosition(page)) === null) {
        annotateStations(visited);
        return { visited, stoppedAt: null };
      }
      continue;
    }
    // Rendert die Seite die Stelle neu, geht die Markierung mit dem alten
    // Element verloren; dann die neue markieren.
    await expect(async () => {
      expect(await markHighlightedTarget(page, stop.target)).toBe(true);
      await expect(page.locator("[data-e2e-tour-target]")).toBeVisible({
        timeout: 1_000,
      });
    }, `${label}: Die markierte Stelle ist nicht zu sehen.`).toPass({
      timeout: 10_000,
    });

    if (SIDE_EFFECT_TARGETS.has(stop.target)) {
      annotateStations(visited);
      return { visited, stoppedAt: stop };
    }
    if (stop.advance === "click") {
      // Lädt die Seite nach, ersetzt sie die Stelle (etwa die Reiter der
      // Kindakte); dann die neue markieren, wie die Tour sie neu findet.
      // Liegt die Stelle unter etwas Festem (auf dem Handy die untere
      // Leiste), scrollt die Person sie erst frei und tippt dann. Scrollte
      // erst der Klick sie frei, schlösse das Scrollen ein eben geöffnetes
      // Menü gleich wieder.
      let navHref: string | null = null;
      await expect(async () => {
        expect(await markHighlightedTarget(page, stop.target)).toBe(true);
        const free = await page
          .locator("[data-e2e-tour-target]")
          .evaluate((element) => {
            const rect = element.getBoundingClientRect();
            const hit = document.elementFromPoint(
              rect.left + rect.width / 2,
              rect.top + rect.height / 2,
            );
            if (hit && element.contains(hit)) return true;
            element.scrollIntoView({ block: "center" });
            return false;
          });
        expect(free, "Die Stelle liegt unter etwas anderem.").toBe(true);
        navHref = await page
          .locator("a[data-e2e-tour-target]")
          .getAttribute("href", { timeout: 500 })
          .catch(() => null);
        await page.locator("[data-e2e-tour-target]").click({ timeout: 2_000 });
      }, `${label}: Die markierte Stelle lässt sich nicht anklicken.`).toPass({
        timeout: 30_000,
      });
      if (stop.nav && navHref) await expectNavigation(page, label, navHref);
    } else {
      await bubble.getByRole("button", { name: /^(Weiter|Fertig)$/ }).click();
    }
    // Weiter heißt: eine andere Station, ein Zweig oder das Ende der Tour.
    await expect
      .poll(async () => JSON.stringify(await tourPosition(page)), {
        message: `${label}: Die Tour geht nicht weiter.`,
        timeout: 30_000,
      })
      .not.toBe(JSON.stringify(position));
    if ((await tourPosition(page)) === null) {
      annotateStations(visited);
      return { visited, stoppedAt: null };
    }
  }
  throw new Error(`Die Tour endet nicht: ${visited.join(" → ")}`);
}

/**
 * Ein Link in der Navigation muss zur Seite führen. Im Dev-Server geht ein
 * Seitenwechsel verloren, wenn gleichzeitig Fast Refresh neu baut; dann
 * tippt die Person einmal nach, wie ein Mensch. Führt auch das nicht zur
 * Seite, ist der Link kaputt.
 */
async function expectNavigation(
  page: Page,
  label: string,
  href: string,
): Promise<void> {
  const target = new URL(href, page.url()).pathname;
  const link = page.locator(`a[href="${href}"]:visible`).first();
  const arrived = () => new URL(page.url()).pathname === target;
  const waitForArrival = () =>
    page
      .waitForURL((url) => url.pathname === target, { timeout: 10_000 })
      .catch(() => undefined);
  await waitForArrival();
  if (!arrived() && (await link.isVisible())) {
    await link.click({ timeout: 2_000 });
    await waitForArrival();
  }
  expect(arrived(), `${label}: Der Link führt nicht zu ${target}.`).toBe(true);
}

/** Klappt die Checkliste auf, den Schritt auf und startet seine Tour. */
async function startTour(page: Page, stepTitle: string): Promise<void> {
  const checklist = page.getByRole("region", {
    name: "Erste Schritte mit moto",
  });
  await expect(async () => {
    if (!(await checklist.isVisible())) {
      await page
        .getByRole("button", { name: /^Erste Schritte öffnen/ })
        .click({ timeout: 2_000 });
    }
    await expect(checklist).toBeVisible({ timeout: 2_000 });
  }).toPass({ timeout: 30_000 });
  const stepButton = checklist.getByRole("button", { name: stepTitle });
  if ((await stepButton.getAttribute("aria-expanded")) !== "true") {
    await stepButton.click();
  }
  const panel = checklist.locator(
    `#${await stepButton.getAttribute("aria-controls")}`,
  );
  await panel
    .getByRole("button", { name: /^(Zeig es mir|Noch einmal zeigen)$/ })
    .click();
}

/** Schließt eine Tour, die an einer Station mit Nebenwirkung steht. */
async function closeTour(page: Page): Promise<void> {
  await tourBubble(page).getByRole("button", { name: "Tour beenden" }).click();
  await expect(tourBubble(page)).toHaveCount(0);
  await expect(
    page.getByRole("region", { name: "Erste Schritte mit moto" }),
  ).toBeVisible();
}

/** Computer und Handy: Auf dem Handy führt die Tour über die untere Leiste. */
const VIEWPORTS = [
  {
    name: "Computer",
    mobile: false,
    use: { viewport: { width: 1280, height: 720 } },
  },
  {
    name: "Handy",
    mobile: true,
    use: {
      viewport: { width: 393, height: 852 },
      isMobile: true,
      hasTouch: true,
    },
  },
] as const;

for (const device of VIEWPORTS) {
  test.describe(`Einrichtungs-Touren (${device.name})`, () => {
    test.use(device.use);

    test.beforeEach(async ({ page }) => {
      test.skip(setupAccess === null, MISSING_SEED_ACCESS);
      if (!setupAccess) return;
      test.setTimeout(240_000);
      await loginToTenant(page, setupAccess, setupBase);
      // Auf der Startseite: Die Touren beginnen in der Navigation.
      await page.goto(`${setupBase}/dashboard`, {
        waitUntil: "domcontentloaded",
      });
    });

    test("Team: über „+ Personal“ bis „Einladung senden“", async ({ page }) => {
      const tour = SETUP_TOURS.team!;
      await startTour(page, SCHOOL_SETUP_STEP_CONTENT.team.title);
      const { visited, stoppedAt } = await walkTour(page, tour);
      expect(stoppedAt?.title).toBe("Einladung senden");
      expectSidebarPath(tour, visited, device.mobile);
      // Die Schule darf auch Externe anlegen: „+ Personal“ klappt auf, und
      // die Tour führt über den Eintrag „Personal einladen“ (#4573, #5142).
      expect(visited).toEqual(
        expect.arrayContaining([
          "Personal einladen",
          "Einladung wählen",
          "Angaben eintragen",
        ]),
      );
      await closeTour(page);
    });

    test("Eltern: von der Kinderliste zur Einladung beim Kind", async ({
      page,
    }) => {
      const tour = SETUP_TOURS.guardians!;
      await startTour(page, SCHOOL_SETUP_STEP_CONTENT.guardians.title);
      const { visited, stoppedAt } = await walkTour(page, tour);
      // Steht beim Kind schon jemand mit E-Mail-Adresse, endet die Tour am
      // Menü „Einladen“, sonst am Speichern der neuen Person.
      expect(["Einladen", "Speichern"]).toContain(stoppedAt?.title);
      expectSidebarPath(tour, visited, device.mobile);
      expect(visited).toEqual(
        expect.arrayContaining(["Kind öffnen", "Erziehungsberechtigte"]),
      );
      await closeTour(page);
    });
  });

  test.describe(`Touren der ersten Schritte im Team (${device.name})`, () => {
    test.use(device.use);

    const tours = staffOnboardingTours(false);

    test.beforeEach(async ({ page }) => {
      test.skip(staffAccess === null, MISSING_SEED_ACCESS);
      if (!staffAccess) return;
      test.setTimeout(240_000);
      await loginToTenant(page, staffAccess);
      // Jeder Lauf beginnt mit offener Checkliste; eine beendete Tour hakt
      // ihren Schritt ab.
      // Nacheinander: Jede Route liest den ganzen Stand und schreibt ihn
      // zurück. Parallel überschriebe ein Schritt das Einblenden wieder.
      const reset = [
        () =>
          page.request.put(`${tenantBase}/api/staff-onboarding/dismissal`, {
            data: { dismissed: false },
          }),
        ...STAFF_ONBOARDING_STEP_KEYS.map(
          (step) => () =>
            page.request.put(
              `${tenantBase}/api/staff-onboarding/steps/${step}`,
              { data: { state: "open" } },
            ),
        ),
      ];
      for (const send of reset) {
        const response = await send();
        expect(response.ok(), await response.text()).toBe(true);
      }
      await page.goto(`${tenantBase}/dashboard`, {
        waitUntil: "domcontentloaded",
      });
    });

    // Die Betreuungskraft hat eine eigene Gruppe, sucht Kinder, meldet sie
    // im Web an und ab, sieht ihren Kalender, beaufsichtigt Räume und erfasst
    // Arbeitszeit: Jeder Schritt gilt für sie, und startTour findet ihn.
    for (const step of STAFF_ONBOARDING_STEP_KEYS) {
      const { title } = STAFF_ONBOARDING_STEP_CONTENT[step];
      test(`${title}: Tour bis zum Ende`, async ({ page }) => {
        await startTour(page, title);
        const { visited, stoppedAt } = await walkTour(page, tours[step]);
        expect(stoppedAt, visited.join(" → ")).toBeNull();
        // Die Betreuungskraft sieht jede Station auf den Seiten der Tour. Eine
        // optionale Station (etwa ein Reiter der Kindakte), deren Stelle
        // fehlt, entfiele sonst still.
        expect(visited).toEqual(
          expect.arrayContaining(
            tours[step].stops
              .filter((stop) => !stop.nav)
              .map((stop) => stop.title),
          ),
        );
        expectSidebarPath(tours[step], visited, device.mobile);
        // Wer die Tour bis zum Ende sieht, hat den Schritt erledigt.
        const checklist = page.getByRole("region", {
          name: "Erste Schritte mit moto",
        });
        await expect(
          checklist.getByRole("button", { name: `${title} Erledigt` }),
        ).toBeVisible({ timeout: 15_000 });
      });
    }
  });
}
