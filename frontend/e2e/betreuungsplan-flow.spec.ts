import { expect, type Page, test } from "@playwright/test";
import { readFileSync } from "node:fs";
import { join } from "node:path";

import { berlinTodayISO } from "../src/lib/date-helpers";
import { nextWorkdayISO } from "../src/lib/timetable-helpers";

// Chunk 9 des Planung-Redesigns Inkrement 4
// (docs/planung-redesign/docs/06-betreuungsplan.md Abschnitt 14): der
// eigenständige /betreuungsplan-Bereich. Diese Spec testet die UI-Verdrahtung
// des Drei-Parameter-Vokabulars (d / view / block), das Kalenderraster als
// erstes Inhaltselement, den Slide-Over, den Lückensprung, die "+ Neu"-Anlage
// und den Alt-Einstieg-Redirect. Die Deviations-/Vertretungs-Backend-Semantik
// deckt vertretung-flow.spec.ts (und die Go-E2E-Flows) ab.
//
// Testdaten sind vollständig selbst angelegt und wieder aufgeräumt: die Spec
// erzeugt über die API einen unterbesetzten Termin (required_staff 2, aber nur
// eine geplante Person) — der als offene Lücke im Raster UND in der
// Sprungliste erscheint — und löscht jede angelegte Instanz im finally/afterEach
// wieder. Die Dev-DB gehört dem Nutzer, daher kein Reset, kein Reseed.
//
// Login und Tenant-Herkunft spiegeln vertretung-flow.spec.ts: der lokale Stack
// bedient die Tenant-App nur unter {slug}.localhost:3000 (bare localhost ist
// die Marketing-Seite), die Login-Maske liegt auf der Tenant-Wurzel `/`. Slug
// und Admin-Zugang stehen in backend/.seed-state.json; E2E_TENANT_SLUG /
// E2E_TEST_EMAIL / E2E_TEST_PASSWORD überschreiben sie. Ohne verwertbare Werte
// überspringt die Spec.

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

// Das verbindliche Drei-Parameter-Vokabular (06 §2.1). Kein Navigationsschritt
// darf einen der sieben Alt-Parameter (week/month/year/instance/day/period/
// Alt-view) hinterlassen; der Dichte-Umschalter bleibt reiner Component-State.
const ALLOWED_PARAMS = new Set(["d", "view", "block"]);
const FORBIDDEN_PARAMS = ["week", "month", "year", "instance", "day", "period"];
// Großzügiges Budget für Zusicherungen, die von einem SWR-Refetch gegen den
// Dev-Server abhängen (Rasterdaten nach Anlage, Lückenliste nach Refetch).
const DATA = { timeout: 15000 } as const;

// Der Betreuungsplan ruft NIE die Deviations-Endpunkte (06 §6: nur Sprünge nach
// /vertretung). Ein Treffer hier ist ein harter Fehler.
const DEVIATIONS_RE =
  /\/api\/timetable\/(instances\/\d+\/deviations|deviations\/history)/;

// Aufräum-Anker: jede angelegte Instanz-ID landet hier; afterEach löscht sie
// (best effort, doppeltes Löschen ist harmlos -> 404). Playwright-Worker sind
// eigene Prozesse -> Modul-State ist pro Worker, kein Cross-Test-Rennen.
const createdInstanceIds: string[] = [];

/**
 * URL-Vokabular-Invariante (Kriterium 2): der Bereich trägt nie mehr als
 * `d`, `view`, `block`, und keinen der Alt-Parameter.
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
  // `domcontentloaded` statt `load`: der Next-Dev-Server hält über SSE
  // Verbindungen offen, das `load`-Event kann dadurch hängen.
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

/** Legt einen unterbesetzten Einzeltermin über die API an: Bedarf 2, aber
 *  niemand geplant. Die Backend-Lückenerkennung ist rein anwesenheitsbasiert
 *  (services/schedule.IsUnderstaffedCounts: present==0 || present<planned) —
 *  ohne geplante Person ist present=0 -> offene Lücke im Raster + Sprungliste,
 *  und required 2 > assigned 0 blendet zugleich den Vertretungs-Sprunglink ein.
 *  Gibt die Instanz-ID zurück und merkt sie für den Cleanup vor. */
async function createGapInstance(
  page: Page,
  day: string,
  title: string,
): Promise<{ id: string; startTime: string; endTime: string }> {
  const startTime = "11:00";
  const endTime = "12:00";
  const rooms = (await (await page.request.get(`${base}/api/rooms`)).json()) as
    { data: Array<{ id: string | number }> } | Array<{ id: string | number }>;
  const roomList = Array.isArray(rooms) ? rooms : rooms.data;
  expect(
    roomList.length,
    "Tenant braucht mindestens einen Raum",
  ).toBeGreaterThan(0);
  const roomId = Number(roomList[0]!.id);

  const created = (await (
    await page.request.post(`${base}/api/timetable/instances`, {
      data: {
        date: day,
        start_time: startTime,
        end_time: endTime,
        title,
        room_id: roomId,
        // Niemand geplant (present=0) -> offene Lücke; Bedarf 2 -> Sprunglink.
        staff_ids: [],
        required_staff: 2,
      },
    })
  ).json()) as { data?: { id: string | number }; id?: string | number };
  const id = String(created.data?.id ?? created.id ?? "");
  expect(id, "Termin-Anlage muss eine ID liefern").not.toEqual("");
  createdInstanceIds.push(id);
  return { id, startTime, endTime };
}

test.describe("Betreuungsplan UI-Flow (Inkrement 4)", () => {
  test.beforeEach(async ({ page }) => {
    test.skip(
      access === null,
      "backend/.seed-state.json (oder E2E_TENANT_SLUG/E2E_TEST_EMAIL/E2E_TEST_PASSWORD) erforderlich",
    );
    if (!access) return;
    await login(page, access.email, access.password);
  });

  test.afterEach(async ({ page }) => {
    if (!access) return;
    while (createdInstanceIds.length > 0) {
      const id = createdInstanceIds.pop()!;
      await page.request
        .delete(`${base}/api/timetable/instances/${id}`)
        .catch(() => undefined);
    }
  });

  test("Kalenderraster, Wochen-/Ansichtswechsel und Alt-Einstieg halten das URL-Vokabular", async ({
    page,
  }) => {
    if (!access) return;
    test.setTimeout(90000);

    // Der Betreuungsplan darf keinen Deviations-Endpunkt anfassen (06 §6).
    let deviationCalls = 0;
    page.on("request", (req) => {
      if (DEVIATIONS_RE.test(req.url())) deviationCalls += 1;
    });

    // --- Erstaufruf: das Raster ist das erste Inhaltselement ---------------
    // Alt-Parameter bewusst mitgeben: der Erstaufruf lässt sie stehen (die
    // Seite liest nur d/view/block), aber der erste Navigationsschritt muss sie
    // ersatzlos abräumen (Kriterium 2).
    const today = berlinTodayISO();
    await page.goto(`${base}/betreuungsplan?d=${today}&week=2&instance=99`, {
      waitUntil: "domcontentloaded",
    });

    // Kopfzeile + Kalenderraster (Tageskopfzeile "N P.") als erstes
    // Inhaltselement; keine Alt-Chrome-Texte mehr.
    await expect(
      page.getByRole("heading", { name: "Betreuungsplan" }),
    ).toBeVisible({ timeout: 20000 });
    await expect(page.getByText(/\d+\s*P\./).first()).toBeVisible(DATA);
    await expect(page.getByText("Betreuungsplan im Blick")).toHaveCount(0);

    // --- Navigation: nach dem ersten Schritt sind die Alt-Params weg --------
    const before = new URL(page.url()).searchParams.get("d");
    await page.getByRole("button", { name: "Weiter" }).click();
    await expect
      .poll(() => new URL(page.url()).searchParams.get("d"))
      .not.toBe(before);
    assertUrlVocabulary(page, "nach Weiter");
    for (const forbidden of FORBIDDEN_PARAMS) {
      expect(
        new URL(page.url()).searchParams.has(forbidden),
        `Alt-Parameter ${forbidden} überlebte den Wochenwechsel`,
      ).toBe(false);
    }

    // goToWeek ankert auf den Montag der Zielwoche, deshalb kein exakter
    // Round-Trip von `before` (heute) — die Rückwärtsnavigation landet auf dem
    // Montag der Vorwoche. Geprüft wird, dass jeder Schritt `d` bewegt und das
    // Vokabular sauber hält (Kriterium 2), nicht die Tag-Identität.
    const afterNext = new URL(page.url()).searchParams.get("d");
    await page.getByRole("button", { name: "Zurück" }).click();
    await expect
      .poll(() => new URL(page.url()).searchParams.get("d"))
      .not.toBe(afterNext);
    assertUrlVocabulary(page, "nach Zurück");

    // --- Ansichtswechsel: view=monat / view=serien / zurück auf woche -------
    await page.getByRole("tab", { name: "Monat" }).click();
    await expect
      .poll(() => new URL(page.url()).searchParams.get("view"))
      .toBe("monat");
    assertUrlVocabulary(page, "nach Ansicht Monat");

    await page.getByRole("tab", { name: "Serien" }).click();
    await expect
      .poll(() => new URL(page.url()).searchParams.get("view"))
      .toBe("serien");
    assertUrlVocabulary(page, "nach Ansicht Serien");

    await page.getByRole("tab", { name: "Woche" }).click();
    // Woche ist der Default -> view-Param entfällt.
    await expect
      .poll(() => new URL(page.url()).searchParams.has("view"))
      .toBe(false);
    assertUrlVocabulary(page, "nach Ansicht Woche");

    // --- Alt-Einstieg-Redirect: /planung?tab=betreuung&instance=42 ----------
    // -> /betreuungsplan?block=42 (reine Parameter-Übersetzung, keine Daten).
    await page.goto(`${base}/planung?tab=betreuung&instance=42`, {
      waitUntil: "domcontentloaded",
    });
    await page.waitForURL((url) => url.pathname.endsWith("/betreuungsplan"), {
      timeout: 15000,
    });
    expect(new URL(page.url()).searchParams.get("block")).toBe("42");
    assertUrlVocabulary(page, "nach Alt-Einstieg-Redirect");

    expect(
      deviationCalls,
      "Der Betreuungsplan darf keinen Deviations-Endpunkt aufrufen",
    ).toBe(0);
  });

  test("Block-Klick, Slide-Over, Vertretungssprung und Lückensprung", async ({
    page,
  }) => {
    if (!access) return;
    test.setTimeout(90000);

    // nextWorkdayISO: die Woche zeigt Mo–Fr, ein Wochenendtag würde snappen.
    const day = nextWorkdayISO(berlinTodayISO());
    const title = `Playwright Betreuungsplan ${Date.now()}`;
    const { id, startTime, endTime } = await createGapInstance(
      page,
      day,
      title,
    );

    await page.goto(`${base}/betreuungsplan?d=${day}`, {
      waitUntil: "domcontentloaded",
    });
    assertUrlVocabulary(page, "nach Navigation");

    // --- Block-Klick öffnet den Slide-Over, setzt `block` -------------------
    const block = page.getByRole("button", {
      name: `${title}, ${startTime} bis ${endTime}`,
    });
    await expect(block).toBeVisible(DATA);
    await block.click();

    const slideOver = page.getByRole("dialog", { name: title });
    await expect(slideOver).toBeVisible(DATA);
    await expect
      .poll(() => new URL(page.url()).searchParams.get("block"))
      .toBe(id);
    assertUrlVocabulary(page, "nach Block-Klick");

    // --- "Vertretung bearbeiten" verweist auf /vertretung?d=&block= ---------
    // Der Block ist unterbesetzt -> der Sprunglink ist sichtbar.
    const vertretungLink = slideOver.getByRole("link", {
      name: "Vertretung bearbeiten",
    });
    await expect(vertretungLink).toBeVisible();
    const href = await vertretungLink.getAttribute("href");
    expect(href).toContain("/vertretung");
    expect(href).toContain(`d=${day}`);
    expect(href).toContain(`block=${id}`);

    // --- Schließen räumt `block` ab -----------------------------------------
    await page.getByRole("button", { name: "Schließen" }).click();
    await expect(slideOver).toBeHidden(DATA);
    await expect
      .poll(() => new URL(page.url()).searchParams.has("block"))
      .toBe(false);
    assertUrlVocabulary(page, "nach Slide-Over-Schließen");

    // --- Lückenzähler -> Sprungliste -> unterbesetzter Block ----------------
    const gapTrigger = page.getByRole("button", { name: /L(ü|ue)cke/ });
    await expect(gapTrigger).toBeVisible(DATA);
    await gapTrigger.click();
    const gapPopover = page.getByRole("dialog", { name: "Offene Lücken" });
    await expect(gapPopover).toBeVisible();
    await gapPopover.getByRole("button", { name: new RegExp(title) }).click();

    // Sprung setzt d + block und öffnet den Slide-Over.
    await expect
      .poll(() => new URL(page.url()).searchParams.get("block"))
      .toBe(id);
    expect(new URL(page.url()).searchParams.get("d")).toBe(day);
    await expect(page.getByRole("dialog", { name: title })).toBeVisible(DATA);
    assertUrlVocabulary(page, "nach Lückensprung");
  });

  test("+ Neu legt einen Einzeltermin an, der im Raster erscheint", async ({
    page,
  }) => {
    if (!access) return;
    test.setTimeout(90000);

    const day = nextWorkdayISO(berlinTodayISO());
    const title = `Playwright Neu ${Date.now()}`;

    await page.goto(`${base}/betreuungsplan?d=${day}`, {
      waitUntil: "domcontentloaded",
    });
    await expect(
      page.getByRole("heading", { name: "Betreuungsplan" }),
    ).toBeVisible({ timeout: 20000 });

    // "+ Neu" -> "Einmaliger Termin" öffnet das Event-Modal.
    // exact: sonst matchen die "Neuen Termin anlegen …"-Slot-Buttons mit.
    await page.getByRole("button", { name: "Neu", exact: true }).click();
    await page.getByRole("menuitem", { name: "Einmaliger Termin" }).click();

    const modal = page.getByRole("dialog");
    await expect(modal.locator("#event_title")).toBeVisible(DATA);

    // Das Formular ist ein Drei-Schritt-Wizard: der Stepper steht sichtbar über
    // den Feldern und beginnt auf Schritt 1 ("Termin"). Alles, was danach
    // ausgefüllt wird, gehört zu Schritt 1 — der anschließende Speichern-Klick
    // ist damit der Beleg, dass Speichern schon aus Schritt 1 möglich ist
    // (Kriterium 1), ohne "Weiter" durch die Schritte 2 und 3.
    await expect(modal.getByText("Schritt 1 von 3")).toBeVisible();

    await modal.locator("#event_title").fill(title);
    await modal.locator("#event_date").fill(day);
    await modal.locator("#event_start").fill("14:00");
    await modal.locator("#event_end").fill("15:00");
    // Erster echter Raum (Index 0 ist der Platzhalter "Raum auswählen …").
    await modal.locator("#event_room").selectOption({ index: 1 });

    // Nach dem Speichern setzt onSaved `block` auf die neue Instanz-ID; die
    // fangen wir für den Cleanup ein.
    await modal.getByRole("button", { name: "Speichern" }).click();
    await expect
      .poll(() => new URL(page.url()).searchParams.get("block"), DATA)
      .not.toBeNull();
    const newId = new URL(page.url()).searchParams.get("block");
    if (newId) createdInstanceIds.push(newId);

    // Der Slide-Over des frisch angelegten Termins ist offen; schließen, damit
    // der neue Block im Raster sichtbar wird. Close-Button auf den Slide-Over
    // scopen — ein Erfolgs-Toast trägt ebenfalls ein "Schließen".
    const newSlideOver = page.getByRole("dialog", { name: title });
    await expect(newSlideOver).toBeVisible(DATA);
    await newSlideOver.getByRole("button", { name: "Schließen" }).click();
    await expect(newSlideOver).toBeHidden(DATA);

    // Der neue Block liegt jetzt im Wochenraster.
    await expect(
      page.getByRole("button", { name: `${title}, 14:00 bis 15:00` }),
    ).toBeVisible(DATA);
    assertUrlVocabulary(page, "nach Neu-Anlage");
  });

  test('"Wiederholen" öffnet den Wizard direkt auf Schritt 2', async ({
    page,
  }) => {
    if (!access) return;
    test.setTimeout(90000);

    // Ein per API angelegter Einzeltermin (status "planned", keine
    // activityGroupId) ist genau der Fall, für den der Slide-Over die Aktion
    // "Wiederholen" anbietet: die Konvertierung Einzeltermin -> Serie.
    const day = nextWorkdayISO(berlinTodayISO());
    const title = `Playwright Wiederholen ${Date.now()}`;
    const { startTime, endTime } = await createGapInstance(page, day, title);

    await page.goto(`${base}/betreuungsplan?d=${day}`, {
      waitUntil: "domcontentloaded",
    });

    // Erst warten, bis die Planungszeiträume geladen sind: "Wiederholen" prüft
    // sie (leere Liste -> Hinweis-Toast statt Wizard), und solange der SWR-Load
    // läuft, ist die Liste noch leer. Der Zeitraum-Umschalter erscheint genau
    // dann, wenn die Daten da sind — vorher steht dort ein Skeleton.
    await expect(
      page.locator('button[title="Planungszeitraum wechseln"]'),
    ).toBeVisible(DATA);

    const block = page.getByRole("button", {
      name: `${title}, ${startTime} bis ${endTime}`,
    });
    await expect(block).toBeVisible(DATA);
    await block.click();

    const slideOver = page.getByRole("dialog", { name: title });
    await expect(slideOver).toBeVisible(DATA);

    await slideOver.getByRole("button", { name: "Wiederholen" }).click();

    // Kern der Zusicherung: die Konvertierung überspringt Schritt 1 ("Termin",
    // die Eckdaten stehen ja schon) und öffnet den Wizard direkt auf Schritt 2
    // ("Wiederholung"). Der Speicherteil der Konvertierung ist per Vitest
    // abgedeckt — hier zählt nur der Einstiegspunkt.
    const modal = page.getByRole("dialog", { name: "Termin wiederholen" });
    await expect(modal).toBeVisible(DATA);
    await expect(modal.getByText("Schritt 2 von 3")).toBeVisible();
    // Die volle Variante rendert "Wiederholt sich" als Überschrift über einer
    // Tab-Gruppe (kein <label> an einem Feld) — daher über die Rolle greifen.
    await expect(modal.getByText("Wiederholt sich")).toBeVisible();
    await expect(
      modal.getByRole("tablist", { name: "Wiederholung" }),
    ).toBeVisible();

    // Gegenprobe, dass wirklich Schritt 2 offen ist und nicht Schritt 1:
    // das Titelfeld aus Schritt 1 ist nicht gerendert, "Zurück" dagegen schon.
    await expect(modal.locator("#event_title")).toHaveCount(0);
    await expect(modal.getByRole("button", { name: "Zurück" })).toBeVisible();
  });
});
