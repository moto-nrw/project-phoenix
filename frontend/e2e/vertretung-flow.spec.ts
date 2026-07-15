import { expect, type Page, test } from "@playwright/test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { berlinTodayISO } from "../src/lib/date-helpers";
import { nextWorkdayISO } from "../src/lib/timetable-helpers";

// Chunk 8 des Planung-Redesigns Inkrement 2
// (docs/planung-redesign/docs/07-vertretung.md Abschnitt 12/13): der Zweiteiler
// /vertretung. Diese Spec testet AUSSCHLIESSLICH die UI-Verdrahtung, nicht die
// Deviations-Backend-Semantik (die decken die Go-E2E-Flows
// flow_c_gaps_substitute_test.go / flow_h_replan_deviations_test.go ab).
//
// Testdaten sind vollständig selbst angelegt und wieder aufgeräumt: die Spec
// erzeugt über die API einen spontanen Termin mit genau einer geplanten Person,
// fährt den Flow (abwesend melden -> Ersatz wählen) und löscht den Termin im
// finally wieder (plus tagesweites "Anwesend melden" als Sicherheitsnetz). Die
// Dev-DB gehört dem Nutzer, daher kein Reset, kein Reseed, keine Fremd-Daten.
//
// Login und Tenant-Herkunft: die Nachbar-Specs melden sich über die
// Login-Maske an und ziehen Credentials aus der Umgebung. Der lokale Stack
// bedient die Tenant-App aber nur unter der Subdomain
// {slug}.localhost:3000 (bare localhost ist die Marketing-Seite), und die
// geschützten Routen rendern ohne Session eine leere Hülle statt auf eine
// Login-Maske umzuleiten. Die Login-Maske liegt auf der Tenant-Wurzel `/`.
// Slug und Admin-Zugang stehen in backend/.seed-state.json (regeneriert bei
// jedem Seed); E2E_TENANT_SLUG / E2E_TEST_EMAIL / E2E_TEST_PASSWORD
// überschreiben sie. Ohne verwertbare Werte überspringt die Spec.

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
    // Playwright läuft mit cwd = frontend/; die Seed-Datei liegt daneben.
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

const ALLOWED_PARAMS = new Set(["d", "block", "verlauf"]);
const DEVIATIONS_RE = /\/api\/timetable\/instances\/\d+\/deviations$/;
// Großzügiges Budget für Zusicherungen, die von einem SWR-Refetch gegen den
// Next-Dev-Server abhängen (Liste nach Save, Verlauf nach Deep-Link).
const DATA = { timeout: 15000 } as const;

// Aufräum-Anker: gesetzt, sobald der Termin angelegt ist. Das in-Test-finally
// räumt im Normalfall auf; afterEach ist das Netz für Timeout-Abbrüche (eigenes
// Zeitbudget), damit ein abgebrochener Lauf keine Waisen in der Dev-DB lässt.
// Playwright-Worker sind eigene Prozesse -> Modul-State ist pro Worker, kein
// Cross-Test-Rennen.
let cleanupInstanceId: string | null = null;
let cleanupStaffId: number | null = null;

/**
 * URL-Vokabular-Invariante (Akzeptanzkriterium 3): der Bereich trägt nie mehr
 * als `d`, `block`, `verlauf`. Der Filterzustand ("Nur Störungen | Ganzer Tag")
 * ist lokaler State und darf nie in der URL landen.
 */
function assertUrlVocabulary(page: Page, context: string) {
  const keys = [...new URL(page.url()).searchParams.keys()];
  const unexpected = keys.filter((key) => !ALLOWED_PARAMS.has(key));
  expect(
    unexpected,
    `${context}: unerwartete URL-Parameter ${unexpected.join(",")} in ${page.url()}`,
  ).toEqual([]);
}

async function login(page: Page, email: string, password: string) {
  // Login-Maske sitzt auf der Tenant-Wurzel; geschützte Routen leiten hier
  // NICHT auf sie um, sie rendern ohne Session nur eine leere Hülle.
  // `domcontentloaded` statt des Default-`load`: der Next-Dev-Server hält über
  // SSE Verbindungen offen, das `load`-Event kann dadurch hängen.
  await page.goto(`${base}/`, { waitUntil: "domcontentloaded" });
  await page.waitForSelector('input[type="email"]', { timeout: 20000 });
  await page.fill('input[type="email"]', email);
  await page.fill('input[type="password"]', password);
  await page.click('button:has-text("Anmelden")');
  // Nicht auf networkidle warten (SSE hält das Netzwerk dauerhaft aktiv);
  // stattdessen auf das Verschwinden der Login-Maske.
  await page.waitForSelector('input[type="email"]', {
    state: "detached",
    timeout: 20000,
  });
}

test.describe("Vertretung UI-Flow (#1886, Inkrement 2)", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(
      access === null,
      "backend/.seed-state.json (oder E2E_TENANT_SLUG/E2E_TEST_EMAIL/E2E_TEST_PASSWORD) erforderlich",
    );
    if (!access) return;
    await login(page, access.email, access.password);
  });

  test.afterEach(async ({ page }) => {
    // Netz für Timeout-Abbrüche: das in-Test-finally kann bei einem harten
    // Timeout abgeschnitten werden, afterEach läuft mit eigenem Budget.
    // Doppeltes Löschen ist harmlos (zweites Mal 404).
    if (!access || cleanupInstanceId === null) return;
    if (cleanupStaffId !== null) {
      await page.request
        .post(
          `${base}/api/timetable/instances/${cleanupInstanceId}/deviations`,
          { data: { presences: [cleanupStaffId] } },
        )
        .catch(() => undefined);
    }
    await page.request
      .delete(`${base}/api/timetable/instances/${cleanupInstanceId}`)
      .catch(() => undefined);
    cleanupInstanceId = null;
    cleanupStaffId = null;
  });

  test("Abwesenheit + Ersatz verdrahten, Verlauf, Deep-Links, URL-Vokabular", async ({
    page,
  }) => {
    if (!access) return;
    // Vielschrittiger E2E (Login, Anlage, zwei Speichervorgänge, mehrere
    // Navigationen); großzügiges Budget für einen kalten Dev-Server-Compile
    // des Routen-Erstaufrufs.
    test.setTimeout(90000);

    // --- Testdaten anlegen (spontaner Termin mit genau einer Person) --------
    // Die View zeigt nur Mo–Fr und snappt Wochenendtage auf den nächsten
    // Montag — Testdaten also immer auf einen Werktag legen.
    const day = nextWorkdayISO(berlinTodayISO());
    const rooms = (await (
      await page.request.get(`${base}/api/rooms`)
    ).json()) as
      { data: Array<{ id: string | number }> } | Array<{ id: string | number }>;
    const roomList = Array.isArray(rooms) ? rooms : rooms.data;
    expect(
      roomList.length,
      "Tenant braucht mindestens einen Raum",
    ).toBeGreaterThan(0);
    const roomId = Number(roomList[0]!.id);

    const staffPayload = (await (
      await page.request.get(`${base}/api/staff`)
    ).json()) as
      | { data: Array<{ id: string | number; name?: string }> }
      | Array<{ id: string | number; name?: string }>;
    const staffList = Array.isArray(staffPayload)
      ? staffPayload
      : staffPayload.data;
    // Nicht die beiden Seed-Admins (id 1/2) einplanen; irgendeine Betreuungskraft.
    const planned =
      staffList.find((s) => !["1", "2"].includes(String(s.id))) ?? staffList[0];
    expect(planned, "Tenant braucht mindestens eine Person").toBeTruthy();
    const plannedStaffId = Number(planned!.id);

    const title = `Playwright Vertretung ${Date.now()}`;
    const created = (await (
      await page.request.post(`${base}/api/timetable/instances`, {
        data: {
          date: day,
          start_time: "11:00",
          end_time: "12:00",
          title,
          room_id: roomId,
          staff_ids: [plannedStaffId],
        },
      })
    ).json()) as { data?: { id: string | number }; id?: string | number };
    const instanceId = String(created.data?.id ?? created.id ?? "");
    expect(instanceId, "Termin-Anlage muss eine ID liefern").not.toEqual("");
    // Aufräum-Anker sofort setzen, damit afterEach auch bei einem Abbruch
    // mitten im Flow den Termin wieder entfernt.
    cleanupInstanceId = instanceId;
    cleanupStaffId = plannedStaffId;

    // Genau ein POST .../deviations pro Speichern (Akzeptanzkriterium 7): nur
    // Seiten-Requests zählen. page.request (Setup/Cleanup) triggert dieses
    // Event nicht.
    let deviationPosts = 0;
    page.on("request", (req) => {
      if (req.method() === "POST" && DEVIATIONS_RE.test(req.url())) {
        deviationPosts += 1;
      }
    });

    const row = page.getByTestId(`vertretung-day-list-row-${instanceId}`);
    const dialog = page.getByRole("dialog");

    try {
      // --- Bereich öffnen -------------------------------------------------
      await page.goto(`${base}/vertretung?d=${day}`, {
        waitUntil: "domcontentloaded",
      });
      assertUrlVocabulary(page, "nach Navigation");

      // Der frische Termin ist noch ungestört; "Ganzer Tag" zeigt ihn. Der
      // Umschalter ist lokaler State und darf die URL nicht anfassen.
      const ganzerTag = page.getByRole("tab", { name: "Ganzer Tag" });
      await ganzerTag.click();
      await expect(ganzerTag).toHaveAttribute("aria-selected", "true");
      assertUrlVocabulary(page, "nach Filter-Umschalter (lokaler State)");

      await expect(row).toBeVisible({ timeout: 15000 });
      await expect(row).toContainText("Soll 1");
      await expect(row).toContainText("Abwesend 0");

      // --- Save 1: Person abwesend melden (Ereignis `absence`) ------------
      deviationPosts = 0;
      await row.getByRole("button", { name: "Bearbeiten" }).click();
      await expect(dialog).toBeVisible();
      assertUrlVocabulary(page, "nach Editor-Öffnen");
      await expect(dialog.getByText(title)).toBeVisible();

      const save1 = page.waitForResponse(
        (res) =>
          res.request().method() === "POST" && DEVIATIONS_RE.test(res.url()),
      );
      await dialog.getByRole("button", { name: "Abwesend" }).click();
      await dialog.getByRole("button", { name: "Speichern" }).click();
      expect((await save1).ok()).toBeTruthy();
      await expect(dialog).toBeHidden({ timeout: 10000 });
      expect(deviationPosts, "Save 1 = genau ein deviations-POST").toBe(1);

      // Ohne Ersatz steht die offene Lücke mit dem "??"-Muster in der Liste.
      await expect(row).toContainText("Abwesend 1", DATA);
      await expect(row).toContainText("Ersatz: ??", DATA);

      // --- Save 2: Ersatz wählen (Ereignis `substitution`) ----------------
      // Zwei Speichervorgänge, weil ein kombiniertes Abwesenheit+Ersatz-Save
      // backendseitig NUR ein `substitution`-Ereignis protokolliert
      // (ApplySubstitute -> logSubstitutionEvent); der Verlauf soll aber beide
      // Ereignisse zeigen. Jeder Speichervorgang bleibt ein einziger Request.
      deviationPosts = 0;
      await row.getByRole("button", { name: "Bearbeiten" }).click();
      await expect(dialog).toBeVisible();

      const picker = dialog.getByRole("combobox", { name: /Vertretung für/ });
      await picker.first().click();
      const options = page.getByRole("option");
      await expect(options.first()).toBeVisible();
      const substituteName =
        (await options.first().textContent())?.trim() ?? "";
      expect(substituteName, "Ersatzliste darf nicht leer sein").not.toEqual(
        "",
      );
      await options.first().click();

      const save2 = page.waitForResponse(
        (res) =>
          res.request().method() === "POST" && DEVIATIONS_RE.test(res.url()),
      );
      await dialog.getByRole("button", { name: "Speichern" }).click();
      expect((await save2).ok()).toBeTruthy();
      await expect(dialog).toBeHidden({ timeout: 10000 });
      expect(deviationPosts, "Save 2 = genau ein deviations-POST").toBe(1);

      // Aktualisiertes Soll/Ist/Abwesend-Tripel plus gesetzter Ersatz.
      await expect(row).toContainText("Soll 1", DATA);
      await expect(row).toContainText("Ist 1", DATA);
      await expect(row).toContainText("Abwesend 1", DATA);
      await expect(row).toContainText(`Ersatz: ${substituteName}`, DATA);

      // --- Deep-Link-Roundtrip: Editor + Verlaufs-Reiter direkt anfahren --
      await page.goto(
        `${base}/vertretung?d=${day}&block=${instanceId}&verlauf=1`,
        { waitUntil: "domcontentloaded" },
      );
      await expect(dialog).toBeVisible({ timeout: 15000 });
      assertUrlVocabulary(page, "nach Deep-Link block+verlauf");
      const verlaufTab = dialog.getByRole("tab", { name: "Verlauf" });
      await expect(verlaufTab).toHaveAttribute("aria-selected", "true", DATA);
      // Beide Ereignisse (deutsche Labels aus DEVIATION_EVENT_LABELS). Der
      // Tages-Scope einer spontanen Instanz (ohne Slot-Anker) listet ALLE
      // Ereignisse des Tages, auch die append-only Audit-Zeilen früherer Läufe;
      // geprüft wird das Vorhandensein, nicht die Eindeutigkeit -> `.first()`.
      await expect(
        dialog.getByText("Abwesenheit eingetragen").first(),
      ).toBeVisible(DATA);
      await expect(
        dialog.getByText("Vertretung zugewiesen").first(),
      ).toBeVisible(DATA);

      // Reiter-Wechsel: verlauf-Param togglet, Vokabular bleibt sauber.
      await dialog.getByRole("tab", { name: "Bearbeiten" }).click();
      await expect(
        dialog.getByRole("tab", { name: "Bearbeiten" }),
      ).toHaveAttribute("aria-selected", "true");
      assertUrlVocabulary(page, "nach Reiter-Wechsel -> Bearbeiten");
      expect(new URL(page.url()).searchParams.has("verlauf")).toBe(false);

      await dialog.getByRole("tab", { name: "Verlauf" }).click();
      await expect(verlaufTab).toHaveAttribute("aria-selected", "true");
      assertUrlVocabulary(page, "nach Reiter-Wechsel -> Verlauf");
      expect(new URL(page.url()).searchParams.get("verlauf")).toBe("1");

      // Editor schließen -> block + verlauf entfallen.
      await dialog.getByRole("button", { name: "Schließen" }).click();
      await expect(dialog).toBeHidden();
      assertUrlVocabulary(page, "nach Editor-Schließen");
      expect(new URL(page.url()).searchParams.has("block")).toBe(false);

      // --- Redirect-Check: Alt-Einstieg /vertretungsplan?instance=&history=1
      await page.goto(
        `${base}/vertretungsplan?instance=${instanceId}&history=1`,
        { waitUntil: "domcontentloaded" },
      );
      await page.waitForURL((url) => url.pathname.endsWith("/vertretung"), {
        timeout: 10000,
      });
      const redirected = new URL(page.url());
      expect(redirected.searchParams.get("block")).toBe(instanceId);
      expect(redirected.searchParams.get("verlauf")).toBe("1");
      assertUrlVocabulary(page, "nach Redirect vom Alt-Einstieg");

      // --- Tag-Wechsel: setzt `d`, räumt block/verlauf ab, Vokabular sauber
      await page.goto(`${base}/vertretung?d=${day}`, {
        waitUntil: "domcontentloaded",
      });
      const target = adjacentDayInSameWeek(day);
      await page
        .getByLabel(new RegExp(`${target.dd}\\.${target.mm}\\.$`))
        .click();
      await expect
        .poll(() => new URL(page.url()).searchParams.get("d"))
        .toBe(target.iso);
      assertUrlVocabulary(page, "nach Tag-Wechsel");
    } finally {
      // Aufräumen: tagesweite Abwesenheit zurücknehmen und den spontanen
      // Termin löschen (Status bleibt `planned` -> löschbar). Beides
      // best-effort, damit ein Flow-Fehler den Cleanup nicht verschluckt.
      await page.request
        .post(`${base}/api/timetable/instances/${instanceId}/deviations`, {
          data: { presences: [plannedStaffId] },
        })
        .catch(() => undefined);
      await page.request
        .delete(`${base}/api/timetable/instances/${instanceId}`)
        .catch(() => undefined);
      // Erledigt -> afterEach zum No-op machen.
      cleanupInstanceId = null;
      cleanupStaffId = null;
    }
  });
});

/**
 * Ein anderer Tag derselben Mo–Fr-Leiste wie `iso` (für den Tag-Chip-Klick).
 * `iso` ist immer ein Werktag (nextWorkdayISO); Freitag weicht rückwärts aus,
 * alle anderen Tage vorwärts.
 */
function adjacentDayInSameWeek(iso: string): {
  iso: string;
  dd: string;
  mm: string;
} {
  const noon = new Date(`${iso}T12:00:00`);
  const delta = noon.getDay() === 5 ? -1 : 1; // 5 = Freitag
  noon.setDate(noon.getDate() + delta);
  const pad = (n: number) => String(n).padStart(2, "0");
  const dd = pad(noon.getDate());
  const mm = pad(noon.getMonth() + 1);
  return { iso: `${noon.getFullYear()}-${mm}-${dd}`, dd, mm };
}
