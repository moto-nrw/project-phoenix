/**
 * Baustein-Register der Startseite (#2875, modular seit #2180).
 *
 * Die Startseite ist keine feste Bildschirmfläche, sondern ein Brett aus
 * Bausteinen, das jede Person selbst zusammenstellt: welche Bausteine, an
 * welcher Stelle, wie breit. Wer nichts anfasst, sieht die
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
 * ANORDNUNG (Zelle und Breite dessen, was die Person platziert hat) und
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
   * Höhe in Rasterzeilen. Eine Kennzahl ist eine Zeile hoch, eine Liste
   * zwei; ein Baustein, der den ganzen Tag trägt, darf drei haben. Ohne
   * Angabe gilt die Höhe der Art.
   */
  readonly height?: 2 | 3;
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
    description:
      "Ihr ganzer Betreuungstag: alle Blöcke, für die Sie eingeteilt sind, mit Raum, Kindern und Kolleginnen. Starten geht direkt hier.",
    concept: "carePlan",
    spans: SECTION_SPANS,
    // Der Tag braucht Platz: drei Zeilen hoch, damit ein Tag mit fünf
    // Blöcken ganz dasteht statt als „Noch 3 Einsätze".
    height: 3,
    // Dieselbe Quelle wie der Tagesplan (/timetable/operations/planned-now),
    // damit die Startseite genau das trägt, was dort steht.
    permitted: (access) => access.has(PERMISSION.schedulesRead),
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
 * Ein Baustein an seinem Platz: welcher, wie breit, in welcher Zelle.
 *
 * Das Brett ist ein freies Raster aus vier Spalten und beliebig vielen
 * Zeilen. Jede Kachel steht in der Zelle, in die man sie gelegt hat — Spalte
 * und Zeile unabhängig voneinander, Lücken erlaubt. Nichts rutscht von
 * allein nach oben oder zur Seite: was die Person baut, bleibt so.
 */
export interface HomeBlockPlacement {
  readonly key: HomeBlockKey;
  readonly span: HomeBlockSpan;
  /** Spalte der linken Kante, nullbasiert; höchstens `4 - span`. */
  readonly col: number;
  /** Rasterzeile der oberen Kante, nullbasiert. Eine Zeile ist 7rem hoch. */
  readonly row: number;
}

/** Spalten des Rasters auf dem Desktop. */
export const HOME_BOARD_COLUMNS = 4;

/**
 * Höhe eines Bausteins in Rasterzeilen: eine Kennzahl eine, eine Liste zwei.
 * Feste Höhen halten die Karten auf einer Linie; was nicht hineinpasst,
 * zählt die Karte und verlinkt es.
 */
export function homeBlockHeight(key: HomeBlockKey): number {
  const definition = homeBlockDefinition(key);
  if (!definition) return 2;
  return definition.height ?? (definition.kind === "tile" ? 1 : 2);
}

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
 * betrifft die Betreuungskraft am Tisch genauso wie die Leitung.
 *
 * Die Zellen hier sind das Bild, wenn ALLE Bausteine da sind. Beim Aufbau
 * wird in dieser Reihenfolge gepackt (`resolveHomeLayout`): fehlt ein
 * Baustein, weil das Recht, der Betriebsmodus oder die Vorgabe der Schule
 * ihn ausschließt, rückt der nächste nach, statt ein Loch zu lassen.
 */
export const DEFAULT_LAYOUTS: Record<
  HomeProfile,
  readonly HomeBlockPlacement[]
> = {
  // Der Tag zuerst und über die volle Breite: das ist die Seite, von der aus
  // eine Betreuungskraft arbeitet. Darunter die Gruppe und was ansteht.
  care: [
    { key: "section.my_day", span: 4, col: 0, row: 0 },
    { key: "section.my_group", span: 2, col: 0, row: 3 },
    { key: "section.reminders", span: 2, col: 2, row: 3 },
    { key: "section.staff_notices", span: 2, col: 0, row: 5 },
    { key: "section.birthdays", span: 2, col: 2, row: 5 },
  ],
  lead: [
    { key: "tile.students_present", span: 1, col: 0, row: 0 },
    { key: "tile.students_sick", span: 1, col: 1, row: 0 },
    { key: "tile.students_excused", span: 1, col: 2, row: 0 },
    { key: "tile.students_home", span: 1, col: 3, row: 0 },
    { key: "section.open_requests", span: 2, col: 0, row: 1 },
    { key: "section.staff_today", span: 2, col: 2, row: 1 },
    { key: "section.staff_notices", span: 2, col: 0, row: 3 },
    { key: "section.day_flow", span: 2, col: 2, row: 3 },
    { key: "section.messages", span: 2, col: 0, row: 5 },
    { key: "section.active_groups", span: 2, col: 2, row: 5 },
    { key: "section.birthdays", span: 4, col: 0, row: 7 },
  ],
  // Die Vereinigung: erst der eigene Tag und die eigene Gruppe, dann die
  // Lage der Schule. Die Erinnerungen fehlen hier, weil die Jetzt-Zone und
  // der Ablauf des Tages den Moment schon tragen; sie stehen im Menü.
  lead_care: [
    { key: "section.my_day", span: 4, col: 0, row: 0 },
    { key: "section.my_group", span: 2, col: 0, row: 3 },
    { key: "section.open_requests", span: 2, col: 2, row: 3 },
    { key: "tile.students_present", span: 1, col: 0, row: 5 },
    { key: "tile.students_sick", span: 1, col: 1, row: 5 },
    { key: "tile.students_excused", span: 1, col: 2, row: 5 },
    { key: "tile.students_home", span: 1, col: 3, row: 5 },
    { key: "section.staff_today", span: 2, col: 0, row: 6 },
    { key: "section.staff_notices", span: 2, col: 2, row: 6 },
    { key: "section.day_flow", span: 2, col: 0, row: 8 },
    { key: "section.birthdays", span: 2, col: 2, row: 8 },
  ],
};

/**
 * Die Zelle, in die die linke obere Ecke der gezogenen Kachel gelegt wird.
 * Dort bleibt sie; was darunter liegt, rückt nach unten, und alles andere
 * rutscht in freie Zellen nach oben und nach links.
 */
export interface HomeMoveTarget {
  readonly col: number;
  readonly row: number;
}

/** Ohne Maus: wohin die Pfeiltasten eine Kachel rücken. */
export type HomeMoveDirection = "left" | "right" | "up" | "down";

function overlaps(a: HomeBlockPlacement, b: HomeBlockPlacement): boolean {
  return (
    a.col < b.col + b.span &&
    b.col < a.col + a.span &&
    a.row < b.row + homeBlockHeight(b.key) &&
    b.row < a.row + homeBlockHeight(a.key)
  );
}

function clampCol(col: number, span: HomeBlockSpan): number {
  return Math.max(0, Math.min(Math.floor(col), HOME_BOARD_COLUMNS - span));
}

/** Zeilen und Spalten zuerst, dann die Reihenfolge, in der man liest. */
function byCell(a: HomeBlockPlacement, b: HomeBlockPlacement): number {
  return a.row - b.row || a.col - b.col;
}

/**
 * Die Anordnung in Lesereihenfolge (Zeile für Zeile, links nach rechts).
 * So wird gespeichert, damit dieselbe Anordnung immer gleich aussieht.
 */
export function sortedPlacements(
  placements: readonly HomeBlockPlacement[],
): HomeBlockPlacement[] {
  return [...placements].sort(byCell);
}

function isFree(
  placements: readonly HomeBlockPlacement[],
  candidate: HomeBlockPlacement,
): boolean {
  return placements.every(
    (entry) => entry.key === candidate.key || !overlaps(entry, candidate),
  );
}

/**
 * Setzt einen Baustein an die erste freie Stelle, von links nach rechts,
 * Zeile für Zeile. Unter dem Brett ist immer Platz.
 */
function firstFit(
  placements: readonly HomeBlockPlacement[],
  key: HomeBlockKey,
  span: HomeBlockSpan,
  columns = HOME_BOARD_COLUMNS,
): HomeBlockPlacement {
  for (let row = 0; ; row += 1) {
    for (let col = 0; col + span <= columns; col += 1) {
      const candidate = { key, span, col, row };
      if (isFree(placements, candidate)) return candidate;
    }
  }
}

/**
 * Packt Bausteine in der gegebenen Reihenfolge von oben links her: für die
 * Standardansicht, für Anordnungen von vor dem Raster und für das schmale
 * Raster des Tablets, das die Spalten der Person nicht zeichnen kann.
 */
function packInOrder(
  ordered: readonly HomeBlockPlacement[],
  columns = HOME_BOARD_COLUMNS,
): HomeBlockPlacement[] {
  const laid: HomeBlockPlacement[] = [];
  for (const entry of ordered) {
    laid.push(
      firstFit(
        laid,
        entry.key,
        Math.min(entry.span, columns) as HomeBlockSpan,
        columns,
      ),
    );
  }
  return laid;
}

/**
 * Löst Überlappungen auf, indem Bausteine nach UNTEN rücken — nie zur Seite
 * und nie nach oben. Wer eine Kachel auf eine andere legt, schiebt die andere
 * unter sich.
 *
 * `first` ist die Kachel, die stehen bleibt (die gerade bewegte). Danach
 * gilt Lesereihenfolge: was weiter oben steht, hat Vorrang. Jede Kachel wird
 * so weit nach unten gesetzt, dass sie keine bereits festgelegte mehr
 * berührt; da sie nur nach unten rückt, endet das immer.
 */
function pushApart(
  placements: readonly HomeBlockPlacement[],
  first: HomeBlockKey | null,
): HomeBlockPlacement[] {
  const fixed: HomeBlockPlacement[] = [];
  const ordered = sortedPlacements(placements);
  const lead = ordered.find((entry) => entry.key === first);
  if (lead) fixed.push(lead);
  for (const entry of ordered) {
    if (entry.key === first) continue;
    let placed = entry;
    let bumped = true;
    while (bumped) {
      bumped = false;
      for (const other of fixed) {
        if (!overlaps(placed, other)) continue;
        placed = { ...placed, row: other.row + homeBlockHeight(other.key) };
        bumped = true;
      }
    }
    fixed.push(placed);
  }
  const byKey = new Map(fixed.map((entry) => [entry.key, entry]));
  return placements.map((entry) => byKey.get(entry.key) ?? entry);
}

/**
 * Die Schwerkraft des Bretts: jede Kachel rutscht so weit nach OBEN, wie
 * ihre Spalten frei sind, und dann so weit nach LINKS, wie ihre Zeilen frei
 * sind — Zeile für Zeile, bis sich nichts mehr bewegt. So bleibt kein Loch,
 * und jede Spalte steht für sich: eine Kennzahl unter einer großen Karte
 * links lässt die rechte Seite in Ruhe. Wie auf einem Startbildschirm.
 */
function settle(
  placements: readonly HomeBlockPlacement[],
): HomeBlockPlacement[] {
  let current = [...placements];
  let moved = true;
  while (moved) {
    moved = false;
    for (const entry of sortedPlacements(current)) {
      let placed = entry;
      while (
        placed.row > 0 &&
        isFree(current, { ...placed, row: placed.row - 1 })
      ) {
        placed = { ...placed, row: placed.row - 1 };
      }
      while (
        placed.col > 0 &&
        isFree(current, { ...placed, col: placed.col - 1 })
      ) {
        placed = { ...placed, col: placed.col - 1 };
      }
      if (placed !== entry) {
        current = current.map((item) =>
          item.key === entry.key ? placed : item,
        );
        moved = true;
      }
    }
  }
  return current;
}

function samePlacements(
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
        entry.col === other.col &&
        entry.row === other.row
      );
    })
  );
}

/** Dieselbe Anordnung heißt dieselbe Referenz: sonst rendert jede Zeigerbewegung neu. */
function unlessSame(
  before: readonly HomeBlockPlacement[],
  after: HomeBlockPlacement[],
): HomeBlockPlacement[] {
  return samePlacements(before, after)
    ? (before as HomeBlockPlacement[])
    : after;
}

/**
 * Bringt eine Anordnung in Form: Spalten im Raster, Zeilen ganze Zahlen,
 * keine zwei Bausteine auf derselben Zelle, kein Loch.
 */
function normalizePlacements(
  placements: readonly HomeBlockPlacement[],
): HomeBlockPlacement[] {
  const inGrid = placements.map((entry) => ({
    ...entry,
    col: clampCol(entry.col, entry.span),
    row: Math.max(0, Math.floor(entry.row)),
  }));
  return settle(pushApart(inGrid, null));
}

/**
 * Legt einen Baustein in eine Zelle. Was dort schon liegt, rückt nach unten;
 * danach wirkt die Schwerkraft — auf ihn selbst wie auf alle anderen. Ein
 * Zug, der nichts ändert, gibt dieselbe Anordnung zurück.
 */
export function placePlacement(
  placements: readonly HomeBlockPlacement[],
  key: HomeBlockKey,
  target: HomeMoveTarget,
): HomeBlockPlacement[] {
  const moving = placements.find((entry) => entry.key === key);
  if (!moving) return placements as HomeBlockPlacement[];
  const col = clampCol(target.col, moving.span);
  const row = Math.max(0, Math.floor(target.row));
  if (col === moving.col && row === moving.row) {
    return placements as HomeBlockPlacement[];
  }
  const dropped = placements.map((entry) =>
    entry.key === key ? { ...entry, col, row } : entry,
  );
  return unlessSame(placements, settle(pushApart(dropped, key)));
}

/**
 * Rückt einen Baustein ohne Maus: links und rechts tauschen den Platz mit
 * der Nachbarkachel, oben setzt ihn auf die Kachel darüber (die weicht nach
 * unten), unten setzt ihn unter die Kachel darunter. Ohne Nachbarn in der
 * Richtung passiert nichts.
 */
export function movePlacementBy(
  placements: readonly HomeBlockPlacement[],
  key: HomeBlockKey,
  direction: HomeMoveDirection,
): HomeBlockPlacement[] {
  const moving = placements.find((entry) => entry.key === key);
  if (!moving) return placements as HomeBlockPlacement[];
  const height = homeBlockHeight(key);
  const sharesRows = (entry: HomeBlockPlacement) =>
    entry.row < moving.row + height &&
    moving.row < entry.row + homeBlockHeight(entry.key);
  const sharesCols = (entry: HomeBlockPlacement) =>
    entry.col < moving.col + moving.span && moving.col < entry.col + entry.span;
  const others = placements.filter((entry) => entry.key !== key);
  let neighbour: HomeBlockPlacement | undefined;
  let target: HomeMoveTarget | undefined;
  switch (direction) {
    // Links: auf den Anfang des Nachbarn, der rückt direkt dahinter. Rechts:
    // der Nachbar rückt auf den eigenen Anfang, die Kachel direkt dahinter.
    // So tauschen auch verschieden breite Kacheln ohne Überlappung.
    case "left":
      neighbour = others
        .filter((entry) => sharesRows(entry) && entry.col < moving.col)
        .sort((a, b) => b.col - a.col)[0];
      target = neighbour && { col: neighbour.col, row: neighbour.row };
      break;
    case "right":
      neighbour = others
        .filter((entry) => sharesRows(entry) && entry.col > moving.col)
        .sort((a, b) => a.col - b.col)[0];
      target = neighbour && {
        col: moving.col + neighbour.span,
        row: neighbour.row,
      };
      break;
    case "up":
      neighbour = others
        .filter((entry) => sharesCols(entry) && entry.row < moving.row)
        .sort((a, b) => b.row - a.row)[0];
      target = neighbour && { col: moving.col, row: neighbour.row };
      break;
    case "down":
      neighbour = others
        .filter((entry) => sharesCols(entry) && entry.row > moving.row)
        .sort((a, b) => a.row - b.row)[0];
      target = neighbour && {
        col: moving.col,
        row: neighbour.row + homeBlockHeight(neighbour.key),
      };
      break;
  }
  if (!neighbour || !target) return placements as HomeBlockPlacement[];
  // Links und rechts ist ein Tausch: der Nachbar nimmt den alten Platz.
  const swapped =
    direction === "left" || direction === "right"
      ? placements.map((entry) =>
          entry.key === neighbour.key
            ? {
                ...entry,
                col: clampCol(moving.col, entry.span),
                row: moving.row,
              }
            : entry,
        )
      : placements;
  return placePlacement(swapped, key, target);
}

/**
 * Ändert die Breite. Reicht die Kachel damit über den rechten Rand, rückt
 * sie nach links; was sie dann überdeckt, rückt nach unten, dann wirkt die
 * Schwerkraft.
 */
export function placementWithSpan(
  placements: readonly HomeBlockPlacement[],
  key: HomeBlockKey,
  span: HomeBlockSpan,
): HomeBlockPlacement[] {
  const current = placements.find((entry) => entry.key === key);
  if (!current || current.span === span) {
    return placements as HomeBlockPlacement[];
  }
  const resized = placements.map((entry) =>
    entry.key === key
      ? { ...entry, span, col: clampCol(entry.col, span) }
      : entry,
  );
  return settle(pushApart(resized, key));
}

/** Hängt einen Baustein unter die bestehende Anordnung; unter dem Brett ist immer Platz. */
export function appendPlacement(
  placements: readonly HomeBlockPlacement[],
  key: HomeBlockKey,
  span: HomeBlockSpan,
): HomeBlockPlacement[] {
  const nextRow = placements.reduce(
    (bottom, entry) => Math.max(bottom, entry.row + homeBlockHeight(entry.key)),
    0,
  );
  return [...placements, { key, span, col: 0, row: nextRow }];
}

/** Entfernt einen Baustein; was seine Zelle brauchen kann, rückt nach. */
export function withoutPlacement(
  placements: readonly HomeBlockPlacement[],
  key: HomeBlockKey,
): HomeBlockPlacement[] {
  return settle(placements.filter((entry) => entry.key !== key));
}

/** Eine Zelle des gezeichneten Rasters, einsbasiert wie in CSS. */
export interface HomeBoardCell {
  readonly columnStart: number;
  readonly columnSpan: number;
  readonly rowStart: number;
  readonly rowSpan: number;
}

/**
 * Rechnet die Anordnung in Zellen eines Rasters mit `columns` Spalten um.
 *
 * Auf vier Spalten ist das die Anordnung selbst. Ein schmaleres Raster
 * (Tablet: zwei Spalten) kann die Spalten der Person nicht zeichnen; dort
 * fließen die Bausteine in Lesereihenfolge nach und füllen das Raster von
 * oben, jeder höchstens so breit wie das Raster.
 */
export function computeBoardCells(
  placements: readonly HomeBlockPlacement[],
  columns: number,
): Map<HomeBlockKey, HomeBoardCell> {
  const cells = new Map<HomeBlockKey, HomeBoardCell>();
  const laid =
    columns >= HOME_BOARD_COLUMNS
      ? placements
      : packInOrder(sortedPlacements(placements), columns);
  for (const placed of laid) {
    cells.set(placed.key, {
      columnStart: placed.col + 1,
      columnSpan: placed.span,
      rowStart: placed.row + 1,
      rowSpan: homeBlockHeight(placed.key),
    });
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
  /** Was gezeigt wird, an diesen Zellen und in dieser Breite. */
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
 *   3. Die gespeicherte Anordnung gibt Zelle und Breite.
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
      col: entry.col,
      row: entry.row,
    });
    placed.add(entry.key);
  }
  placements = normalizePlacements(placements);

  for (const entry of DEFAULT_LAYOUTS[profile]) {
    // Entfernt bleibt entfernt. Alles andere aus der Standardansicht steht am
    // Ende — beim ersten Besuch ist das die ganze Ansicht, später ist es der
    // Weg, auf dem ein neuer Baustein bestehende Konten erreicht.
    if (overrides?.[entry.key] === false) continue;
    const block = allowed(entry.key);
    if (!block) continue;
    const span = clampSpan(block, entry.span);
    // Die Standardansicht wird in Lesereihenfolge GEPACKT, nicht an ihre
    // Zellen gesetzt: fehlt ein Baustein (Recht, Betriebsmodus, Vorgabe der
    // Schule), rückt der nächste in seine Zelle nach, statt dass ein Loch
    // bleibt. Die Zellen in DEFAULT_LAYOUTS sind das Bild, das entsteht, wenn
    // alles da ist. Eine eigene Anordnung dagegen behält ihre Löcher — die
    // hat die Person so gebaut.
    placements = arranged
      ? appendPlacement(placements, entry.key, span)
      : [...placements, firstFit(placements, entry.key, span)];
    placed.add(entry.key);
  }

  // Was die Schule verlangt, steht auf jeder Startseite, die es sehen darf.
  // In der Standardansicht rückt es in die erste freie Zelle (sonst hinge
  // eine einzelne Kennzahl unten rechts neben der letzten Karte); eine
  // eigene Anordnung bekommt es hinten angehängt.
  for (const block of available) {
    if (policyOf(block.key) !== "required" || !allowed(block.key)) continue;
    const span = clampSpan(block, undefined);
    placements = arranged
      ? appendPlacement(placements, block.key, span)
      : [...placements, firstFit(placements, block.key, span)];
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
  // Anordnungen von vor dem freien Raster kennen keine Spalte. Sie werden
  // in ihrer Reihenfolge von oben links her gepackt, so wie das Raster sie
  // damals zeichnete — die Person sieht also, was sie kannte, nur jetzt mit
  // fester Zelle.
  const legacy = raw.every(
    (entry) =>
      entry === null ||
      typeof entry !== "object" ||
      typeof (entry as { col?: unknown }).col !== "number",
  );
  for (const entry of raw) {
    if (entry === null || typeof entry !== "object") continue;
    const {
      key,
      span: rawSpan,
      col: rawCol,
      row: rawRow,
    } = entry as {
      key?: unknown;
      span?: unknown;
      col?: unknown;
      row?: unknown;
    };
    if (!isHomeBlockKey(key) || seen.has(key)) continue;
    const block = BLOCK_BY_KEY.get(key)!;
    const span = clampSpan(
      block,
      typeof rawSpan === "number" ? rawSpan : undefined,
    );
    seen.add(key);
    if (legacy) {
      result.push(firstFit(result, key, span));
      continue;
    }
    const cell = (value: unknown) =>
      typeof value === "number" && Number.isFinite(value) && value >= 0
        ? Math.floor(value)
        : 0;
    result.push({ key, span, col: cell(rawCol), row: cell(rawRow) });
  }
  return normalizePlacements(result);
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
