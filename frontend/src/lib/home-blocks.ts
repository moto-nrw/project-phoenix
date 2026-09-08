/**
 * Baustein-Register der Startseite (#2875, modular seit #2180).
 *
 * Die Startseite ist keine feste Bildschirmfläche, sondern ein Brett aus
 * Bausteinen, das jede Person selbst zusammenstellt: welche Bausteine, in
 * welcher Reihenfolge, wie breit. Wer nichts anfasst, sieht die
 * Standardansicht ihrer Rolle.
 *
 * Vier Ebenen entscheiden, in dieser Reihenfolge:
 *
 *   1. Berechtigung   — was die Person abrufen darf. Jede Regel spiegelt das
 *                       Gate des Endpunkts hinter dem Baustein, damit eine
 *                       frei angelegte Rolle ohne Codeänderung eine brauchbare
 *                       Startseite bekommt.
 *   2. Verfügbarkeit  — Betriebsmodus der Schule. Was hier wegfällt, gibt es
 *                       für diese Schule schlicht nicht.
 *   3. Vorgabe        — die Einrichtung kann einen Baustein verpflichtend
 *                       machen oder ganz abschalten.
 *   4. Eigene Anordnung — alles, was danach noch offen ist.
 *
 * Der Katalog lebt bewusst hier und nicht im Backend: nur das Frontend kennt
 * Beschriftung, Betriebsmodus, Datenquelle und Größe eines Bausteins. Der
 * Server prüft Schlüssel und Breite nur auf ihre Form.
 *
 * Gespeichert werden ZWEI Dinge, beide als Abweichung vom Standard: die
 * ANORDNUNG (Reihenfolge und Breite dessen, was die Person platziert hat) und
 * die ENTFERNTEN Bausteine. Ein später ergänzter Baustein steht in keinem von
 * beiden — er erscheint deshalb bei bestehenden Konten am Ende der Fläche,
 * statt für alle zu verschwinden, die den Dialog je geöffnet haben.
 */

import type { MotoConceptKey } from "~/lib/moto-concepts";

/** Was die Einrichtung für einen Baustein vorgibt. */
export type HomeBlockPolicy = "optional" | "required" | "disabled";

/** Abweichungen der Person: false = entfernt. */
export type HomeLayoutOverrides = Record<string, boolean>;

/** Vorgaben der Schule. Ein fehlender Eintrag heisst "frei wählbar". */
export type HomeBlockPolicies = Record<string, HomeBlockPolicy>;

type HomeBlockKind = "tile" | "section";

/**
 * Breite eines Bausteins in Spalten des vierspaltigen Rasters.
 *
 * Drei Stufen statt freier Größe: eine Kennzahl ist immer schmal, eine Liste
 * schmal, breit oder über die volle Breite. So kann sich jede Person die
 * Fläche bauen, ohne dass sie zerfällt.
 */
export type HomeBlockSpan = 1 | 2 | 4;

/**
 * Standardansicht, aus der jemand startet. Leitet sich aus Rechten ab.
 *
 * `lead_care` ist die Vereinigung: eine Person, die die Einrichtung führt UND
 * selbst betreut, bekommt beides — den eigenen Tag vorneweg und die Lage der
 * Schule dahinter.
 */
export type HomeProfile = "care" | "lead" | "lead_care";

/**
 * Was die Person darf — genau so viel, wie der Katalog zum Entscheiden
 * braucht. `has` prüft ein einzelnes Tenant-Recht, `isAdminScope` steht für
 * den Adminzuschnitt.
 */
export interface HomeBlockAccess {
  readonly isAdminScope: boolean;
  readonly has: (permission: string) => boolean;
  /**
   * Das Anfragen-Modul hat eine zusammengesetzte Regel über sechs Rechte, die
   * an vier Stellen gleich lauten muss. Sie kommt deshalb fertig aus
   * `change-request-access.ts` statt hier ein siebtes Mal zu stehen.
   */
  readonly canOpenRequestsPage: boolean;
  /**
   * Betreut die Person selbst (Rolle mit Betreuungszuschnitt)? Entscheidet
   * nicht über Rechte, sondern darüber, ob der eigene Tag zur Standardansicht
   * gehört: ein reines Adminkonto hat keine Einsätze, die man ihm zeigen
   * könnte.
   */
  readonly caresForGroups: boolean;
  /**
   * Hat die Person heute mindestens eine eigene Betreuungsgruppe (auch in
   * Vertretung)? Kommt aus der Sitzung, nicht aus einem Recht: „Meine Gruppe"
   * ohne Gruppe wäre eine Karte, die nur sagt, dass sie leer ist.
   */
  readonly hasOwnGroups: boolean;
}

/** Der Betriebsmodus der Schule, ohne Session- oder Kontext-Typen. */
interface HomeBlockModeContext {
  /** Anwesenheitsmodus "detailed" (Räume, Wege) statt "binary" (da/nicht da). */
  readonly detailed: boolean;
  /** Offene Betreuung ohne feste Gruppen (#1544). */
  readonly openCareGroupMode: boolean;
  readonly nfcEnabled: boolean;
  /** Geburtstage sind pro Schule abschaltbar und kommen vom Server. */
  readonly birthdaysEnabled: boolean;
  /** Der Betreuungsplan ist pro Schule abschaltbar (#2383). */
  readonly timetableEnabled: boolean;
  /**
   * Erinnerungen sind pro Schule abschaltbar und stehen standardmäßig aus
   * (#1457). Ob eine Art eingeschaltet ist, steht erst in der Antwort des
   * Servers; die Startseite reicht es hierher.
   */
  readonly remindersEnabled: boolean;
  /** Eltern-Nachrichten sind pro Schule einschaltbar (Elternportal). */
  readonly messagingEnabled: boolean;
  /** Der Team-Chat ist pro Schule einschaltbar und steht standardmäßig aus. */
  readonly staffMessagingEnabled: boolean;
}

/** Betriebsmodus plus die Rechte der angemeldeten Person (#2180). */
export interface HomeBlockContext extends HomeBlockModeContext {
  readonly access: HomeBlockAccess;
}

export interface HomeBlockDefinition {
  readonly key: HomeBlockKey;
  readonly kind: HomeBlockKind;
  /** Name auf der Karte und im Hinzufügen-Menü. */
  readonly label: string;
  /** Ein Satz, was der Baustein zeigt. */
  readonly description: string;
  /** Symbol aus dem Begriffs-Register; trägt die Karte im Anpassen-Modus. */
  readonly concept: MotoConceptKey;
  /** Erlaubte Breiten, von schmal nach breit. */
  readonly spans: readonly HomeBlockSpan[];
  /**
   * Darf die Person die Daten des Bausteins abrufen? Spiegelt das Gate des
   * Endpunkts dahinter; ohne das Recht liefert der Server ohnehin nichts.
   */
  readonly permitted: (access: HomeBlockAccess) => boolean;
  /** Gibt es den Baustein in dieser Schule? Reiner Betriebsmodus. */
  readonly available: (ctx: HomeBlockModeContext) => boolean;
}

export type HomeBlockKey =
  | "tile.students_present"
  | "tile.students_in_rooms"
  | "tile.students_in_transit"
  | "tile.students_on_playground"
  | "tile.students_sick"
  | "tile.students_excused"
  | "tile.students_home"
  | "tile.active_activities"
  | "tile.capacity_utilization"
  | "section.birthdays"
  | "section.recent_activity"
  | "section.current_activities"
  | "section.active_groups"
  | "section.my_day"
  | "section.my_group"
  | "section.staff_notices"
  | "section.day_flow"
  | "section.open_requests"
  | "section.reminders"
  | "section.staff_today"
  | "section.messages";

const always = () => true;
const roomSurfaces = (ctx: HomeBlockModeContext) => ctx.detailed;
const activitySurfaces = (ctx: HomeBlockModeContext) =>
  ctx.detailed && ctx.nfcEnabled;

// Die Rechte, an denen die Endpunkte der Bausteine hängen. Namen wie im
// Backend (auth/authorize/permissions), damit ein Vergleich möglich bleibt.
const PERMISSION = {
  /** GET /api/active/analytics/dashboard */
  analytics: "groups:read",
  /**
   * GET /api/birthdays, GET /api/staff-notices/today, GET /api/ogs-group-live
   * und GET /api/staff/dashboard-summary.
   */
  usersRead: "users:read",
  /** GET /api/time-tracking/assignments */
  timeTrackingOwn: "time_tracking:own",
  /** GET /api/timetable/operations/planned-now */
  schedulesRead: "schedules:read",
} as const;

const operationalNumbers = (access: HomeBlockAccess) =>
  access.has(PERMISSION.analytics);
const anyRequestQueue = (access: HomeBlockAccess) => access.canOpenRequestsPage;

/** Eine Kennzahl ist immer schmal, eine Liste hat drei Stufen. */
const TILE_SPANS: readonly HomeBlockSpan[] = [1];
const SECTION_SPANS: readonly HomeBlockSpan[] = [1, 2, 4];

/** Symbol je Kennzahl — dasselbe Vokabular, das die Kachel selbst trägt. */
const TILE_CONCEPT: Record<string, MotoConceptKey> = {
  "tile.students_present": "present",
  "tile.students_in_rooms": "rooms",
  "tile.students_in_transit": "transit",
  "tile.students_on_playground": "schoolyard",
  "tile.students_sick": "sick",
  "tile.students_excused": "excused",
  "tile.students_home": "home",
  "tile.active_activities": "activities",
  "tile.capacity_utilization": "utilization",
};

function tile(
  key: HomeBlockKey,
  label: string,
  description: string,
  available: (ctx: HomeBlockModeContext) => boolean,
): HomeBlockDefinition {
  return {
    key,
    kind: "tile",
    label,
    description,
    concept: TILE_CONCEPT[key] ?? "dashboard",
    spans: TILE_SPANS,
    permitted: operationalNumbers,
    available,
  };
}

export const HOME_BLOCKS: readonly HomeBlockDefinition[] = [
  // ---- Kennzahlen ---------------------------------------------------------
  tile(
    "tile.students_present",
    "Kinder anwesend",
    "Wie viele Kinder gerade eingecheckt sind.",
    always,
  ),
  tile(
    "tile.students_in_rooms",
    "In Räumen",
    "Anwesende Kinder, die gerade in einem Raum sind.",
    roomSurfaces,
  ),
  tile(
    "tile.students_in_transit",
    "Unterwegs",
    "Anwesende Kinder ohne Raum, zum Beispiel auf dem Weg.",
    roomSurfaces,
  ),
  tile(
    "tile.students_on_playground",
    "Schulhof",
    "Kinder, die gerade auf dem Schulhof sind.",
    always,
  ),
  tile(
    "tile.students_sick",
    "Krank",
    "Kinder, die heute krank gemeldet sind.",
    always,
  ),
  tile(
    "tile.students_excused",
    "Entschuldigt",
    "Kinder, die heute entschuldigt fehlen.",
    always,
  ),
  tile(
    "tile.students_home",
    "Zuhause",
    "Kinder, die heute nicht in der Betreuung sind.",
    always,
  ),
  tile(
    "tile.active_activities",
    "Aktive Aktivitäten",
    "Wie viele Aktivitäten gerade laufen.",
    activitySurfaces,
  ),
  tile(
    "tile.capacity_utilization",
    "Auslastung",
    "Belegte Plätze in den Räumen in Prozent.",
    roomSurfaces,
  ),

  // ---- Der eigene Tag -----------------------------------------------------
  {
    key: "section.my_day",
    kind: "section",
    label: "Mein Tag",
    description: "Ihre heutigen Einsätze mit Ort, Zeit und Vertretungen.",
    concept: "carePlan",
    spans: SECTION_SPANS,
    // /api/time-tracking/assignments (#1844) liefert nur die eigenen Blöcke.
    permitted: (access) => access.has(PERMISSION.timeTrackingOwn),
    available: (ctx) => ctx.timetableEnabled,
  },
  {
    key: "section.my_group",
    kind: "section",
    label: "Meine Gruppe heute",
    description:
      "Wie viele Kinder Ihrer Gruppe da sind, wer heute fehlt und wann die nächste Abholung ist.",
    concept: "groups",
    spans: SECTION_SPANS,
    // Dieselbe Quelle wie die Seite „Meine Gruppen": der Server wählt die
    // Gruppe der Person. Ohne eigene Gruppe gibt es nichts zu zeigen.
    permitted: (access) =>
      access.has(PERMISSION.usersRead) && access.hasOwnGroups,
    // Offene Betreuung kennt keine feste Gruppe, und im Anwesenheitsmodus
    // „binary" gibt es die Gruppenansicht nicht.
    available: (ctx) => !ctx.openCareGroupMode && ctx.detailed,
  },
  {
    key: "section.staff_notices",
    kind: "section",
    label: "Tagesinformationen",
    description: "Hinweise der Leitung, die heute gelten.",
    concept: "announcements",
    spans: SECTION_SPANS,
    permitted: (access) => access.has(PERMISSION.usersRead),
    available: always,
  },
  {
    key: "section.reminders",
    kind: "section",
    label: "Erinnerungen",
    description:
      "Was in den nächsten Minuten ansteht oder überfällig ist: Abholungen und Aktivitätsbeginn.",
    concept: "pickup",
    spans: SECTION_SPANS,
    // GET /api/reminders (#1457) hängt an users:read wie die
    // Tagesinformationen.
    permitted: (access) => access.has(PERMISSION.usersRead),
    // Hat die Schule keine Erinnerungsart eingeschaltet, gibt es den Baustein
    // für sie nicht: eine Karte, die dauerhaft „ist ausgeschaltet" sagt,
    // belegt nur einen Platz.
    available: (ctx) => ctx.remindersEnabled,
  },
  {
    key: "section.open_requests",
    kind: "section",
    label: "Offene Anfragen",
    description:
      "Wünsche von Eltern und Anträge des Teams, die auf eine Entscheidung warten.",
    concept: "requests",
    spans: SECTION_SPANS,
    permitted: anyRequestQueue,
    available: always,
  },
  {
    key: "section.messages",
    kind: "section",
    label: "Ungelesene Nachrichten",
    description:
      "Wie viele Nachrichten von Eltern und aus dem Team-Chat noch ungelesen sind.",
    concept: "messages",
    spans: SECTION_SPANS,
    // Beide Zähler hängen am Konto, nicht an einem Recht: die Seitenleiste
    // zeigt Nachrichten jeder Mitarbeiterin, und der Server zählt nur, was
    // die Person lesen darf.
    permitted: always,
    available: (ctx) => ctx.messagingEnabled || ctx.staffMessagingEnabled,
  },

  // ---- Das Team -----------------------------------------------------------
  {
    key: "section.staff_today",
    kind: "section",
    label: "Personal heute",
    description:
      "Wer in Aufsicht ist, wer fehlt und ob offene Anträge auf die Leitung warten.",
    concept: "staff",
    spans: SECTION_SPANS,
    // GET /api/staff/dashboard-summary hängt an users:read; die Zahl der
    // Kräfte in Aufsicht kommt aus den Betriebszahlen und fehlt ohne
    // groups:read schlicht.
    permitted: (access) => access.has(PERMISSION.usersRead),
    available: always,
  },

  // ---- Der laufende Betrieb ----------------------------------------------
  {
    key: "section.day_flow",
    kind: "section",
    label: "Ablauf des Tages",
    description:
      "Die Blöcke des Betreuungsplans, die gerade laufen oder als Nächstes anstehen.",
    concept: "carePlan",
    spans: SECTION_SPANS,
    permitted: (access) => access.has(PERMISSION.schedulesRead),
    // Ohne Betreuungsplan gibt es keinen Ablauf, und im Anwesenheitsmodus
    // "binary" plant die Schule keine Blöcke.
    available: (ctx) => ctx.timetableEnabled && ctx.detailed,
  },
  {
    key: "section.active_groups",
    kind: "section",
    label: "Laufende Betreuung",
    description:
      "Welche Betreuungsgruppen und Aktivitäten gerade laufen, mit Ort und Kinderzahl.",
    concept: "groups",
    spans: SECTION_SPANS,
    permitted: operationalNumbers,
    available: (ctx) => !ctx.openCareGroupMode,
  },
  {
    key: "section.current_activities",
    kind: "section",
    label: "Laufende Aktivitäten",
    description: "Welche Aktivitäten gerade stattfinden und wie voll sie sind.",
    concept: "activities",
    spans: SECTION_SPANS,
    permitted: operationalNumbers,
    available: activitySurfaces,
  },
  {
    key: "section.recent_activity",
    kind: "section",
    label: "Letzte Bewegungen",
    description: "Welche Gruppen zuletzt den Raum gewechselt haben.",
    concept: "changeHistory",
    spans: SECTION_SPANS,
    permitted: operationalNumbers,
    available: roomSurfaces,
  },
  {
    key: "section.birthdays",
    kind: "section",
    label: "Geburtstage",
    description: "Wer heute oder in den nächsten Tagen Geburtstag hat.",
    concept: "birthdays",
    spans: SECTION_SPANS,
    permitted: (access) => access.has(PERMISSION.usersRead),
    available: (ctx) => ctx.birthdaysEnabled,
  },
];

const BLOCK_BY_KEY = new Map<HomeBlockKey, HomeBlockDefinition>(
  HOME_BLOCKS.map((block) => [block.key, block]),
);

/**
 * Ein Baustein an seinem Platz: welcher, wie breit, in welcher Reihe.
 *
 * Die REIHE ist die Gruppierung der Person, nicht die Zeile, in die das
 * Raster den Baustein zufällig spült. Eine Kachel, die zwischen zwei Reihen
 * abgelegt wird, bekommt eine eigene — auch eine schmale Kennzahl allein,
 * mit Luft daneben. Ohne dieses Feld entschiede das Raster selbst, und eine
 * Kennzahl rutschte neben die Karte, unter die sie gehören sollte.
 */
export interface HomeBlockPlacement {
  readonly key: HomeBlockKey;
  readonly span: HomeBlockSpan;
  /** Nullbasiert, lückenlos, in Anzeigereihenfolge. */
  readonly row: number;
}

/** Spalten des Rasters auf dem Desktop; eine Reihe fasst höchstens so viel. */
export const HOME_BOARD_COLUMNS = 4;

/**
 * Die Standardansichten, aus denen jemand startet.
 *
 * Drei Regeln halten sie brauchbar:
 *
 * 1. DIE JETZT-ZONE ÜBER DEM BRETT trägt den Blick auf den Moment: laufender
 *    Einsatz, nächster Einsatz, Hauptaktion. Das Brett darunter muss das
 *    nicht noch einmal leisten und darf in Ruhe den Tag zeigen.
 * 2. KURZ. Die Startseite gibt den Einstieg, sie zeigt nicht alles, was es
 *    gibt. Was fehlt, steht im Hinzufügen-Menü.
 * 3. KEINE DOPPLUNG. Zwei Bausteine, die dieselben Zeilen zeigen, gehören
 *    nicht zusammen in einen Standard. „Mein Tag" (die eigenen Einsätze) und
 *    „Ablauf des Tages" (alle Blöcke) sind so ein Paar: für eine
 *    Betreuungskraft ist der eigene Tag der interessante Ausschnitt, für die
 *    Leitung der ganze Ablauf. Wer beides ist, bekommt beides.
 *
 * Betreuung: der eigene Tag, die eigene Gruppe, die Hinweise der Leitung, was
 * in den nächsten Minuten ansteht — alles auf die Person zugeschnitten. Die
 * schulweite „Laufende Betreuung" steht im Hinzufügen-Menü: für die Kraft in
 * der Sternengruppe ist die Bärengruppe Rauschen.
 *
 * Leitung: die Lage der Schule in vier Zahlen, was auf eine Entscheidung
 * wartet, das Personal, die Hinweise, der Ablauf des Tages und die laufende
 * Betreuung.
 *
 * Geburtstage stehen in JEDER Standardansicht: wer heute Geburtstag hat,
 * betrifft die Betreuungskraft am Tisch genauso wie die Leitung. Über die
 * volle Breite, weil die Karte mehrere Kinder nebeneinander zeigt.
 */
export const DEFAULT_LAYOUTS: Record<
  HomeProfile,
  readonly HomeBlockPlacement[]
> = {
  care: [
    { key: "section.my_day", span: 2, row: 0 },
    { key: "section.my_group", span: 2, row: 0 },
    { key: "section.staff_notices", span: 2, row: 1 },
    { key: "section.reminders", span: 2, row: 1 },
    { key: "section.birthdays", span: 4, row: 2 },
  ],
  lead: [
    { key: "tile.students_present", span: 1, row: 0 },
    { key: "tile.students_sick", span: 1, row: 0 },
    { key: "tile.students_excused", span: 1, row: 0 },
    { key: "tile.students_home", span: 1, row: 0 },
    { key: "section.open_requests", span: 2, row: 1 },
    { key: "section.staff_today", span: 2, row: 1 },
    { key: "section.staff_notices", span: 2, row: 2 },
    { key: "section.day_flow", span: 2, row: 2 },
    { key: "section.messages", span: 2, row: 3 },
    { key: "section.active_groups", span: 2, row: 3 },
    { key: "section.birthdays", span: 4, row: 4 },
  ],
  // Die Vereinigung: erst der eigene Tag und die eigene Gruppe, dann die
  // Lage der Schule. Die Erinnerungen fehlen hier, weil die Jetzt-Zone und
  // der Ablauf des Tages den Moment schon tragen; sie stehen im Menü.
  lead_care: [
    { key: "section.my_day", span: 2, row: 0 },
    { key: "section.my_group", span: 2, row: 0 },
    { key: "tile.students_present", span: 1, row: 1 },
    { key: "tile.students_sick", span: 1, row: 1 },
    { key: "tile.students_excused", span: 1, row: 1 },
    { key: "tile.students_home", span: 1, row: 1 },
    { key: "section.open_requests", span: 2, row: 2 },
    { key: "section.staff_today", span: 2, row: 2 },
    { key: "section.staff_notices", span: 2, row: 3 },
    { key: "section.day_flow", span: 2, row: 3 },
    { key: "section.birthdays", span: 4, row: 4 },
  ],
};

/**
 * Bringt eine Anordnung in Form: Reihen lückenlos von 0 an, in der
 * Reihenfolge ihres ersten Auftretens, und keine Reihe breiter als das
 * Raster — was überläuft, rückt in eine neue Reihe direkt dahinter. Leere
 * Reihen gibt es danach nicht mehr.
 */
export function normalizeRows(
  placements: readonly HomeBlockPlacement[],
): HomeBlockPlacement[] {
  const rows = rowsOf(placements);
  const result: HomeBlockPlacement[] = [];
  let row = 0;
  for (const entries of rows) {
    let width = 0;
    for (const entry of entries) {
      if (width > 0 && width + entry.span > HOME_BOARD_COLUMNS) {
        row += 1;
        width = 0;
      }
      result.push({ key: entry.key, span: entry.span, row });
      width += entry.span;
    }
    row += 1;
  }
  return result;
}

/** Die Anordnung als Reihen, jede in Anzeigereihenfolge. */
export function rowsOf(
  placements: readonly HomeBlockPlacement[],
): HomeBlockPlacement[][] {
  const byRow = new Map<number, HomeBlockPlacement[]>();
  for (const placement of placements) {
    const entries = byRow.get(placement.row);
    if (entries) entries.push(placement);
    else byRow.set(placement.row, [placement]);
  }
  return Array.from(byRow.keys())
    .sort((a, b) => a - b)
    .map((row) => byRow.get(row)!);
}

/**
 * Wohin ein Baustein zieht: in eine bestehende Reihe an eine Stelle, oder
 * in eine neue Reihe vor der Reihe `before` (`before` gleich der Zahl der
 * Reihen heißt: ganz unten).
 */
export type HomeMoveTarget =
  | { readonly kind: "into"; readonly row: number; readonly index: number }
  | { readonly kind: "newRow"; readonly before: number };

/**
 * Versetzt einen Baustein. Läuft die Zielreihe dadurch über, teilt sie sich;
 * eine Reihe, die leer wird, verschwindet.
 */
export function movePlacement(
  placements: readonly HomeBlockPlacement[],
  key: HomeBlockKey,
  target: HomeMoveTarget,
): HomeBlockPlacement[] {
  const moving = placements.find((entry) => entry.key === key);
  if (!moving) return placements as HomeBlockPlacement[];
  const rows = rowsOf(placements).map((entries) =>
    entries.filter((entry) => entry.key !== key),
  );

  if (target.kind === "newRow") {
    const before = Math.max(0, Math.min(target.before, rows.length));
    rows.splice(before, 0, [moving]);
  } else {
    const row = Math.max(0, Math.min(target.row, rows.length - 1));
    const entries = rows[row] ?? [];
    // In eine volle Reihe passt nichts mehr hinein. Sie wird NICHT geteilt:
    // wer eine Kachel über eine volle Reihe zieht, will sie dort nicht
    // ablegen, und ein Zug darüber hinweg darf keine Spuren hinterlassen.
    const width = entries.reduce((sum, entry) => sum + entry.span, 0);
    if (width + moving.span > HOME_BOARD_COLUMNS) {
      return placements as HomeBlockPlacement[];
    }
    const index = Math.max(0, Math.min(target.index, entries.length));
    entries.splice(index, 0, moving);
    rows[row] = entries;
  }

  const next = normalizeRows(
    rows.flatMap((entries, row) => entries.map((entry) => ({ ...entry, row }))),
  );
  // Ein Zug, der nichts ändert (der Zeiger steht noch in derselben Zelle),
  // gibt dieselbe Anordnung zurück — sonst rendert jede Zeigerbewegung neu.
  return sameArrangement(placements, next)
    ? (placements as HomeBlockPlacement[])
    : next;
}

function sameArrangement(
  a: readonly HomeBlockPlacement[],
  b: readonly HomeBlockPlacement[],
): boolean {
  return (
    a.length === b.length &&
    a.every((entry, index) => {
      const other = b[index]!;
      return (
        entry.key === other.key &&
        entry.span === other.span &&
        entry.row === other.row
      );
    })
  );
}

/** Ändert die Breite; eine Reihe, die dadurch überläuft, teilt sich. */
export function placementWithSpan(
  placements: readonly HomeBlockPlacement[],
  key: HomeBlockKey,
  span: HomeBlockSpan,
): HomeBlockPlacement[] {
  return normalizeRows(
    placements.map((entry) => (entry.key === key ? { ...entry, span } : entry)),
  );
}

/**
 * Hängt einen Baustein ans Ende: in die letzte Reihe, wenn er dort noch
 * Platz hat, sonst in eine neue darunter.
 */
export function appendPlacement(
  placements: readonly HomeBlockPlacement[],
  key: HomeBlockKey,
  span: HomeBlockSpan,
): HomeBlockPlacement[] {
  const rows = rowsOf(placements);
  const last = rows[rows.length - 1];
  const lastWidth = last?.reduce((sum, entry) => sum + entry.span, 0) ?? 0;
  const row =
    last && lastWidth + span <= HOME_BOARD_COLUMNS
      ? rows.length - 1
      : rows.length;
  return normalizeRows([...placements, { key, span, row }]);
}

/** Entfernt einen Baustein; seine Reihe verschwindet, wenn sie leer wird. */
export function withoutPlacement(
  placements: readonly HomeBlockPlacement[],
  key: HomeBlockKey,
): HomeBlockPlacement[] {
  return normalizeRows(placements.filter((entry) => entry.key !== key));
}

/** Eine Zelle des gezeichneten Rasters, einsbasiert wie in CSS. */
export interface HomeBoardCell {
  readonly columnStart: number;
  readonly columnSpan: number;
  readonly rowStart: number;
  readonly rowSpan: number;
}

/**
 * Rechnet die Reihen in Zellen eines Rasters mit `columns` Spalten um.
 *
 * Eine Kennzahl ist eine Rasterzeile hoch, eine Liste zwei. Eine Reihe des
 * Modells belegt so viele Rasterzeilen, wie ihr höchster Baustein braucht;
 * ist das Raster schmaler als die Reihe (Tablet: zwei Spalten), bricht die
 * Reihe innerhalb ihres Bandes um. Das Raster zeichnet damit genau die
 * Reihenfolge, die die Person gebaut hat — statt Lücken nach eigenem
 * Ermessen zu füllen.
 */
export function computeBoardCells(
  placements: readonly HomeBlockPlacement[],
  columns: number,
): Map<HomeBlockKey, HomeBoardCell> {
  const cells = new Map<HomeBlockKey, HomeBoardCell>();
  let rowStart = 1;
  for (const entries of rowsOf(placements)) {
    let column = 1;
    let lineStart = rowStart;
    let lineHeight = 0;
    for (const entry of entries) {
      const columnSpan = Math.min(entry.span, columns);
      const rowSpan = homeBlockDefinition(entry.key)?.kind === "tile" ? 1 : 2;
      if (column > 1 && column + columnSpan - 1 > columns) {
        lineStart += lineHeight;
        column = 1;
        lineHeight = 0;
      }
      cells.set(entry.key, {
        columnStart: column,
        columnSpan,
        rowStart: lineStart,
        rowSpan,
      });
      column += columnSpan;
      lineHeight = Math.max(lineHeight, rowSpan);
    }
    rowStart = lineStart + lineHeight;
  }
  return cells;
}

/**
 * Welche Standardansicht jemand bekommt.
 *
 * Am Adminzuschnitt festgemacht, nicht am Rollennamen: eine Schule kann ihre
 * Rollen frei anlegen, und die Frage ist nur, ob jemand die Einrichtung führt
 * oder in ihr betreut. Wer beides tut, bekommt die Vereinigung — den eigenen
 * Tag vorneweg, die Lage der Schule dahinter.
 */
export function homeProfileFor(access: HomeBlockAccess): HomeProfile {
  if (!access.isAdminScope) return "care";
  return access.caresForGroups ? "lead_care" : "lead";
}

export interface ResolvedHomeLayout {
  /** Was gezeigt wird, in dieser Reihenfolge und Breite. */
  readonly placements: readonly HomeBlockPlacement[];
  /** Was die Person noch hinzufügen kann. */
  readonly addable: readonly HomeBlockDefinition[];
  /** Alles, was es in dieser Schule für diese Person gibt. */
  readonly available: readonly HomeBlockDefinition[];
  /** Weicht die Person vom Standard ab? Steuert "Standard wiederherstellen". */
  readonly customized: boolean;
}

function clampSpan(
  block: HomeBlockDefinition,
  span: number | undefined,
): HomeBlockSpan {
  const allowed = block.spans;
  if (span !== undefined && allowed.includes(span as HomeBlockSpan)) {
    return span as HomeBlockSpan;
  }
  // Der mittlere Wert ist die Voreinstellung einer Liste, der einzige Wert die
  // einer Kennzahl.
  return allowed.length > 1 ? (allowed[1] as HomeBlockSpan) : allowed[0]!;
}

/**
 * Baut die Fläche aus Katalog, Vorgabe der Schule und eigener Anordnung.
 *
 * Reihenfolge der Entscheidungen:
 *   1. Was die Person abrufen darf und was es in dieser Schule gibt.
 *   2. Was die Schule abgeschaltet hat, fällt raus; was sie verlangt, bleibt
 *      drin, auch wenn die Person es entfernt hat.
 *   3. Die gespeicherte Anordnung gibt Reihenfolge und Breite.
 *   4. Ein Baustein aus der Standardansicht, den die Person weder angeordnet
 *      noch entfernt hat, kommt ans Ende — so erreicht ein später ergänzter
 *      Baustein auch bestehende Konten.
 */
export function resolveHomeLayout(
  ctx: HomeBlockContext,
  stored: readonly HomeBlockPlacement[] | null | undefined,
  overrides: HomeLayoutOverrides | null | undefined,
  policies: HomeBlockPolicies | null | undefined,
): ResolvedHomeLayout {
  const available = HOME_BLOCKS.filter(
    (block) => block.permitted(ctx.access) && block.available(ctx),
  );
  const availableKeys = new Set(available.map((block) => block.key));
  const policyOf = (key: HomeBlockKey) => policies?.[key] ?? "optional";

  let placements: HomeBlockPlacement[] = [];
  const placed = new Set<HomeBlockKey>();

  const allowed = (key: HomeBlockKey): HomeBlockDefinition | null => {
    if (placed.has(key) || !availableKeys.has(key)) return null;
    if (policyOf(key) === "disabled") return null;
    return BLOCK_BY_KEY.get(key) ?? null;
  };

  // Die gespeicherte Anordnung behält ihre Reihen; die Reihen der
  // Standardansicht gelten nur, solange niemand angeordnet hat.
  const arranged = (stored?.length ?? 0) > 0;
  const profile = homeProfileFor(ctx.access);

  for (const entry of stored ?? []) {
    const block = allowed(entry.key);
    if (!block) continue;
    placements.push({
      key: entry.key,
      span: clampSpan(block, entry.span),
      row: entry.row,
    });
    placed.add(entry.key);
  }
  placements = normalizeRows(placements);

  for (const entry of DEFAULT_LAYOUTS[profile]) {
    // Entfernt bleibt entfernt. Alles andere aus der Standardansicht steht am
    // Ende — beim ersten Besuch ist das die ganze Ansicht, später ist es der
    // Weg, auf dem ein neuer Baustein bestehende Konten erreicht.
    if (overrides?.[entry.key] === false) continue;
    const block = allowed(entry.key);
    if (!block) continue;
    const span = clampSpan(block, entry.span);
    placements = arranged
      ? appendPlacement(placements, entry.key, span)
      : normalizeRows([
          ...placements,
          { key: entry.key, span, row: entry.row },
        ]);
    placed.add(entry.key);
  }

  // Was die Schule verlangt, steht auf jeder Startseite, die es sehen darf.
  for (const block of available) {
    if (policyOf(block.key) !== "required" || !allowed(block.key)) continue;
    placements = appendPlacement(
      placements,
      block.key,
      clampSpan(block, undefined),
    );
    placed.add(block.key);
  }

  const addable = available.filter(
    (block) => !placed.has(block.key) && policyOf(block.key) !== "disabled",
  );

  const customized =
    arranged || Object.values(overrides ?? {}).some((shown) => shown === false);

  return { placements, addable, available, customized };
}

function isHomeBlockKey(value: unknown): value is HomeBlockKey {
  return typeof value === "string" && BLOCK_BY_KEY.has(value as HomeBlockKey);
}

function isHomeBlockPolicy(value: unknown): value is HomeBlockPolicy {
  return value === "optional" || value === "required" || value === "disabled";
}

/**
 * Verwirft unbekannte Schlüssel und falsche Werte aus gespeicherten Daten.
 *
 * Ein Baustein, den es nicht mehr gibt, darf keine Rolle mehr spielen — weder
 * auf der Fläche noch beim nächsten Speichern.
 */
export function sanitizeHomeLayoutOverrides(raw: unknown): HomeLayoutOverrides {
  if (raw === null || typeof raw !== "object") return {};
  const result: HomeLayoutOverrides = {};
  for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
    if (isHomeBlockKey(key) && typeof value === "boolean") {
      result[key] = value;
    }
  }
  return result;
}

/** Dasselbe für die gespeicherte Anordnung: Form prüfen, Doppelte verwerfen. */
export function sanitizeHomeBlockPlacements(
  raw: unknown,
): readonly HomeBlockPlacement[] {
  if (!Array.isArray(raw)) return [];
  const result: HomeBlockPlacement[] = [];
  const seen = new Set<HomeBlockKey>();
  // Anordnungen von vor den Reihen tragen überall Reihe 0 (oder gar keine).
  // Sie werden nach Breite gepackt, so wie das Raster sie damals gezeichnet
  // hat — die Person sieht also, was sie kannte, nur jetzt mit fester Reihe.
  const legacy = raw.every(
    (entry) =>
      entry === null ||
      typeof entry !== "object" ||
      typeof (entry as { row?: unknown }).row !== "number" ||
      (entry as { row: number }).row === 0,
  );
  let row = 0;
  let width = 0;
  for (const entry of raw) {
    if (entry === null || typeof entry !== "object") continue;
    const {
      key,
      span: rawSpan,
      row: rawRow,
    } = entry as { key?: unknown; span?: unknown; row?: unknown };
    if (!isHomeBlockKey(key) || seen.has(key)) continue;
    const block = BLOCK_BY_KEY.get(key)!;
    const span = clampSpan(
      block,
      typeof rawSpan === "number" ? rawSpan : undefined,
    );
    if (legacy) {
      if (width > 0 && width + span > HOME_BOARD_COLUMNS) {
        row += 1;
        width = 0;
      }
      width += span;
    } else {
      row =
        typeof rawRow === "number" && Number.isInteger(rawRow) && rawRow >= 0
          ? rawRow
          : row;
    }
    result.push({ key, span, row });
    seen.add(key);
  }
  return normalizeRows(result);
}

export function sanitizeHomeBlockPolicies(raw: unknown): HomeBlockPolicies {
  if (raw === null || typeof raw !== "object") return {};
  const result: HomeBlockPolicies = {};
  for (const [key, value] of Object.entries(raw as Record<string, unknown>)) {
    if (isHomeBlockKey(key) && isHomeBlockPolicy(value)) {
      result[key] = value;
    }
  }
  return result;
}

/** Die Breite, mit der ein Baustein neu auf die Fläche kommt. */
export function defaultSpanFor(key: HomeBlockKey): HomeBlockSpan {
  const block = BLOCK_BY_KEY.get(key);
  if (!block) return 2;
  return clampSpan(block, undefined);
}

/** Beschriftung eines Bausteins, oder null, wenn es ihn nicht mehr gibt. */
export function homeBlockDefinition(
  key: HomeBlockKey,
): HomeBlockDefinition | null {
  return BLOCK_BY_KEY.get(key) ?? null;
}
