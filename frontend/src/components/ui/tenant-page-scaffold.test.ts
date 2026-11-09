import { readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import { describe, expect, it } from "vitest";

// Gate für das Seitengerüst (siehe .claude/rules/frontend-ui-kit.md und
// components/ui/TENANT-PAGE-SPEC.md): jede Seite des Tenant-, Träger-, Schul-
// und Operator-Portals rendert `TenantPage` und baut kein eigenes Layout. Der Test liest Quelltext, nicht das DOM — er
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
  "database/qualification-types/page.tsx": "sammlung",
  "database/qualifications/page.tsx": "sammlung",
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
  "messages/groups/page.tsx": "sammlung",
  "ogs-groups/page.tsx": "sammlung",
  "ogs-groups/overview/page.tsx": "sammlung",
  "parent-announcements/page.tsx": "sammlung",
  "umfragen/page.tsx": "sammlung",
  "wochenprotokolle/page.tsx": "sammlung",
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
  "messages/groups/[groupThreadId]/page.tsx": "objekt",
  "parent-announcements/[id]/page.tsx": "objekt",
  "umfragen/[id]/page.tsx": "objekt",
  "wochenprotokolle/[protocolId]/page.tsx": "objekt",
  "profile/page.tsx": "objekt",
  "rooms/[id]/page.tsx": "objekt",
  "staff/[id]/page.tsx": "objekt",
  "students/[id]/page.tsx": "objekt",
  "students/[id]/change-history/page.tsx": "objekt",
  "students/[id]/unfallanzeigen/[reportId]/page.tsx": "objekt",
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
  "ferienwochen/page.tsx": "werkzeug",
  "lists/page.tsx": "werkzeug",
  "meal-plan/freigabe/page.tsx": "werkzeug",
  "meal-plan/page.tsx": "werkzeug",
  "mein-dienstplan/page.tsx": "werkzeug",
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

/**
 * Das Träger-Portal (#5146) baut aus denselben Bausteinen wie das
 * Tenant-Portal und rendert deshalb dasselbe Gerüst. Ausgenommen sind nur die
 * Seiten vor der Anmeldung: Anmelden und Einladung annehmen haben keine
 * Portalhülle und keinen Seitenkopf, sie sind Formulare auf leerem Grund wie
 * die Anmeldeseiten der übrigen Portale.
 */
const TRAEGER_DIR = path.join(SRC_DIR, "app", "traeger");
const TRAEGER_PUBLIC = new Set(["login/page.tsx", "invite/page.tsx"]);

const traegerPages = collectPageFiles(TRAEGER_DIR)
  .map((file) => ({
    id: path.relative(TRAEGER_DIR, file),
    file,
    source: readFileSync(file, "utf8"),
  }))
  .filter((page) => !TRAEGER_PUBLIC.has(pageId(page)));

/**
 * Schul- und Operator-Portal rendern seit #5604 dasselbe Gerüst. Ausgenommen
 * sind auch hier nur die Seiten vor der Anmeldung; sie haben keine Portalhülle.
 */
function portalPages(portal: string, publicPages: ReadonlySet<string>) {
  const directory = path.join(SRC_DIR, "app", portal);
  return collectPageFiles(directory)
    .map((file) => ({
      id: path.relative(directory, file),
      file,
      source: readFileSync(file, "utf8"),
    }))
    .filter((page) => !publicPages.has(pageId(page)));
}

const schoolPages = portalPages(
  "school",
  new Set(["login/page.tsx", "invite/page.tsx", "reset-password/page.tsx"]),
);
const operatorPages = portalPages(
  "operator",
  new Set(["login/page.tsx", "invite/page.tsx", "email-confirm/page.tsx"]),
);

/**
 * Schulseiten, die nur eine View rendern, tragen das Gerüst in dieser View.
 * Der Markup-Filter unten sieht sie nicht; deshalb stehen sie hier einzeln.
 */
const SCHOOL_VIEWS = [
  "components/class-day/class-day-overview.tsx",
  "components/class-day/class-day-class.tsx",
  "components/school/supervisions/supervisions-view.tsx",
  "components/messaging/team-chat-frames.tsx",
];

describe("Views des Schul-Portals", () => {
  it.each(SCHOOL_VIEWS)("%s rendert TenantPage", (file) => {
    const source = readFileSync(path.join(SRC_DIR, file), "utf8");
    expect(source).toMatch(/<TenantPage\b/);
    expect(source).not.toMatch(/<h1\b/);
  });
});

describe.each([
  { portal: "Tenant-Portals", pages, minimum: 51 },
  { portal: "Träger-Portals", pages: traegerPages, minimum: 13 },
  { portal: "Schul-Portals", pages: schoolPages, minimum: 13 },
  { portal: "Operator-Portals", pages: operatorPages, minimum: 16 },
])("Seitengerüst des $portal", ({ pages, minimum }) => {
  it("findet Seiten zum Prüfen", () => {
    expect(pages.length).toBeGreaterThanOrEqual(minimum);
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

// --- Statuszeile und Kopfaktionen (Spec R3 und R4, #5605) ---------------------

/** Die Gerüste, deren Kopf eine Seite füllt. */
const SHELLS = new Set(["TenantPage", "DatabasePageLayout"]);
/** Bauteile, die im Kopf als Text sichtbar sind; ein Symbolknopf zählt nicht. */
const TEXT_ACTIONS = new Set([
  "Button",
  "ButtonLink",
  "DatabaseCreateAction",
  "ExportMenu",
]);
/** Länger ist ein fester Text in `stats` keine Zahl mehr, sondern ein Satz. */
const MAX_STATS_LITERAL = 40;

function isIconSized(attributes: ts.JsxAttributes): boolean {
  return attributes.properties.some(
    (attribute) =>
      ts.isJsxAttribute(attribute) &&
      attribute.name.getText() === "size" &&
      attribute.initializer?.getText().includes("icon") === true,
  );
}

/** Konstanten einer Datei: `actions={headActions}` zählt wie inline. */
type LocalConstants = ReadonlyMap<string, ts.Expression>;

function collectLocalConstants(file: ts.SourceFile): LocalConstants {
  const constants = new Map<string, ts.Expression>();
  const visit = (node: ts.Node) => {
    if (
      ts.isVariableDeclaration(node) &&
      ts.isIdentifier(node.name) &&
      node.initializer &&
      ts.getCombinedNodeFlags(node) & ts.NodeFlags.Const
    ) {
      constants.set(node.name.text, node.initializer);
    }
    ts.forEachChild(node, visit);
  };
  visit(file);
  return constants;
}

/** Sichtbare Text-Aktionen; bei Verzweigungen zählt der vollere Zweig. */
function countTextActions(
  node: ts.Node | undefined,
  constants: LocalConstants,
  seen: ReadonlySet<string> = new Set(),
): number {
  if (!node) return 0;
  if (ts.isIdentifier(node)) {
    if (seen.has(node.text)) return 0;
    return countTextActions(
      constants.get(node.text),
      constants,
      new Set([...seen, node.text]),
    );
  }
  if (ts.isJsxExpression(node) || ts.isParenthesizedExpression(node)) {
    return countTextActions(node.expression, constants, seen);
  }
  if (ts.isConditionalExpression(node)) {
    return Math.max(
      countTextActions(node.whenTrue, constants, seen),
      countTextActions(node.whenFalse, constants, seen),
    );
  }
  if (ts.isBinaryExpression(node)) {
    const operator = node.operatorToken.kind;
    if (operator === ts.SyntaxKind.AmpersandAmpersandToken) {
      return countTextActions(node.right, constants, seen);
    }
    if (
      operator === ts.SyntaxKind.BarBarToken ||
      operator === ts.SyntaxKind.QuestionQuestionToken
    ) {
      return Math.max(
        countTextActions(node.left, constants, seen),
        countTextActions(node.right, constants, seen),
      );
    }
    return 0;
  }
  if (ts.isJsxSelfClosingElement(node)) {
    return TEXT_ACTIONS.has(node.tagName.getText()) &&
      !isIconSized(node.attributes)
      ? 1
      : 0;
  }
  if (ts.isJsxElement(node)) {
    const opening = node.openingElement;
    if (TEXT_ACTIONS.has(opening.tagName.getText())) {
      return isIconSized(opening.attributes) ? 0 : 1;
    }
  }
  if (ts.isJsxElement(node) || ts.isJsxFragment(node)) {
    return node.children.reduce(
      (sum, child) => sum + countTextActions(child, constants, seen),
      0,
    );
  }
  return 0;
}

/** Feste Texte, die `stats` je Zweig zeigen kann. */
function statsLiterals(
  node: ts.Node | undefined,
  constants: LocalConstants,
  seen: ReadonlySet<string> = new Set(),
): string[] {
  if (!node) return [];
  if (ts.isIdentifier(node)) {
    if (seen.has(node.text)) return [];
    return statsLiterals(
      constants.get(node.text),
      constants,
      new Set([...seen, node.text]),
    );
  }
  if (ts.isJsxExpression(node) || ts.isParenthesizedExpression(node)) {
    return statsLiterals(node.expression, constants, seen);
  }
  if (ts.isStringLiteralLike(node)) return [node.text];
  if (ts.isConditionalExpression(node)) {
    return [
      ...statsLiterals(node.whenTrue, constants, seen),
      ...statsLiterals(node.whenFalse, constants, seen),
    ];
  }
  return [];
}

function introProperty(
  attribute: ts.JsxAttribute,
  name: string,
): ts.Expression | undefined {
  const value = attribute.initializer;
  if (!value || !ts.isJsxExpression(value)) return undefined;
  const { expression } = value;
  if (!expression || !ts.isObjectLiteralExpression(expression)) {
    return undefined;
  }
  for (const property of expression.properties) {
    if (ts.isPropertyAssignment(property) && property.name.getText() === name) {
      return property.initializer;
    }
  }
  return undefined;
}

/** Prüft jeden Seitenkopf einer Datei gegen Spec R3 und R4. */
function headViolations(source: string): string[] {
  const file = ts.createSourceFile(
    "probe.tsx",
    source,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  const violations: string[] = [];
  const constants = collectLocalConstants(file);
  const checkActions = (node: ts.Node | undefined) => {
    const count = countTextActions(node, constants);
    if (count > 1) violations.push(`${count} sichtbare Text-Aktionen`);
  };
  const checkStats = (node: ts.Node | undefined) => {
    for (const text of statsLiterals(node, constants)) {
      if (text.length > MAX_STATS_LITERAL) {
        violations.push(`Satz in der Statuszeile: „${text}“`);
      }
    }
  };
  const visit = (node: ts.Node) => {
    if (
      ts.isJsxOpeningLikeElement(node) &&
      SHELLS.has(node.tagName.getText())
    ) {
      for (const attribute of node.attributes.properties) {
        if (!ts.isJsxAttribute(attribute)) continue;
        const name = attribute.name.getText();
        if (name === "actions") checkActions(attribute.initializer);
        if (name === "stats") checkStats(attribute.initializer);
        if (name === "intro") {
          checkActions(introProperty(attribute, "actions"));
          checkStats(introProperty(attribute, "description"));
        }
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(file);
  return violations;
}

/**
 * Köpfe, die bewusst mehr als eine Text-Aktion zeigen. Shrink-only; ein neuer
 * Eintrag braucht die Zustimmung im PR (Regel „When to deviate“).
 */
const MEHR_ALS_EINE_AKTION: Readonly<Record<string, string>> = {
  "app/[tenant]/(protected)/home/page.tsx":
    "Bearbeitungszustand: „Abbrechen“ und „Fertig“ sind eine Entscheidung, wie EditActions; vom Product Owner am 08.11.2026 freigegeben (#5605)",
};

function collectTsxFiles(directory: string): string[] {
  return readdirSync(directory).flatMap((entry) => {
    const entryPath = path.join(directory, entry);
    if (statSync(entryPath).isDirectory()) return collectTsxFiles(entryPath);
    return entry.endsWith(".tsx") && !/\.(test|stories)\.tsx$/.test(entry)
      ? [entryPath]
      : [];
  });
}

// Das Träger-Portal trägt noch mehrere Kopfaktionen; dieser Test gilt dem
// Tenant-Portal (#5605). Seitenköpfe stehen auch in View-Komponenten.
const tenantHeads = [
  ...collectTsxFiles(PROTECTED_DIR),
  ...collectTsxFiles(path.join(SRC_DIR, "components")).filter(
    (file) => !file.includes(`${path.sep}traeger${path.sep}`),
  ),
]
  .map((file) => ({
    id: path.relative(SRC_DIR, file).split(path.sep).join("/"),
    source: readFileSync(file, "utf8"),
  }))
  .filter((head) => /<(?:TenantPage|DatabasePageLayout)\b/.test(head.source));

describe("Seitenkopf des Tenant-Portals (Spec R3 und R4)", () => {
  it("meldet einen langen festen Text in stats, auch in einem Zweig", () => {
    expect(
      headViolations(
        `<TenantPage title="A" stats={editing ? "Karte ziehen, um sie zu verschieben und zu ändern." : line} />`,
      ),
    ).toEqual([
      "Satz in der Statuszeile: „Karte ziehen, um sie zu verschieben und zu ändern.“",
    ]);
  });

  it("meldet einen langen festen Text in einer lokalen Konstante", () => {
    expect(
      headViolations(`
        const statusLine = "Karte ziehen, um sie zu verschieben und zu ändern.";
        const page = <TenantPage title="A" stats={statusLine} />;
      `),
    ).toEqual([
      "Satz in der Statuszeile: „Karte ziehen, um sie zu verschieben und zu ändern.“",
    ]);
  });

  it("lässt echte Zahlen und kurze Zustände in stats zu", () => {
    expect(
      headViolations(
        '<TenantPage title="A" stats={group ? `${present} von ${total} da` : "Keine Gruppe zugeordnet"} />',
      ),
    ).toEqual([]);
  });

  it("meldet mehr als eine sichtbare Text-Aktion, auch im intro", () => {
    expect(
      headViolations(`<TenantPage title="A" actions={
        <>
          <ButtonLink href="/import">Importieren</ButtonLink>
          {can && <DatabaseCreateAction label="Neu" />}
        </>
      } />`),
    ).toEqual(["2 sichtbare Text-Aktionen"]);
    expect(
      headViolations(`<DatabasePageLayout intro={{
        title: "A",
        actions: <><Button>Eins</Button><ExportMenu options={[]} /></>,
      }} />`),
    ).toEqual(["2 sichtbare Text-Aktionen"]);
  });

  it("folgt einer Konstante in actions", () => {
    expect(
      headViolations(`
        const headActions = <><Button>Eins</Button><Button>Zwei</Button></>;
        const page = <TenantPage title="A" actions={headActions} />;
      `),
    ).toEqual(["2 sichtbare Text-Aktionen"]);
  });

  it("zählt Symbolknöpfe, Menüs und einander ausschließende Zweige nicht", () => {
    expect(
      headViolations(`<TenantPage title="A" actions={
        <>
          <Button size="icon" aria-label="Drucken" />
          <OverflowMenu items={[]} />
          <DatabaseGroupingToggle value="a" />
          {editing ? <Button>Speichern</Button> : <Button>Bearbeiten</Button>}
        </>
      } />`),
    ).toEqual([]);
  });

  it("findet Seitenköpfe zum Prüfen", () => {
    expect(tenantHeads.length).toBeGreaterThanOrEqual(51);
  });

  it.each(tenantHeads.map((head) => [head.id, head] as const))(
    "%s hält Statuszeile und Aktionen ein",
    (id, head) => {
      // Die Ausnahme deckt genau das eine Paar, keine dritte Aktion.
      const violations = headViolations(head.source).filter(
        (violation) =>
          !(id in MEHR_ALS_EINE_AKTION) ||
          violation !== "2 sichtbare Text-Aktionen",
      );
      expect(violations, id).toEqual([]);
    },
  );

  it("führt keine Ausnahme ohne mehrere Aktionen", () => {
    const stale = Object.keys(MEHR_ALS_EINE_AKTION).filter((id) => {
      const head = tenantHeads.find((candidate) => candidate.id === id);
      return !head || headViolations(head.source).length === 0;
    });
    expect(stale).toEqual([]);
  });
});

// --- Ladezustand als Text (#5588) ---------------------------------------------

const APP_DIR = path.join(SRC_DIR, "app");
const LOADING_TEXT_RE =
  /\b(?:wird|werden) geladen\b|\bLädt\b|\bLaden(?:\.\.\.|…| …)/i;
/** Hier ist der Text die Ansage eines Kit-Bausteins, kein Eigenbau. */
const LOADING_ANNOUNCEMENT_NAMES = new Set([
  "aria-label",
  "ariaLabel",
  "loadingLabel",
  "loadingText",
  "placeholder",
]);
/** `label` und `message` sind nur an diesen Bausteinen die Ansage. */
const LOADING_ANNOUNCEMENT_ELEMENTS = new Map([
  ["Loading", "message"],
  ["SkeletonRegion", "label"],
]);

function isLoadingAnnouncement(owner: ts.Node): boolean {
  if (
    ts.isPropertyAssignment(owner) ||
    ts.isVariableDeclaration(owner) ||
    ts.isJsxAttribute(owner)
  ) {
    if (LOADING_ANNOUNCEMENT_NAMES.has(owner.name.getText())) return true;
  }
  if (!ts.isJsxAttribute(owner)) return false;
  const element = owner.parent.parent;
  return (
    LOADING_ANNOUNCEMENT_ELEMENTS.get(element.tagName.getText()) ===
    owner.name.getText()
  );
}

function isInsideScreenReaderText(node: ts.Node): boolean {
  for (let current = node.parent; current; current = current.parent) {
    if (!ts.isJsxElement(current)) continue;
    const className = current.openingElement.attributes.properties.find(
      (attribute) =>
        ts.isJsxAttribute(attribute) &&
        attribute.name.getText() === "className",
    );
    if (className?.getText().includes("sr-only")) return true;
  }
  return false;
}

/** Sichtbare Ladetexte einer Datei; Kommentare sieht der Parser nicht als Text. */
function visibleLoadingTexts(file: string, source: string): string[] {
  const sourceFile = ts.createSourceFile(
    file,
    source,
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TSX,
  );
  const found: string[] = [];
  const visit = (node: ts.Node) => {
    const isText =
      ts.isJsxText(node) ||
      ts.isStringLiteral(node) ||
      ts.isNoSubstitutionTemplateLiteral(node);
    if (isText && LOADING_TEXT_RE.test(node.text)) {
      let owner: ts.Node = node.parent;
      while (
        ts.isJsxExpression(owner) ||
        ts.isConditionalExpression(owner) ||
        ts.isParenthesizedExpression(owner)
      ) {
        owner = owner.parent;
      }
      if (!isLoadingAnnouncement(owner) && !isInsideScreenReaderText(node)) {
        const { line } = sourceFile.getLineAndCharacterOfPosition(
          node.getStart(),
        );
        found.push(`${line + 1}: ${node.text.trim()}`);
      }
    }
    ts.forEachChild(node, visit);
  };
  visit(sourceFile);
  return found;
}

describe("Ladezustand in Seitendateien", () => {
  const pageFiles = collectPageFiles(APP_DIR);

  it("findet die Seiten aller Portale", () => {
    expect(pageFiles.length).toBeGreaterThan(100);
  });

  it.each(pageFiles.map((file) => [path.relative(APP_DIR, file), file]))(
    "%s zeigt keinen Ladetext von Hand",
    (_id, file) => {
      // `Loading`, `SkeletonRegion`, `TenantPage loading` oder `Button isLoading`
      // tragen die Ansage; ein grauer Satz sagt nicht, dass etwas passiert.
      expect(visibleLoadingTexts(file, readFileSync(file, "utf8"))).toEqual([]);
    },
  );

  it("schlägt auf Ladetext an und lässt die Ansage der Kit-Bausteine durch", () => {
    expect(
      visibleLoadingTexts(
        "probe.tsx",
        `const a = <p>Wird geladen…</p>;
         const b = <SectionCard description="Lädt…" />;
         const c = busy ? "Laden..." : "Weiter";
         const d = <StatusBadge label="Vorschau wird geladen" />;
         const e = { label: "Vorschau wird geladen" };`,
      ),
    ).toHaveLength(5);
    expect(
      visibleLoadingTexts(
        "probe.tsx",
        `// Wird geladen…
         const a = <Loading message="Wird geladen…" />;
         const b = <SkeletonRegion label="Liste wird geladen" />;
         const c = <Button loadingText="Lädt …" />;
         const d = <span className="sr-only">wird geladen</span>;
         const e = <div aria-label="Wird geladen" />;`,
      ),
    ).toEqual([]);
  });
});
