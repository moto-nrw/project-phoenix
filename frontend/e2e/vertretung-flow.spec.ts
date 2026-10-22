import { expect, type Page, test } from "@playwright/test";

import {
  loginToTenant,
  MISSING_SEED_ACCESS,
  seedAccess as access,
  tenantBase as base,
} from "./tenant-session";
import { berlinTodayISO } from "../src/lib/date-helpers";
import { nextWorkdayISO } from "../src/lib/timetable-helpers";

// Der Zweiteiler /vertretung. Diese Spec testet AUSSCHLIESSLICH die UI-Verdrahtung, nicht die
// Deviations-Backend-Semantik (die decken die Go-E2E-Flows
// flow_c_gaps_substitute_test.go / flow_h_replan_deviations_test.go ab).
//
// Testdaten sind vollständig selbst angelegt und wieder aufgeräumt: die Spec
// erzeugt über die API zwei spontane Termine mit derselben geplanten Person,
// fährt den terminbezogenen Flow (abwesend melden -> Ersatz wählen) und löscht
// beide Termine im finally wieder. Die
// Dev-DB gehört dem Nutzer, daher kein Reset, kein Reseed, keine Fremd-Daten.
//
// Login und Tenant-Herkunft: e2e/tenant-session.ts.

// `view` kam mit der Wochenansicht (#2030) hinzu; die Tagesansicht bleibt der
// Standard und schreibt den Parameter nicht.
const ALLOWED_PARAMS = new Set(["d", "view", "block", "verlauf"]);
const DEVIATIONS_RE = /\/api\/timetable\/instances\/\d+\/deviations$/;
// Seit 9cdca3668b speichert der Editor Abwesenheit/Ersatz über das gemeinsame
// Vertretungsmodul (POST /api/substitutions, type schedule_substitution);
// POST .../deviations bleibt der Weg für Absagen.
const SUBSTITUTIONS_RE = /\/api\/substitutions$/;
const isSaveUrl = (url: string) =>
  SUBSTITUTIONS_RE.test(url) || DEVIATIONS_RE.test(url);
// Großzügiges Budget für Zusicherungen, die von einem SWR-Refetch gegen den
// Next-Dev-Server abhängen (Liste nach Save, Verlauf nach Deep-Link).
const DATA = { timeout: 15000 } as const;

// Aufräum-Anker: gesetzt, sobald der Termin angelegt ist. Das in-Test-finally
// räumt im Normalfall auf; afterEach ist das Netz für Timeout-Abbrüche (eigenes
// Zeitbudget), damit ein abgebrochener Lauf keine Waisen in der Dev-DB lässt.
// Playwright-Worker sind eigene Prozesse -> Modul-State ist pro Worker, kein
// Cross-Test-Rennen.
let cleanupInstanceIds: string[] = [];
let cleanupStaffId: number | null = null;

/**
 * URL-Vokabular-Invariante (Akzeptanzkriterium 3): der Bereich trägt nie mehr
 * als `d`, `view`, `block`, `verlauf`. Der Filterzustand ("Nur Störungen |
 * Ganzer Tag") ist lokaler State und darf nie in der URL landen.
 */
function assertUrlVocabulary(page: Page, context: string) {
  const keys = [...new URL(page.url()).searchParams.keys()];
  const unexpected = keys.filter((key) => !ALLOWED_PARAMS.has(key));
  expect(
    unexpected,
    `${context}: unerwartete URL-Parameter ${unexpected.join(",")} in ${page.url()}`,
  ).toEqual([]);
}

test.describe("Vertretung UI-Flow (#1886, Inkrement 2)", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(access === null, MISSING_SEED_ACCESS);
    if (!access) return;
    await loginToTenant(page, access);
  });

  test.afterEach(async ({ page }) => {
    // Netz für Timeout-Abbrüche: das in-Test-finally kann bei einem harten
    // Timeout abgeschnitten werden, afterEach läuft mit eigenem Budget.
    // Doppeltes Löschen ist harmlos (zweites Mal 404).
    if (!access || cleanupInstanceIds.length === 0) return;
    for (const instanceId of cleanupInstanceIds) {
      if (cleanupStaffId !== null) {
        await page.request
          .post(`${base}/api/timetable/instances/${instanceId}/deviations`, {
            data: {
              presences: [
                {
                  staff_id: cleanupStaffId,
                  instance_ids: [Number(instanceId)],
                },
              ],
            },
          })
          .catch(() => undefined);
      }
      await page.request
        .delete(`${base}/api/timetable/instances/${instanceId}`)
        .catch(() => undefined);
    }
    cleanupInstanceIds = [];
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

    // --- Testdaten: zwei Termine derselben Person am selben Tag ------------
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
    // Nicht die angemeldete Person einplanen (wie dienstplan.spec.ts: feste
    // Admin-IDs passen nicht zum Seed); irgendeine
    // Betreuungskraft OHNE eigene Termine an `day`. Der Seed (seed_planning.go,
    // seedPlanningException) meldet die erste Betreuungskraft heute in der
    // Frühbetreuung abwesend mit Ersatz; diese Abwesenheit wäre im Editor unter
    // "Bestimmte Termine" vorausgewählt und ließe Save 2 mit "Der Termin hat
    // schon eine andere Ersatzperson" scheitern.
    const dayInstancesResponse = await page.request.get(
      `${base}/api/timetable/instances?from=${day}&to=${day}`,
    );
    expect(
      dayInstancesResponse.ok(),
      `Termine am Testtag laden fehlgeschlagen (${dayInstancesResponse.status()})`,
    ).toBe(true);
    const dayInstances = (await dayInstancesResponse.json()) as {
      data?: { instances?: Array<{ staff?: Array<{ staff_id: number }> }> };
    };
    const busyStaffIds = new Set(
      (dayInstances.data?.instances ?? []).flatMap((instance) =>
        (instance.staff ?? []).map((s) => String(s.staff_id)),
      ),
    );
    const meResponse = await page.request.get(`${base}/api/me/staff`);
    expect(
      meResponse.ok(),
      `eigenes Personal laden fehlgeschlagen (${meResponse.status()})`,
    ).toBe(true);
    const own = (await meResponse.json()) as {
      data?: { id?: number | string };
    };
    const ownId = String(own.data?.id ?? "");
    expect(ownId, "eigene Personal-ID fehlt").not.toBe("");
    const planned = staffList.find(
      (s) => String(s.id) !== ownId && !busyStaffIds.has(String(s.id)),
    );
    expect(
      planned,
      "Tenant braucht mindestens eine Person ohne Termine am Testtag",
    ).toBeTruthy();
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
    const otherTitle = `${title} zweiter Termin`;
    const otherCreated = (await (
      await page.request.post(`${base}/api/timetable/instances`, {
        data: {
          date: day,
          start_time: "14:00",
          end_time: "15:00",
          title: otherTitle,
          room_id: roomId,
          staff_ids: [plannedStaffId],
        },
      })
    ).json()) as { data?: { id: string | number }; id?: string | number };
    const otherInstanceId = String(
      otherCreated.data?.id ?? otherCreated.id ?? "",
    );
    expect(otherInstanceId, "Zweiter Termin muss eine ID liefern").not.toEqual(
      "",
    );
    // Aufräum-Anker sofort setzen, damit afterEach auch bei einem Abbruch
    // mitten im Flow den Termin wieder entfernt.
    cleanupInstanceIds = [instanceId, otherInstanceId];
    cleanupStaffId = plannedStaffId;

    // Genau ein Speicher-POST (substitutions ODER deviations) pro Speichern
    // (Akzeptanzkriterium 7): nur Seiten-Requests zählen. page.request
    // (Setup/Cleanup) triggert dieses Event nicht.
    let deviationPosts = 0;
    page.on("request", (req) => {
      if (req.method() === "POST" && isSaveUrl(req.url())) {
        deviationPosts += 1;
      }
    });

    const row = page.getByTestId(`vertretung-day-list-row-${instanceId}`);
    const otherRow = page.getByTestId(
      `vertretung-day-list-row-${otherInstanceId}`,
    );
    const dialog = page.getByRole("dialog");

    try {
      // --- Bereich öffnen -------------------------------------------------
      await page.goto(`${base}/vertretung?d=${day}`, {
        waitUntil: "domcontentloaded",
      });
      assertUrlVocabulary(page, "nach Navigation");

      // Der frische Termin ist noch ungestört; "Ganzer Tag" zeigt ihn. Der
      // Umschalter ist lokaler State und darf die URL nicht anfassen.
      // Seit 7b2e83b328 ein SegmentedControl (Toggle-Buttons mit
      // aria-pressed) statt Radix-Tabs (aria-selected).
      const ganzerTag = page.getByRole("button", {
        name: "Ganzer Tag",
        exact: true,
      });
      await ganzerTag.click();
      await expect(ganzerTag).toHaveAttribute("aria-pressed", "true");
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
          res.request().method() === "POST" && SUBSTITUTIONS_RE.test(res.url()),
      );
      await dialog
        .getByRole("button", { name: /als abwesend markieren/ })
        .click();
      // Der sichtbare Label-Text ist das eigentliche Klickziel. Das native
      // Radio bleibt für Tastatur und Formsemantik sr-only im DOM; ein direkter
      // Koordinatenklick darauf kann hinter dem Footer landen.
      await dialog.locator('label[for^="vp-scope-selected-"]').click();
      await expect(
        dialog.getByRole("checkbox", { name: new RegExp(title) }).first(),
      ).toBeChecked();
      await expect(
        dialog.getByRole("checkbox", { name: new RegExp(otherTitle) }),
      ).not.toBeChecked();
      await dialog.getByRole("button", { name: "Speichern" }).click();
      expect((await save1).ok()).toBeTruthy();
      await expect(dialog).toBeHidden({ timeout: 10000 });
      expect(deviationPosts, "Save 1 = genau ein Speicher-POST").toBe(1);

      // Ohne Ersatz nennt die Liste die offene Vertretung ausdrücklich.
      await expect(row).toContainText("Abwesend 1", DATA);
      await expect(row).toContainText("Ersatzkräfte: keine", DATA);
      await expect(otherRow).toContainText("Abwesend 0", DATA);

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
          res.request().method() === "POST" && SUBSTITUTIONS_RE.test(res.url()),
      );
      await dialog.getByRole("button", { name: "Speichern" }).click();
      expect((await save2).ok()).toBeTruthy();
      await expect(dialog).toBeHidden({ timeout: 10000 });
      expect(deviationPosts, "Save 2 = genau ein Speicher-POST").toBe(1);

      // Aktualisiertes Soll/Ist/Abwesend-Tripel plus gesetzter Ersatz.
      await expect(row).toContainText("Soll 1", DATA);
      await expect(row).toContainText("Ist 1", DATA);
      await expect(row).toContainText("Abwesend 1", DATA);
      await expect(row).toContainText(`Ersatzkräfte: ${substituteName}`, DATA);

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
      // Aufräumen best-effort, damit ein Flow-Fehler keine Termine hinterlässt.
      for (const cleanupId of [instanceId, otherInstanceId]) {
        await page.request
          .post(`${base}/api/timetable/instances/${cleanupId}/deviations`, {
            data: {
              presences: [
                {
                  staff_id: plannedStaffId,
                  instance_ids: [Number(cleanupId)],
                },
              ],
            },
          })
          .catch(() => undefined);
        await page.request
          .delete(`${base}/api/timetable/instances/${cleanupId}`)
          .catch(() => undefined);
      }
      // Erledigt -> afterEach zum No-op machen.
      cleanupInstanceIds = [];
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
