import { readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

// Gate für das Seitengerüst (siehe .claude/rules/frontend-ui-kit.md und
// components/ui/TENANT-PAGE-SPEC.md): jede Tenant-Seite rendert `TenantPage`
// und baut kein eigenes Layout. Der Test liest Quelltext, nicht das DOM — er
// hält die Konvention auch für Seiten fest, die noch keinen eigenen Test haben.

const SRC_DIR = path.join(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
  "..",
);
const PROTECTED_DIR = path.join(SRC_DIR, "app", "[tenant]", "(protected)");

/**
 * Seiten mit bewusst eigenem Gerüst. Die Liste ist leer und soll es bleiben:
 * Startseite, Profil und Notfallliste hatten früher ein eigenes Layout und
 * tragen seit dem Umbau dasselbe Gerüst wie alle anderen Seiten. Ein neuer
 * Eintrag braucht die Zustimmung im PR (Regel „When to deviate").
 */
const EXEMPT = new Set<string>();

function collectPageFiles(directory: string): string[] {
  return readdirSync(directory).flatMap((entry) => {
    const entryPath = path.join(directory, entry);
    if (statSync(entryPath).isDirectory()) return collectPageFiles(entryPath);
    return entry === "page.tsx" ? [entryPath] : [];
  });
}

const pages = collectPageFiles(PROTECTED_DIR)
  .map((file) => ({
    id: path.relative(PROTECTED_DIR, file),
    file,
    source: readFileSync(file, "utf8"),
  }))
  .filter((page) => !EXEMPT.has(page.id));

/**
 * Seiten, die nur eine View-Komponente rendern, tragen das Gerüst in dieser
 * Komponente. Sie erkennen wir daran, dass die Datei selbst kaum Markup hat.
 */
function rendersOwnMarkup(source: string): boolean {
  return /<(div|section|main|h1|header)\b/.test(source);
}

// --- Bauart-Deklaration (BAUARTEN-SPEC.md, Ratsche 7, #3118) ------------------
//
// Jede Seite entscheidet sich für genau eine der vier Bauarten. Die Zuordnung
// liegt hier zentral, damit eine neue Seite ohne Eintrag und ein Eintrag ohne
// Seite gleichermaßen durchfallen. Schlüssel sind die Seiten-IDs relativ zu
// app/[tenant]/(protected), mit "/" als Trenner.

type Bauart = "sammlung" | "objekt" | "werkzeug" | "einstellungen";

const BAUART: Readonly<Record<string, Bauart>> = {
  // Bauart 1 — Sammlung: eine Liste von Objekten eines Typs.
  "absences/page.tsx": "sammlung",
  "activities/page.tsx": "sammlung",
  "admin/enrollments/page.tsx": "sammlung",
  "admin/guardian-approvals/page.tsx": "sammlung",
  "anfragen/page.tsx": "sammlung",
  "calendar-periods/page.tsx": "sammlung",
  "care-offerings/page.tsx": "sammlung",
  "database/absence-types/page.tsx": "sammlung",
  "database/activities/page.tsx": "sammlung",
  "database/categories/page.tsx": "sammlung",
  "database/devices/page.tsx": "sammlung",
  "database/groups/page.tsx": "sammlung",
  "database/permissions/page.tsx": "sammlung",
  "database/personal/page.tsx": "sammlung",
  "database/planning-tracks/page.tsx": "sammlung",
  "database/roles/page.tsx": "sammlung",
  "database/rooms/page.tsx": "sammlung",
  "database/shift-types/page.tsx": "sammlung",
  "database/students/page.tsx": "sammlung",
  "database/students/class-list/page.tsx": "sammlung",
  "database/students/ended-care/page.tsx": "sammlung",
  "dateien/page.tsx": "sammlung",
  "eltern/bankverbindungen/page.tsx": "sammlung",
  "emergency/page.tsx": "sammlung",
  "enrollment-phases/page.tsx": "sammlung",
  "info-displays/page.tsx": "sammlung",
  "messages/page.tsx": "sammlung",
  "ogs-groups/page.tsx": "sammlung",
  "parent-announcements/page.tsx": "sammlung",
  "reminders/page.tsx": "sammlung",
  "rooms/page.tsx": "sammlung",
  "rooms/unterwegs/page.tsx": "sammlung",
  "staff/page.tsx": "sammlung",
  "students/search/page.tsx": "sammlung",
  "tagesinformationen/page.tsx": "sammlung",
  "team-chat/page.tsx": "sammlung",

  // Bauart 2 — Objekt: ein einzelnes Ding unter einer Route. Jede Route mit
  // dynamischem Segment gehört hierher; das Profil ist das eigene Konto.
  "admin/enrollments/[id]/page.tsx": "objekt",
  "admin/enrollments/change-requests/[id]/page.tsx": "objekt",
  "admin/enrollments/phases/[phaseId]/page.tsx": "objekt",
  "enrollment-phases/[id]/review/page.tsx": "objekt",
  "enrollment-phases/[id]/rollover/page.tsx": "objekt",
  "messages/[threadId]/page.tsx": "objekt",
  "parent-announcements/[id]/page.tsx": "objekt",
  "profile/page.tsx": "objekt",
  "rooms/[id]/page.tsx": "objekt",
  "staff/[id]/page.tsx": "objekt",
  "students/[id]/page.tsx": "objekt",
  "students/[id]/change-history/page.tsx": "objekt",
  "students/[id]/feedback-history/page.tsx": "objekt",
  "students/[id]/room-history/page.tsx": "objekt",
  "team-chat/[threadID]/page.tsx": "objekt",

  // Bauart 3 — Werkzeug: Flächen, auf denen über Zeit oder in Schritten
  // gearbeitet wird (Pläne, Auswertungen, Importe, Jahrgangswechsel).
  "active-supervisions/page.tsx": "werkzeug",
  "betreuungsplan/page.tsx": "werkzeug",
  "calendar/page.tsx": "werkzeug",
  "database/grade-transitions/page.tsx": "werkzeug",
  "database/personal/import/page.tsx": "werkzeug",
  "database/personal/opening-balances/page.tsx": "werkzeug",
  "database/students/class-list/import/page.tsx": "werkzeug",
  "database/students/import/page.tsx": "werkzeug",
  "day-log/page.tsx": "werkzeug",
  "dienstplan/page.tsx": "werkzeug",
  "enrollment-form/page.tsx": "werkzeug",
  "lists/page.tsx": "werkzeug",
  "meal-plan/page.tsx": "werkzeug",
  "payroll/page.tsx": "werkzeug",
  "statistics/page.tsx": "werkzeug",
  "substitutions/page.tsx": "werkzeug",
  "tagesplan/page.tsx": "werkzeug",
  "time-tracking/page.tsx": "werkzeug",
  "vertretung/page.tsx": "werkzeug",

  // Bauart 4 — Einstellungen: Konfiguration einer Schule.
  "settings/page.tsx": "einstellungen",
};

/**
 * Seiten außerhalb der vier Bauarten, mit Begründung. Die Liste ist
 * shrink-only: wer hier etwas hinzufügen will, ändert BAUARTEN-SPEC.md.
 * Eine Weiterleitung muss als solche im Quelltext erkennbar sein.
 */
const OHNE_BAUART: Readonly<Record<string, string>> = {
  "admin/change-requests/page.tsx": "Weiterleitung nach /anfragen (#2429)",
  "admin/enrollments/change-requests/page.tsx":
    "Weiterleitung nach /anfragen (#2435)",
  "dashboard/page.tsx": "Weiterleitung auf die Startseite (#2180)",
  "invitations/page.tsx": "Weiterleitung nach /database/personal",
  "planung/page.tsx": "Weiterleitung auf die Planungsbereiche (#1886)",
  "staff/dienstplan/page.tsx": "Weiterleitung nach /dienstplan",
  "timetables/page.tsx": "Weiterleitung nach /betreuungsplan",
  "vertretungsplan/page.tsx": "Weiterleitung nach /vertretung",
  "home/page.tsx":
    "Startseite aus Bausteinen (ADR 0008, #2180): keine Sammlung, kein Werkzeug",
  "database/page.tsx":
    "Index der Datenverwaltung: Navigationskacheln zu den Sammlungen",
  "database/exports/page.tsx":
    "Export-Übersicht: Kacheln lösen Downloads aus, keine Fläche über Zeit",
};

function pageId(page: { id: string }): string {
  return page.id.split(path.sep).join("/");
}

describe("Bauart-Deklaration des Tenant-Portals", () => {
  const declared = new Set([
    ...Object.keys(BAUART),
    ...Object.keys(OHNE_BAUART),
  ]);
  const existing = new Set(pages.map(pageId));

  it("deklariert jede Seite genau einmal", () => {
    const undeclared = [...existing].filter((id) => !declared.has(id)).sort();
    expect(
      undeclared,
      "Seiten ohne Bauart — Eintrag in BAUART (oder mit Begründung in OHNE_BAUART) ergänzen",
    ).toEqual([]);

    const twice = Object.keys(BAUART).filter((id) => id in OHNE_BAUART);
    expect(twice, "Seiten in BAUART und OHNE_BAUART zugleich").toEqual([]);
  });

  it("trägt keinen Eintrag ohne Seite", () => {
    const stale = [...declared].filter((id) => !existing.has(id)).sort();
    expect(stale, "Einträge ohne page.tsx — Zuordnung aufräumen").toEqual([]);
  });

  it.each(
    pages
      .filter((page) => /\[[^\]]+\]/.test(pageId(page)))
      .map((page) => [pageId(page)] as const),
  )("%s ist als Objektansicht deklariert (dynamisches Segment)", (id) => {
    expect(BAUART[id]).toBe("objekt");
  });

  it.each(
    Object.entries(OHNE_BAUART)
      .filter(([, grund]) => grund.startsWith("Weiterleitung"))
      .map(([id]) => [id] as const),
  )("%s ist im Quelltext eine Weiterleitung", (id) => {
    const page = pages.find((candidate) => pageId(candidate) === id);
    expect(page).toBeDefined();
    expect(/\bredirect\(|Redirect\b|router\.replace\(/.test(page!.source)).toBe(
      true,
    );
  });

  it("kennt nur die vier Bauarten der Spec", () => {
    const allowed = new Set<Bauart>([
      "sammlung",
      "objekt",
      "werkzeug",
      "einstellungen",
    ]);
    for (const bauart of Object.values(BAUART)) {
      expect(allowed.has(bauart)).toBe(true);
    }
  });
});

describe("Seitengerüst des Tenant-Portals", () => {
  it("findet Seiten zum Prüfen", () => {
    expect(pages.length).toBeGreaterThan(50);
  });

  it.each(pages.map((page) => [page.id, page] as const))(
    "%s baut kein eigenes Layout",
    (_id, page) => {
      const verstoesse: string[] = [];
      if (/<h1\b/.test(page.source)) verstoesse.push("eigene <h1>");
      if (/<main\b/.test(page.source)) verstoesse.push("eigenes <main>");
      // Bewusst NICHT geprüft: `max-w-` und `mx-auto`. Beides ist innerhalb
      // des Inhalts normales Layout (ein zentrierter Monatsumschalter, eine
      // schmale Karte, ein Dialog). Textuell lässt sich das nicht von einer
      // Seite unterscheiden, die sich als Ganzes zentriert — und ein Gate,
      // das zu Umbauten am falschen Ort zwingt, richtet mehr Schaden an als
      // es verhindert. Die Seitenbreite deckt stattdessen die Prüfung unten
      // ab: wer `TenantPage` als Wurzel rendert, bekommt die Breite aus der
      // Shell.
      if (/\bkicker[=:]/.test(page.source))
        verstoesse.push("Mini-Überschrift (kicker)");
      // Der Seitenrumpf mit eigenem Rhythmus. Genau diese Klassenfolge war
      // das alte, handgepflegte Gerüst; heute liefert `TenantPage` sie. Eine
      // Seite, die sie noch selbst setzt, rendert das Gerüst nicht als
      // Wurzel — auch dann nicht, wenn sie es in einem Ladezweig importiert.
      if (/className="w-full space-y-6"/.test(page.source))
        verstoesse.push("eigener Seitenrumpf (w-full space-y-6)");

      expect(verstoesse, `${page.id}: ${verstoesse.join(", ")}`).toEqual([]);
    },
  );

  it.each(
    pages
      .filter((page) => rendersOwnMarkup(page.source))
      .map((page) => [page.id, page] as const),
  )("%s rendert TenantPage als Wurzel", (_id, page) => {
    // `DatabasePageLayout` ist der einzige zugelassene Adapter: er rendert
    // selbst `TenantPage` und ergänzt nur das Master-Detail-Skelett.
    const nutztGeruest =
      /import\s*\{[^}]*\bTenantPage\b[^}]*\}\s*from\s*"~\/components\/ui\/tenant-page"/.test(
        page.source,
      ) || /<DatabasePageLayout\b/.test(page.source);

    expect(nutztGeruest, `${page.id} baut Markup ohne TenantPage`).toBe(true);
  });
});
