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

/** Standardansicht, aus der jemand startet. Leitet sich aus Rechten ab. */
export type HomeProfile = "care" | "lead";

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
  | "section.staff_notices"
  | "section.day_flow"
  | "section.open_requests"
  | "section.reminders";

const always = () => true;
const roomSurfaces = (ctx: HomeBlockModeContext) => ctx.detailed;
const activitySurfaces = (ctx: HomeBlockModeContext) =>
  ctx.detailed && ctx.nfcEnabled;

// Die Rechte, an denen die Endpunkte der Bausteine hängen. Namen wie im
// Backend (auth/authorize/permissions), damit ein Vergleich möglich bleibt.
const PERMISSION = {
  /** GET /api/active/analytics/dashboard */
  analytics: "groups:read",
  /** GET /api/birthdays und GET /api/staff-notices/today */
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

/** Ein Baustein an seinem Platz: welcher, wie breit. */
export interface HomeBlockPlacement {
  readonly key: HomeBlockKey;
  readonly span: HomeBlockSpan;
}

/**
 * Die Standardansichten, aus denen jemand startet.
 *
 * Zwei Regeln halten sie brauchbar:
 *
 * 1. KURZ. Die Startseite gibt den Einstieg auf einen Blick, sie zeigt nicht
 *    alles, was es gibt. Vier bis acht Bausteine, nicht mehr.
 * 2. KEINE DOPPLUNG. Zwei Bausteine, die dieselben Zeilen zeigen, gehören
 *    nicht zusammen in einen Standard. „Mein Tag" (die eigenen Einsätze) und
 *    „Ablauf des Tages" (alle Blöcke) sind genau so ein Paar: für eine
 *    Betreuungskraft ist der eigene Tag der interessante Ausschnitt, für die
 *    Leitung der ganze Ablauf. Also bekommt jede Seite genau EINEN von beiden;
 *    der andere steht im Hinzufügen-Menü.
 *
 * Betreuung: der eigene Tag, die Hinweise der Leitung, was in den nächsten
 * Minuten ansteht, und wer gerade betreut.
 *
 * Leitung: die Lage der Schule in vier Zahlen, was auf eine Entscheidung
 * wartet, die Hinweise, der laufende Betrieb und der Ablauf des Tages.
 */
export const DEFAULT_LAYOUTS: Record<
  HomeProfile,
  readonly HomeBlockPlacement[]
> = {
  care: [
    { key: "section.my_day", span: 2 },
    { key: "section.staff_notices", span: 2 },
    { key: "section.reminders", span: 2 },
    { key: "section.active_groups", span: 2 },
    // Geburtstage stehen in BEIDEN Standardansichten: wer heute Geburtstag
    // hat, betrifft die Betreuungskraft am Tisch genauso wie die Leitung, und
    // die Karte doppelt nichts anderes auf der Fläche. Über die volle Breite,
    // weil sie mehrere Kinder nebeneinander zeigt statt untereinander.
    { key: "section.birthdays", span: 4 },
    // Der Ablauf des Tages fehlt hier bewusst — „Mein Tag" zeigt derselben
    // Person dieselben Blöcke, nur auf sie gefiltert. Die offenen Anfragen
    // betreffen nur Gruppenleitungen und Vertretungen. Beides steht im
    // Hinzufügen-Menü.
  ],
  lead: [
    { key: "tile.students_present", span: 1 },
    { key: "tile.students_sick", span: 1 },
    { key: "tile.students_excused", span: 1 },
    { key: "tile.students_home", span: 1 },
    { key: "section.open_requests", span: 2 },
    { key: "section.staff_notices", span: 2 },
    { key: "section.active_groups", span: 2 },
    { key: "section.day_flow", span: 2 },
    { key: "section.birthdays", span: 4 },
  ],
};

/**
 * Welche Standardansicht jemand bekommt.
 *
 * Am Adminzuschnitt festgemacht, nicht am Rollennamen: eine Schule kann ihre
 * Rollen frei anlegen, und die Frage ist nur, ob jemand die Einrichtung führt
 * oder in ihr betreut. Wer beides tut, bekommt die Leitungsansicht und findet
 * "Mein Tag" im Hinzufügen-Menü.
 */
export function homeProfileFor(access: HomeBlockAccess): HomeProfile {
  return access.isAdminScope ? "lead" : "care";
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

  const placements: HomeBlockPlacement[] = [];
  const placed = new Set<HomeBlockKey>();

  const place = (key: HomeBlockKey, span: number | undefined) => {
    if (placed.has(key) || !availableKeys.has(key)) return;
    if (policyOf(key) === "disabled") return;
    const block = BLOCK_BY_KEY.get(key);
    if (!block) return;
    placements.push({ key, span: clampSpan(block, span) });
    placed.add(key);
  };

  for (const entry of stored ?? []) {
    place(entry.key, entry.span);
  }

  const profile = homeProfileFor(ctx.access);
  const arranged = (stored?.length ?? 0) > 0;
  for (const entry of DEFAULT_LAYOUTS[profile]) {
    // Entfernt bleibt entfernt. Alles andere aus der Standardansicht steht am
    // Ende — beim ersten Besuch ist das die ganze Ansicht, später ist es der
    // Weg, auf dem ein neuer Baustein bestehende Konten erreicht.
    if (overrides?.[entry.key] === false) continue;
    place(entry.key, entry.span);
  }

  // Was die Schule verlangt, steht auf jeder Startseite, die es sehen darf.
  for (const block of available) {
    if (policyOf(block.key) === "required") place(block.key, undefined);
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
  for (const entry of raw) {
    if (entry === null || typeof entry !== "object") continue;
    const { key, span } = entry as { key?: unknown; span?: unknown };
    if (!isHomeBlockKey(key) || seen.has(key)) continue;
    const block = BLOCK_BY_KEY.get(key)!;
    result.push({
      key,
      span: clampSpan(block, typeof span === "number" ? span : undefined),
    });
    seen.add(key);
  }
  return result;
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
