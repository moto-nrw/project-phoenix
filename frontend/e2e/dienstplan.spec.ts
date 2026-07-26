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
// Testdaten werden über die Next.js-Proxy-Routen (/api/staff/shifts …)
// angelegt und wieder entfernt; die Requests laufen über den Browser-Context,
// teilen also die NextAuth-Session des UI-Logins. Die Dev-DB gehört dem Nutzer:
// jeder Test merkt sich die IDs seiner eigenen Datensätze. finally + afterEach
// entfernen ausschließlich diese IDs; ein paralleler Nutzer oder Testlauf kann
// deshalb nie über einen Bereichsvergleich mit aufgeräumt werden.
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

let ownedShiftIds = new Set<number>();
let ownedAbsences = new Map<number, number>();

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
  ownedShiftIds.add(json.data.id);
  return json.data.id;
}

async function listShifts(
  request: APIRequestContext,
  staffId: number,
  from: string,
  to: string,
): Promise<Array<{ id: number }>> {
  const res = await request.get(
    `${base}/api/staff/shifts?staff_id=${staffId}&from=${from}&to=${to}`,
  );
  expect(res.ok(), `Schichten laden fehlgeschlagen (${res.status()})`).toBe(
    true,
  );
  const json = (await res.json()) as { data?: Array<{ id: number }> };
  return json.data ?? [];
}

async function listAbsences(
  request: APIRequestContext,
  staffId: number,
  from: string,
  to: string,
): Promise<Array<{ id: number }>> {
  const res = await request.get(
    `${base}/api/staff/${staffId}/absences?from=${from}&to=${to}`,
  );
  expect(res.ok(), `Abwesenheiten laden fehlgeschlagen (${res.status()})`).toBe(
    true,
  );
  const json = (await res.json()) as { data?: Array<{ id: number }> };
  return json.data ?? [];
}

async function findStaffWithEmptyRange(
  request: APIRequestContext,
  staff: readonly StaffLite[],
  from: string,
  to: string,
  count: number,
  options: { requireNoAbsence?: boolean } = {},
): Promise<StaffLite[]> {
  const available: StaffLite[] = [];
  for (const person of staff) {
    const shifts = await listShifts(request, person.id, from, to);
    const absences = options.requireNoAbsence
      ? await listAbsences(request, person.id, from, to)
      : [];
    if (shifts.length === 0 && absences.length === 0) {
      available.push(person);
      if (available.length === count) break;
    }
  }
  return available;
}

async function cleanupOwnedRecords(request: APIRequestContext): Promise<void> {
  // Abwesenheiten zuerst entfernen: der Backend-DELETE nimmt die
  // Krankmeldungs-Kaskade zurück, bevor die zugehörige Schicht gelöscht wird.
  for (const [absenceId, staffId] of ownedAbsences) {
    await request
      .delete(`${base}/api/staff/${staffId}/absences/${absenceId}`)
      .catch(() => undefined);
  }

  for (const shiftId of ownedShiftIds) {
    await request
      .delete(`${base}/api/staff/shifts/${shiftId}`)
      .catch(() => undefined);
  }
}

function resetCleanupRegistry(): void {
  ownedShiftIds = new Set<number>();
  ownedAbsences = new Map<number, number>();
}

async function captureCreatedShift(
  page: Page,
  action: () => Promise<void>,
): Promise<void> {
  const responsePromise = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return (
      response.request().method() === "POST" &&
      url.pathname.endsWith("/api/staff/shifts")
    );
  });
  await action();
  const response = await responsePromise;
  if (!response.ok()) return;
  const json = (await response.json()) as { data?: { id?: number } };
  if (json.data?.id != null) ownedShiftIds.add(json.data.id);
}

async function captureCreatedAbsence(
  page: Page,
  staffId: number,
  action: () => Promise<void>,
): Promise<void> {
  const responsePromise = page.waitForResponse((response) => {
    const url = new URL(response.url());
    return (
      response.request().method() === "POST" &&
      url.pathname.endsWith(`/api/staff/${staffId}/absences`)
    );
  });
  await action();
  const response = await responsePromise;
  if (!response.ok()) return;
  const json = (await response.json()) as { data?: { id?: number } };
  if (json.data?.id != null) ownedAbsences.set(json.data.id, staffId);
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
  // Diese Flows mutieren dieselbe lokale Dev-DB und lösen mehrere kalte
  // Next.js-Routen aus. Seriell bleiben die Ownership-Snapshots eindeutig und
  // der Dev-Server wird nicht mit sechs parallelen Kompiliervorgängen blockiert.
  test.describe.configure({ mode: "serial" });

  test.beforeEach(async ({ page }) => {
    test.skip(
      access === null,
      "backend/.seed-state.json (oder E2E_TENANT_SLUG/E2E_TEST_EMAIL/E2E_TEST_PASSWORD) erforderlich",
    );
    if (!access) return;
    await login(page, access.email, access.password);
  });

  test.afterEach(async ({ page }) => {
    // Netz für harte Test-Timeouts: afterEach erhält ein eigenes Budget. Das
    // in-Test-finally räumt den Normalfall auf; die Registry bleibt bis hierhin
    // erhalten, damit ein unterbrochenes finally erneut versucht werden kann.
    if (!access) return;
    try {
      await cleanupOwnedRecords(page.request);
    } finally {
      resetCleanupRegistry();
    }
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
    const day = mondayOf(addDaysISO(TODAY, 7)); // Montag nächster Woche
    const [person] = await findStaffWithEmptyRange(
      page.request,
      staff,
      day,
      day,
      1,
    );
    test.skip(
      person === undefined,
      "Keine Testperson ohne bestehende Schicht am Zieltag verfügbar",
    );
    if (!person) return;

    try {
      await openWeek(page, day);
      const dialog = page.getByRole("dialog");

      // Leere Zelle -> Editor im create-Modus.
      await emptyCellButton(page, person, day).first().click();
      await expect(dialog).toBeVisible();
      await dialog.getByLabel("Beginn", { exact: true }).fill("08:00");
      await dialog.getByLabel("Ende", { exact: true }).fill("10:00");
      await captureCreatedShift(page, () =>
        dialog.getByRole("button", { name: "Schicht anlegen" }).click(),
      );
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
      await cleanupOwnedRecords(page.request);
    }
  });

  // ── Flow 4: Verschieben nach einer anderen Person ────────────────────────
  test("Verschieben nach: Schicht wandert zur Zielperson (Quelle leer, Ziel gefüllt)", async ({
    page,
  }) => {
    if (!access) return;
    test.setTimeout(90000);
    const staff = await fetchNonAdminStaff(page.request);
    const day = mondayOf(addDaysISO(TODAY, 14)); // eigene Woche
    const [source, target] = await findStaffWithEmptyRange(
      page.request,
      staff,
      day,
      day,
      2,
    );
    test.skip(
      source === undefined || target === undefined,
      "Weniger als zwei Testpersonen ohne Schicht am Zieltag verfügbar",
    );
    if (!source || !target) return;

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
      await cleanupOwnedRecords(page.request);
    }
  });

  // ── Flow 5: Halbjahres-Sicht — Zellklick springt in die Woche ────────────
  test("Halbjahr: Zellklick springt in die richtige Woche (d gesetzt, view entfernt)", async ({
    page,
  }) => {
    if (!access) return;
    test.setTimeout(90000);
    const staff = await fetchNonAdminStaff(page.request);
    // Woche innerhalb des laufenden Planungszeitraums (Seed-Schuljahr deckt heute
    // ab), damit eine klickbare Zelle (plannedMinutes > 0) entsteht.
    const kwMonday = mondayOf(addDaysISO(TODAY, 7));
    const kwFriday = addDaysISO(kwMonday, 4);
    const kw = getWeekNumber(parseISODate(kwMonday));
    const [person] = await findStaffWithEmptyRange(
      page.request,
      staff,
      kwMonday,
      kwFriday,
      1,
    );
    test.skip(
      person === undefined,
      "Keine Testperson ohne Schichten in der Zielwoche verfügbar",
    );
    if (!person) return;

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
      await cleanupOwnedRecords(page.request);
    }
  });

  // ── Flow 6: Krank-Flow (Einstieg Vertretung + "Fällt aus") ───────────────
  test("Krank-Flow: 'Für heute abwesend melden' navigiert nach /vertretung; 'Krank melden' storniert die Schicht", async ({
    page,
  }) => {
    if (!access) return;
    // Beide Teilflüsse hängen am echten Heute: 6a navigiert nach
    // /vertretung?d=heute, 6b storniert die heutige Schicht per Krankmeldung
    // und erwartet den "Fällt aus"-Block im Wochenraster. Das Raster rendert
    // nur Mo-Fr — am Wochenende hat die heutige Schicht keine Spalte, der
    // Block kann nie erscheinen. Deshalb Skip statt Datums-Verbiegung.
    const todayWeekday = parseISODate(TODAY).getDay();
    test.skip(
      todayWeekday === 0 || todayWeekday === 6,
      "Krank-Flow braucht ein Heute innerhalb Mo-Fr (Wochenraster ohne Sa/So)",
    );
    // Flow 6a wechselt auf /vertretung — der erste Kompiliervorgang dieser Route
    // im Next-Dev-Server kann kalt spürbar dauern, daher großzügiges Budget.
    test.setTimeout(180000);
    // Hohes Viewport: das Personen-Menü (drei Einträge) klappt nach unten auf;
    // bei einer weit unten liegenden Person läge der zweite Eintrag ("Für heute
    // abwesend melden") sonst unter dem Fold und wäre nicht anklickbar.
    await page.setViewportSize({ width: 1280, height: 1200 });
    const staff = await fetchNonAdminStaff(page.request);
    // Schicht heute (die Krankmeldung storniert reguläre Schichten im Zeitraum).
    const from = mondayOf(TODAY);
    const to = addDaysISO(from, 4);
    const [person] = await findStaffWithEmptyRange(
      page.request,
      staff,
      from,
      to,
      1,
      { requireNoAbsence: true },
    );
    test.skip(
      person === undefined,
      "Keine Testperson ohne Schicht oder Abwesenheit in dieser Woche verfügbar",
    );
    if (!person) return;

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
      await captureCreatedAbsence(page, person.id, () =>
        sick.getByRole("button", { name: "Krank melden" }).click(),
      );
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
      await cleanupOwnedRecords(page.request);
    }
  });
});
