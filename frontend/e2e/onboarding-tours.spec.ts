import {
  expect,
  test,
  type Locator,
  type Page,
  type Request,
  type Response,
} from "@playwright/test";

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
import {
  mapStaffOnboardingState,
  STAFF_ONBOARDING_STEP_KEYS,
  type StaffOnboardingStepKey,
} from "../src/lib/staff-onboarding-api";
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
// Beim ersten Aufruf einer noch nicht besuchten Route kompiliert der
// Entwicklungsserver die Seite. Die Navigation selbst bleibt der Nachweis;
// nur das Eintreffen darf nicht schon nach zehn Sekunden als fehlend gelten.
const NAVIGATION_TIMEOUT = 30_000;
/**
 * Wie lange die Checkliste nach dem Ende einer Tour hat, um die Speicherung
 * zu bestätigen und den Schritt als erledigt zu zeigen. Antwort und Anzeige
 * teilen sich diese Zeit.
 */
const STEP_SAVED_TIMEOUT = 15_000;

/**
 * Seiten, die der Entwicklungsserver dieses Workers schon gebaut hat. Er baut
 * eine Seite und die Routen, die sie lädt, beim ersten Aufruf. Endete eine
 * Tour auf einer kalten Seite, wartete ihre Speicherung hinter diesem Bau
 * (#6339: 13,7 s Seitenwechsel, danach über 15 s keine Antwort).
 */
const warmedPages = new Set<string>();

/**
 * Ruft jede Seite der Touren einmal auf, bis sie keine Anfrage mehr
 * offen hat. So misst die Tour die App, nicht den ersten Bau einer Seite.
 */
async function warmTourPages(
  page: Page,
  tours: Readonly<Record<string, SetupTour>>,
): Promise<void> {
  const paths = [...new Set(Object.values(tours).map((tour) => tour.path))];
  const cold = paths.filter((path) => !warmedPages.has(path));
  if (cold.length === 0) return;
  const pending = new Set<Request>();
  let lastActivity = Date.now();
  // Der Live-Kanal (SSE) bleibt offen und zählt nicht.
  const counts = (request: Request) =>
    request.headers().accept !== "text/event-stream";
  const onRequest = (request: Request) => {
    if (!counts(request)) return;
    pending.add(request);
    lastActivity = Date.now();
  };
  const onDone = (request: Request) => {
    if (!counts(request)) return;
    pending.delete(request);
    lastActivity = Date.now();
  };
  page.on("request", onRequest);
  page.on("requestfinished", onDone);
  page.on("requestfailed", onDone);
  try {
    for (const path of cold) {
      await page.goto(`${tenantBase}${path}`, {
        waitUntil: "domcontentloaded",
        timeout: NAVIGATION_TIMEOUT,
      });
      // Die Checkliste (offen oder als Knopf) erscheint erst, wenn die Seite
      // geladen und ihr Stand gelesen ist.
      await expect(
        page
          .getByRole("region", { name: "Erste Schritte mit moto" })
          .or(page.getByRole("button", { name: /^Erste Schritte öffnen/ })),
      ).toBeVisible({ timeout: NAVIGATION_TIMEOUT });
      await expect
        .poll(() => pending.size === 0 && Date.now() - lastActivity >= 500, {
          message: `${path}: Die Seite lädt nicht zu Ende.`,
          intervals: [250],
          timeout: NAVIGATION_TIMEOUT,
        })
        .toBe(true);
      warmedPages.add(path);
    }
  } finally {
    page.off("request", onRequest);
    page.off("requestfinished", onDone);
    page.off("requestfailed", onDone);
  }
}

/** Sammelt die Antworten auf das Speichern eines Schritts. */
function recordStepSaves(page: Page, step: StaffOnboardingStepKey): Response[] {
  const saves: Response[] = [];
  page.on("response", (response) => {
    if (
      response.request().method() === "PUT" &&
      new URL(response.url()).pathname === `/api/staff-onboarding/steps/${step}`
    ) {
      saves.push(response);
    }
  });
  return saves;
}

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

/** Eine Station erst übernehmen, wenn Nummer und Titel zusammenpassen. */
async function currentTourStop(
  page: Page,
  tour: SetupTour,
): Promise<{
  position: { index: number; total: number };
  stop: SetupTourStop;
} | null> {
  const position = await tourPosition(page);
  if (!position) return null;
  const stop = stopAt(tour, position);
  const title = await tourBubble(page).getAttribute("aria-label");
  return title === stop.title ? { position, stop } : null;
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
    const station = await currentTourStop(page, tour);
    // Eine bedingte Station kann genau zwischen dem Fortschritt und ihrem
    // Titel zum nächsten Schritt springen. Dann die neue, vollständige
    // Station abwarten statt den alten Fortschritt zu prüfen.
    if (!station) {
      await page.waitForTimeout(100);
      continue;
    }
    const { position, stop } = station;
    const label = `Station ${position.index + 1} von ${position.total} „${stop.title}“`;
    visited.push(stop.title);

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
      .waitForURL((url) => url.pathname === target, {
        timeout: NAVIGATION_TIMEOUT,
      })
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
      // Vorgeladene Kinddaten dürfen den Reiter sofort bedienbar machen.
      // Sein UI-Zustand braucht keinen weiteren Server-Roundtrip, auf den
      // eine Tour oder eine Person bei kalter Route warten müsste.
      const tabNavigations: string[] = [];
      page.on("request", (request) => {
        const url = new URL(request.url());
        if (
          /\/students\/\d+$/.test(url.pathname) &&
          url.searchParams.has("tab") &&
          request.headers().rsc === "1"
        ) {
          tabNavigations.push(url.pathname + url.search);
        }
      });
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
      expect(tabNavigations).toEqual([]);
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
      await warmTourPages(page, tours);
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
        const saves = recordStepSaves(page, step);
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
        // Wer die Tour bis zum Ende sieht, hat den Schritt erledigt: Die
        // Speicherung antwortet mit dem erledigten Schritt, und die
        // Checkliste zeigt ihn. Beides in derselben Zeit; so sagt ein
        // Fehlschlag, ob die Antwort fehlt, scheiterte oder nicht ankam.
        const deadline = Date.now() + STEP_SAVED_TIMEOUT;
        await expect
          .poll(() => saves.length, {
            message: "Die Speicherung des Schritts antwortet nicht.",
            timeout: STEP_SAVED_TIMEOUT,
          })
          .toBe(1);
        const saved = saves[0]!;
        expect(saved.ok(), await saved.text()).toBe(true);
        const body = (await saved.json()) as { data?: object };
        expect(mapStaffOnboardingState(body.data ?? body).doneSteps).toContain(
          step,
        );
        const checklist = page.getByRole("region", {
          name: "Erste Schritte mit moto",
        });
        await expect(
          checklist.getByRole("button", { name: `${title} Erledigt` }),
        ).toBeVisible({ timeout: Math.max(deadline - Date.now(), 1) });
      });
    }
  });
}
