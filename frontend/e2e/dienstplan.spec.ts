import {
  expect,
  type APIRequestContext,
  type Page,
  test,
} from "@playwright/test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

import {
  berlinTodayISO,
  parseISODate,
  toISODate,
} from "../src/lib/date-helpers";
import { getWeekNumber } from "../src/lib/time-tracking-helpers";

// Chunk 9 des Planung-Redesigns Inkrement 3
// (docs/planung-redesign/docs/05-dienstplan.md Abschnitt 12): der neue
// /dienstplan-Bereich (ResourceGrid-Wochenraster, Halbjahres-Sicht,
// Verschieben-nach, URL-State d/view). Diese Spec prüft die UI-Verdrahtung des
// Bereichs gegen den lokalen Stack, nicht die Schicht-/Serien-Backend-Semantik
// (die decken die Go-Tests in backend/api/staff-shifts ab, die Detached-/
// Delta-Logik die Vitest-Tests dienstplan-resource-grid.test.tsx u. a.).
//
// Muster: exakt wie vertretung-flow.spec.ts. Testdaten werden über die
// Next.js-Proxy-Routen (/api/staff/shifts …) angelegt und wieder entfernt; die
// Requests laufen über den Browser-Context, teilen also die NextAuth-Session
// des UI-Logins. Die Dev-DB gehört dem Nutzer: jeder Test räumt vor UND nach
// dem Lauf die von ihm berührten Schichten wieder ab (idempotent, wiederholbar).
//
// Login/Tenant-Herkunft wie bei den Nachbar-Specs: Slug + Admin-Zugang aus
// backend/.seed-state.json (regeneriert bei jedem Seed); E2E_TENANT_SLUG /
// E2E_TEST_EMAIL / E2E_TEST_PASSWORD überschreiben sie. Ohne verwertbare Werte
// überspringt die Spec.
//
// Der lokale Stack bedient die Tenant-App nur unter {slug}.localhost:3000 (bare
// localhost ist die Marketing-Seite); die geschützten Routen rendern ohne
// Session eine leere Hülle statt auf eine Login-Maske umzuleiten. Die
// Login-Maske liegt auf der Tenant-Wurzel `/`. SSE hält das Netzwerk dauerhaft
// aktiv, deshalb NIE auf `load`/`networkidle` warten, sondern auf konkrete
// Selektoren mit `domcontentloaded`.

interface SeedAccess {
  slug: string;
  email: string;
  password: string;
}

function loadAccess(): SeedAccess | null {
  const envSlug = process.env.E2E_TENANT_SLUG;
  const envEmail = process.env.E2E_TEST_EMAIL;
  const envPassword = process.env.E2E_TEST_PASSWORD;
  let slug = envSlug;
  let email = envEmail;
  let password = envPassword;
  try {
    const raw = readFileSync(
      join(process.cwd(), "..", "backend", ".seed-state.json"),
      "utf8",
    );
    const seed = JSON.parse(raw) as {
      bootstrap?: { tenant_slug?: string };
      accounts?: { admin?: Array<{ email?: string; password?: string }> };
    };
    const admin = seed.accounts?.admin?.[0];
    slug = slug ?? seed.bootstrap?.tenant_slug;
    email = email ?? admin?.email;
    password = password ?? admin?.password;
  } catch {
    // Keine Seed-Datei (z. B. CI ohne lokalen Stack): nur Env-Werte zählen.
  }
  if (slug && email && password) return { slug, email, password };
  return null;
}

const access = loadAccess();
const base = access ? `http://${access.slug}.localhost:3000` : "";
const TODAY = berlinTodayISO();

// Großzügiges Budget für Zusicherungen hinter einem SWR-Refetch gegen den
// Next-Dev-Server (Grid nach Save, Halbjahres-Spalten).
const DATA = { timeout: 20000 } as const;
const GRID = { timeout: 30000 } as const;

// ─── kleine reine Datums-Helfer (kein toISOString) ──────────────────────────
function mondayOf(iso: string): string {
  const d = parseISODate(iso);
  const offset = (d.getDay() + 6) % 7; // Mo = 0
  d.setDate(d.getDate() - offset);
  return toISODate(d);
}
function addDaysISO(iso: string, days: number): string {
  const d = parseISODate(iso);
  d.setDate(d.getDate() + days);
  return toISODate(d);
}
function escapeRegExp(value: string): string {
  return value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

// ─── Proxy-Helfer (teilen die UI-Session-Cookies) ───────────────────────────
interface StaffLite {
  id: number;
  first_name: string;
  last_name: string;
}

async function fetchNonAdminStaff(
  request: APIRequestContext,
): Promise<StaffLite[]> {
  const from = mondayOf(TODAY);
  const to = addDaysISO(from, 4);
  const res = await request.get(
    `${base}/api/staff/shifts/overview?from=${from}&to=${to}`,
  );
  expect(res.ok(), `overview laden fehlgeschlagen (${res.status()})`).toBe(
    true,
  );
  const json = (await res.json()) as { data?: { staff?: StaffLite[] } };
  const staff = json.data?.staff ?? [];
  // Die beiden Seed-Admins (id 1/2) nicht einplanen.
  return staff.filter((s) => s.id !== 1 && s.id !== 2);
}

async function createShift(
  request: APIRequestContext,
  input: {
    staffId: number;
    date: string;
    start: string;
    end: string;
    breakMinutes?: number;
  },
): Promise<number> {
  const res = await request.post(`${base}/api/staff/shifts`, {
    data: {
      staff_id: input.staffId,
      date: input.date,
      start_time: input.start,
      end_time: input.end,
      break_minutes: input.breakMinutes ?? 0,
    },
  });
  expect(res.ok(), `Schicht-Anlage fehlgeschlagen (${res.status()})`).toBe(
    true,
  );
  const json = (await res.json()) as { data: { id: number } };
  return json.data.id;
}

// Fegt ALLE Schichten einer Person im Zeitraum weg — unabhängig davon, ob sie
// per API oder über die UI entstanden sind. Als Vorbedingung UND im finally
// aufgerufen, damit ein abgebrochener Lauf keine Waisen hinterlässt.
async function clearShifts(
  request: APIRequestContext,
  staffId: number,
  from: string,
  to: string,
): Promise<void> {
  const res = await request.get(
    `${base}/api/staff/shifts?staff_id=${staffId}&from=${from}&to=${to}`,
  );
  if (!res.ok()) return;
  const json = (await res.json()) as { data?: Array<{ id: number }> };
  for (const row of json.data ?? []) {
    await request
      .delete(`${base}/api/staff/shifts/${row.id}`)
      .catch(() => undefined);
  }
}

// Nimmt eine Krankmeldung im Zeitraum zurück (der Backend-DELETE dreht die
// Kaskade zurück und aktiviert die stornierte Schicht wieder).
async function clearSickAbsences(
  request: APIRequestContext,
  staffId: number,
  from: string,
  to: string,
): Promise<void> {
  const res = await request.get(
    `${base}/api/staff/${staffId}/absences?from=${from}&to=${to}`,
  );
  if (!res.ok()) return;
  const json = (await res.json()) as { data?: Array<{ id: number }> };
  for (const row of json.data ?? []) {
    await request
      .delete(`${base}/api/staff/${staffId}/absences/${row.id}`)
      .catch(() => undefined);
  }
}

async function login(page: Page, email: string, password: string) {
  await page.goto(`${base}/`, { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[type="email"]', { timeout: 20000 });
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', password);
  await page.click('button:has-text("Anmelden")');
  await page.waitForSelector('input[type="email"]', {
    state: "detached",
    timeout: 20000,
  });
}

async function openWeek(page: Page, day: string) {
  await page.goto(`${base}/dienstplan?d=${day}&view=woche`, {
    waitUntil: "domcontentloaded",
  });
  await expect(
    page.getByRole("region", { name: "Dienstplan-Wochenansicht" }),
  ).toBeVisible(GRID);
}

// Empty-/Plus-Zelle einer Person an einem Tag (Grid und gefüllte Zelle tragen
// dieselbe Beschriftung — pro Zelle existiert genau eine).
function emptyCellButton(page: Page, staff: StaffLite, date: string) {
  const [, m, d] = date.split("-");
  return page.getByRole("button", {
    name: new RegExp(
      `Schicht anlegen, ${escapeRegExp(staff.first_name)} ${escapeRegExp(
        staff.last_name,
      )}, \\w+ ${d}\\.${m}\\.`,
    ),
  });
}

// Eine ganze Grid-Zeile einer Person (für "enthält Block ja/nein").
function personRow(page: Page, staff: StaffLite) {
  return page
    .getByRole("row")
    .filter({ hasText: `${staff.last_name}, ${staff.first_name}` });
}

test.describe("Dienstplan UI-Flow (Inkrement 3, docs/05-dienstplan.md §12)", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(
      access === null,
      "backend/.seed-state.json (oder E2E_TENANT_SLUG/E2E_TEST_EMAIL/E2E_TEST_PASSWORD) erforderlich",
    );
    if (!access) return;
    await login(page, access.email, access.password);
  });

  // ── Flow 1: Navigation + Redirects ────────────────────────────────────────
  test("Navigation: /staff/dienstplan und /planung?tab=dienstplan landen auf /dienstplan", async ({
    page,
  }) => {
    if (!access) return;
    test.setTimeout(60000);

    // Der Sidebar-Eintrag öffnet /dienstplan.
    await page.goto(`${base}/dashboard`, { waitUntil: "domcontentloaded" });
    await page.locator("aside").getByText("Dienstplan").click();
    await page.waitForURL((url) => url.pathname.endsWith("/dienstplan"), {
      timeout: 15000,
    });

    // Alt-Einstieg /staff/dienstplan -> /dienstplan (ohne Query). Die Start-URL
    // endet selbst auf "/dienstplan", deshalb explizit auf das Verschwinden des
    // /staff-Präfixes warten.
    await page.goto(`${base}/staff/dienstplan`, {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(
      (url) =>
        url.pathname.endsWith("/dienstplan") &&
        !url.pathname.includes("/staff/"),
      { timeout: 15000 },
    );

    // Alt-Tab-Seite /planung?tab=dienstplan -> /dienstplan.
    await page.goto(`${base}/planung?tab=dienstplan`, {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL(
      (url) => url.pathname.endsWith("/dienstplan") && url.search === "",
      { timeout: 15000 },
    );
  });

  // ── Flow 2: Deep-Link KW-Label + Wochen-Navigation ───────────────────────
  test("Deep-Link zeigt die erwartete KW-Beschriftung; Wochen-Pfeil schreibt d", async ({
    page,
  }) => {
    if (!access) return;
    test.setTimeout(60000);

    // Feste, weit in der Zukunft liegende Woche (kollidiert mit keinen Testdaten).
    const monday = "2026-08-17"; // Montag, KW 34
    const kw = getWeekNumber(parseISODate(monday));
    await openWeek(page, monday);

    // "KW {n}: …" ist die Wochenraster-Beschriftung.
    await expect(page.getByText(new RegExp(`KW ${kw}:`))).toBeVisible(DATA);

    // Wochen-Pfeil schreibt den Montag der Zielwoche nach d (via history-Replace;
    // der Bereich re-rendert ohne Server-Roundtrip).
    await page.getByRole("button", { name: "Nächste Woche" }).click();
    const nextMonday = addDaysISO(monday, 7);
    await expect
      .poll(() => new URL(page.url()).searchParams.get("d"))
      .toBe(nextMonday);
    await expect(
      page.getByText(
        new RegExp(`KW ${getWeekNumber(parseISODate(nextMonday))}:`),
      ),
    ).toBeVisible(DATA);
  });

  // ── Flow 3: Schicht über leere Zelle anlegen + 409-Überschneidung ────────
  test("Schicht über leere Zelle anlegen; zweite überlappende Schicht zeigt Überschneidungs-Fehler", async ({
    page,
  }) => {
    if (!access) return;
    test.setTimeout(90000);
    const staff = await fetchNonAdminStaff(page.request);
    const person = staff[0]!;
    const day = mondayOf(addDaysISO(TODAY, 7)); // Montag nächster Woche

    await clearShifts(page.request, person.id, day, day);
    try {
      await openWeek(page, day);
      const dialog = page.getByRole("dialog");

      // Leere Zelle -> Editor im create-Modus.
      await emptyCellButton(page, person, day).first().click();
      await expect(dialog).toBeVisible();
      await dialog.getByLabel("Beginn", { exact: true }).fill("08:00");
      await dialog.getByLabel("Ende", { exact: true }).fill("10:00");
      await dialog.getByRole("button", { name: "Schicht anlegen" }).click();
      await expect(dialog).toBeHidden(DATA);

      // Block erscheint.
      await expect(
        personRow(page, person).getByText(/08:00.*10:00/),
      ).toBeVisible(DATA);

      // 409-Provokation: zweite, überlappende Schicht in derselben Zelle.
      await emptyCellButton(page, person, day).first().click();
      await expect(dialog).toBeVisible();
      await dialog.getByLabel("Beginn", { exact: true }).fill("09:00");
      await dialog.getByLabel("Ende", { exact: true }).fill("11:00");
      await dialog.getByRole("button", { name: "Schicht anlegen" }).click();

      // Fehlermeldung erscheint, Editor bleibt offen.
      await expect(
        dialog.getByText(
          "Diese Schicht überschneidet sich mit einer bestehenden Schicht.",
        ),
      ).toBeVisible(DATA);
      await expect(dialog).toBeVisible();
    } finally {
      await clearShifts(page.request, person.id, day, day);
    }
  });

  // ── Flow 4: Serie mit Woche A + "Nur diese Woche" (Detached-Icon) ────────
  // Erfordert einen aktiven Kalenderzeitraum mit weekCycleLength > 1. Der Seed
  // liefert nur einen 1-Wochen-Zyklus; die öffentliche API kann einen selbst
  // angelegten Zeitraum nicht wieder löschen, sobald eine Serie ihn referenziert
  // (FK fk_staff_shift_series_calendar_period; EndSeries kappt nur, löscht die
  // Serien-Wurzel nicht). Ein selbst-aufräumender E2E darf daher keinen
  // Zyklus-Zeitraum anlegen. Existiert bereits einer, läuft der Test; sonst
  // überspringt er mit klarer Begründung (siehe Bericht — der Detached-Zustand
  // ist zusätzlich in dienstplan-resource-grid.test.tsx unit-getestet).
  test("Serie (Woche A) anlegen, dann 'Nur diese Woche' -> gelbes Detached-Icon", async ({
    page,
  }) => {
    if (!access) return;
    test.setTimeout(120000);

    const periodsRes = await page.request.get(`${base}/api/timetable/periods`);
    const periodsJson = (await periodsRes.json()) as {
      data?: Array<{
        id: number | string;
        start_date: string;
        end_date: string;
        week_cycle_length: number;
        is_active: boolean;
      }>;
    };
    const cyclePeriod = (periodsJson.data ?? []).find(
      (p) => p.is_active && p.week_cycle_length > 1,
    );
    test.skip(
      cyclePeriod === undefined,
      "Kein aktiver Kalenderzeitraum mit weekCycleLength > 1 vorhanden (Seed liefert keinen; die API kann keinen selbst-aufräumend anlegen). Detached-Zustand ist unit-getestet.",
    );
    if (!cyclePeriod) return;

    const staff = await fetchNonAdminStaff(page.request);
    const person = staff[4]!;
    // Erster Montag des Zyklus-Zeitraums, der in der Zukunft liegt (ab morgen
    // materialisiert die Serie).
    let seriesStart = mondayOf(cyclePeriod.start_date);
    while (seriesStart <= TODAY) seriesStart = addDaysISO(seriesStart, 7);
    const rangeEnd = cyclePeriod.end_date;

    await clearShifts(page.request, person.id, seriesStart, rangeEnd);
    let seriesId: number | null = null;
    try {
      await openWeek(page, seriesStart);
      const dialog = page.getByRole("dialog");

      await emptyCellButton(page, person, seriesStart).first().click();
      await expect(dialog).toBeVisible();
      await dialog.getByLabel("Beginn", { exact: true }).fill("09:00");
      await dialog.getByLabel("Ende", { exact: true }).fill("12:00");
      // Serien-Abschnitt (der angeklickte Wochentag ist bereits vorausgewählt).
      await dialog.getByText("Als Serie wiederholen").click();
      await dialog.getByRole("radio", { name: "Alle 2 Wochen" }).check();
      await dialog.getByRole("radio", { name: "Woche A" }).check();
      await dialog.getByRole("button", { name: "Serie anlegen" }).click();
      // Serien-Notiz + Schließen.
      await expect(
        dialog.getByText("Die Serie wurde gespeichert."),
      ).toBeVisible(DATA);
      await dialog.getByRole("button", { name: "Schließen" }).click();
      await expect(dialog).toBeHidden(DATA);

      // Materialisierte A-Wochen ermitteln (jede zweite Woche ab seriesStart).
      const listRes = await page.request.get(
        `${base}/api/staff/shifts?staff_id=${person.id}&from=${seriesStart}&to=${rangeEnd}`,
      );
      const listJson = (await listRes.json()) as {
        data?: Array<{ date: string; series_id: number | null }>;
      };
      const seriesShifts = (listJson.data ?? []).filter(
        (s) => s.series_id != null,
      );
      seriesId = seriesShifts[0]?.series_id ?? null;
      expect(
        seriesShifts.length,
        "Serie muss mehrere A-Wochen materialisieren",
      ).toBeGreaterThanOrEqual(2);
      const editWeek = mondayOf(seriesShifts[0]!.date);
      const keepWeek = mondayOf(seriesShifts[1]!.date);

      // Serien-Block der ersten A-Woche: graues Wiederhol-Symbol.
      await openWeek(page, editWeek);
      const editBlock = personRow(page, person).getByRole("button", {
        name: /09:00.*12:00/,
      });
      await expect(editBlock).toBeVisible(DATA);
      await expect(
        personRow(page, person).getByLabel("Teil einer Serie"),
      ).toBeVisible(DATA);

      // Bearbeiten -> "Nur diese Woche".
      await editBlock.click();
      await expect(dialog).toBeVisible();
      await dialog
        .getByRole("button", { name: "Änderungen speichern" })
        .click();
      await page.getByRole("button", { name: "Nur diese Woche" }).click();
      await expect(dialog).toBeHidden(DATA);

      // Diese Woche: gelbes Detached-Icon (aria "Serie, für diese Woche angepasst").
      await expect(
        personRow(page, person).getByLabel("Serie, für diese Woche angepasst"),
      ).toBeVisible(DATA);

      // Die übrigen Serien-Blöcke bleiben unverändert (grau).
      await openWeek(page, keepWeek);
      await expect(
        personRow(page, person).getByLabel("Teil einer Serie"),
      ).toBeVisible(DATA);
    } finally {
      // Serie beenden (stoppt künftige Materialisierung) und alle Schichten der
      // Person im Zeitraum entfernen. Die Serien-Wurzel bleibt (kein Hard-Delete
      // in der API) — sie referenziert einen bereits bestehenden Zeitraum.
      if (seriesId != null) {
        await page.request
          .delete(
            `${base}/api/staff/shifts/series/${seriesId}?from=${seriesStart}`,
          )
          .catch(() => undefined);
      }
      await clearShifts(page.request, person.id, seriesStart, rangeEnd);
    }
  });

  // ── Flow 5: Verschieben nach einer anderen Person ────────────────────────
  test("Verschieben nach: Schicht wandert zur Zielperson (Quelle leer, Ziel gefüllt)", async ({
    page,
  }) => {
    if (!access) return;
    test.setTimeout(90000);
    const staff = await fetchNonAdminStaff(page.request);
    const source = staff[1]!;
    const target = staff[2]!;
    const day = mondayOf(addDaysISO(TODAY, 14)); // eigene Woche

    await clearShifts(page.request, source.id, day, day);
    await clearShifts(page.request, target.id, day, day);
    try {
      await createShift(page.request, {
        staffId: source.id,
        date: day,
        start: "12:00",
        end: "16:00",
      });
      await openWeek(page, day);

      // Block-Kontextmenü -> Verschieben nach.
      await personRow(page, source)
        .getByRole("button", { name: /Aktionen zur Schicht 12:00/ })
        .click();
      await page.getByRole("menuitem", { name: "Verschieben nach" }).click();

      const dialog = page.getByRole("dialog");
      await expect(
        dialog.getByRole("heading", { name: "Schicht verschieben" }),
      ).toBeVisible();

      // Zielperson umstellen.
      await dialog.getByRole("combobox", { name: "Zielperson" }).click();
      await page
        .getByRole("option", {
          name: `${target.last_name}, ${target.first_name}`,
        })
        .click();
      await dialog.getByRole("button", { name: "Verschieben" }).click();

      // Bestätigung.
      await expect(
        page.getByRole("heading", { name: "Verschieben bestätigen" }),
      ).toBeVisible();
      await page.getByRole("button", { name: "Verschieben" }).click();

      // Ziel gefüllt, Quelle leer.
      await expect(
        personRow(page, target).getByText(/12:00.*16:00/),
      ).toBeVisible(DATA);
      await expect(
        personRow(page, source).getByText(/12:00.*16:00/),
      ).toHaveCount(0);
    } finally {
      await clearShifts(page.request, source.id, day, day);
      await clearShifts(page.request, target.id, day, day);
    }
  });

  // ── Flow 6: Halbjahres-Sicht — Zellklick springt in die Woche ────────────
  test("Halbjahr: Zellklick springt in die richtige Woche (d gesetzt, view entfernt)", async ({
    page,
  }) => {
    if (!access) return;
    test.setTimeout(90000);
    const staff = await fetchNonAdminStaff(page.request);
    const person = staff[3]!;
    // Woche innerhalb des laufenden Planungszeitraums (Seed-Schuljahr deckt heute
    // ab), damit eine klickbare Zelle (plannedMinutes > 0) entsteht.
    const kwMonday = mondayOf(addDaysISO(TODAY, 7));
    const kw = getWeekNumber(parseISODate(kwMonday));

    await clearShifts(
      page.request,
      person.id,
      kwMonday,
      addDaysISO(kwMonday, 4),
    );
    try {
      await createShift(page.request, {
        staffId: person.id,
        date: kwMonday,
        start: "09:00",
        end: "13:00",
      });
      await openWeek(page, TODAY);

      // Auf Halbjahr umschalten.
      await page.getByRole("tab", { name: "Halbjahr" }).click();
      await expect(
        page.getByRole("region", { name: "Dienstplan-Halbjahresansicht" }),
      ).toBeVisible(GRID);

      // Zelle der Person in ihrer KW anklicken (die Zelle rendert, sobald die
      // Wochendaten geladen sind — großzügiges Budget für die Progressiv-Loader).
      const cell = page.getByRole("button", {
        name: `Woche ${kw} öffnen, ${person.first_name} ${person.last_name}`,
      });
      await cell.click({ timeout: 45000 });

      // Sprung in die Wochenansicht der angeklickten Woche.
      await expect
        .poll(() => new URL(page.url()).searchParams.get("d"))
        .toBe(kwMonday);
      expect(new URL(page.url()).searchParams.get("view")).toBeNull();
      await expect(
        page.getByRole("region", { name: "Dienstplan-Wochenansicht" }),
      ).toBeVisible(GRID);
    } finally {
      await clearShifts(
        page.request,
        person.id,
        kwMonday,
        addDaysISO(kwMonday, 4),
      );
    }
  });

  // ── Flow 7: Krank-Flow (Einstieg Vertretung + "Fällt aus") ───────────────
  test("Krank-Flow: 'Für heute abwesend melden' navigiert nach /vertretung; 'Krank melden' storniert die Schicht", async ({
    page,
  }) => {
    if (!access) return;
    // Flow 7a wechselt auf /vertretung — der erste Kompiliervorgang dieser Route
    // im Next-Dev-Server kann kalt spürbar dauern, daher großzügiges Budget.
    test.setTimeout(180000);
    // Hohes Viewport: das Personen-Menü (drei Einträge) klappt nach unten auf;
    // bei einer weit unten liegenden Person läge der zweite Eintrag ("Für heute
    // abwesend melden") sonst unter dem Fold und wäre nicht anklickbar.
    await page.setViewportSize({ width: 1280, height: 1200 });
    const staff = await fetchNonAdminStaff(page.request);
    const person = staff[5]!;
    // Schicht heute (die Krankmeldung storniert reguläre Schichten im Zeitraum).
    const from = mondayOf(TODAY);
    const to = addDaysISO(from, 4);

    await clearSickAbsences(page.request, person.id, TODAY, TODAY);
    await clearShifts(page.request, person.id, from, to);
    try {
      await createShift(page.request, {
        staffId: person.id,
        date: TODAY,
        start: "12:00",
        end: "16:00",
      });
      await openWeek(page, TODAY);

      // (b) Krank melden -> SickReportModal -> speichern -> "Fällt aus".
      await personRow(page, person)
        .getByRole("button", {
          name: `Aktionen für ${person.first_name} ${person.last_name}`,
        })
        .click();
      await page.getByRole("menuitem", { name: "Krank melden" }).click();
      const sick = page.getByRole("dialog");
      await expect(
        sick.getByRole("heading", {
          name: `Krank melden: ${person.first_name} ${person.last_name}`,
        }),
      ).toBeVisible();
      await sick.getByRole("button", { name: "Krank melden" }).click();
      await expect(sick.getByText(/als krank gemeldet/)).toBeVisible(DATA);
      await sick
        .getByRole("button", { name: "Schließen", exact: true })
        .click();
      await expect(sick).toBeHidden(DATA);

      // Die Schicht der Person rendert als "Fällt aus".
      await expect(personRow(page, person).getByText(/Fällt aus/)).toBeVisible(
        DATA,
      );

      // (a) "Für heute abwesend melden" -> /vertretung?d=heute.
      await personRow(page, person)
        .getByRole("button", {
          name: `Aktionen für ${person.first_name} ${person.last_name}`,
        })
        .click();
      await page
        .getByRole("menuitem", { name: "Für heute abwesend melden" })
        .click();
      await page.waitForURL((url) => url.pathname.endsWith("/vertretung"), {
        timeout: 60000,
      });
      expect(new URL(page.url()).searchParams.get("d")).toBe(TODAY);
    } finally {
      // Krankmeldung zurücknehmen (dreht die Kaskade zurück) und Schicht löschen.
      await clearSickAbsences(page.request, person.id, TODAY, TODAY);
      await clearShifts(page.request, person.id, from, to);
    }
  });
});
