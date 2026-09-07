/**
 * Baustein-Register der Startseite (#2875).
 *
 * Die Startseite ist keine feste Bildschirmfläche, sondern eine Auswahl aus
 * Bausteinen. Jeder Baustein sagt hier, in welchem Betriebsmodus er überhaupt
 * Sinn ergibt (Anwesenheitsmodus, offene Betreuung, NFC, Geburtstage) und ob er
 * ohne eigene Entscheidung sichtbar ist.
 *
 * Vier Ebenen entscheiden, in dieser Reihenfolge:
 *
 *   1. Berechtigung   — was die Person abrufen darf. Jede Regel spiegelt das
 *                       Gate des Endpunkts hinter dem Baustein (#2180), damit
 *                       eine frei angelegte Rolle ohne Codeänderung eine
 *                       brauchbare Startseite bekommt.
 *   2. Verfügbarkeit  — Betriebsmodus der Schule. Was hier wegfällt, gibt es
 *                       für diese Schule schlicht nicht.
 *   3. Vorgabe        — die Einrichtung kann einen Baustein verpflichtend
 *                       machen oder ganz abschalten.
 *   4. Eigene Auswahl — alles, was danach noch offen ist.
 *
 * Der Katalog lebt bewusst hier und nicht im Backend: nur das Frontend kennt
 * Beschriftung, Betriebsmodus und die Datenquelle eines Bausteins. Der Server
 * prüft die Schlüssel nur auf ihre Form. Beide Speicher halten ausschliesslich
 * ABWEICHUNGEN, damit eine später ergänzte Kachel bestehende Konten in ihrem
 * gedachten Standardzustand erreicht, statt für jeden zu verschwinden, der den
 * Dialog je geöffnet hat.
 */

/** Was die Einrichtung für einen Baustein vorgibt. */
export type HomeBlockPolicy = "optional" | "required" | "disabled";

/** Abweichungen der Person: true = eingeblendet, false = ausgeblendet. */
export type HomeLayoutOverrides = Record<string, boolean>;

/** Vorgaben der Schule. Ein fehlender Eintrag heisst "frei wählbar". */
export type HomeBlockPolicies = Record<string, HomeBlockPolicy>;

type HomeBlockKind = "tile" | "section";

/**
 * Die drei Zonen der Startseite, in der Reihenfolge, in der sie stehen.
 *
 * Die Zone ist Ordnung, keine Berechtigung: wer von einer Zone nichts sehen
 * darf, bekommt sie gar nicht erst zu sehen, statt einer leeren Überschrift.
 */
type HomeBlockZone = "today" | "operations" | "todo";

/**
 * Was die Person darf — genau so viel, wie der Katalog zum Entscheiden
 * braucht. `has` prüft ein einzelnes Tenant-Recht, `isAdminScope` steht für
 * den Adminzuschnitt, den einzelne Bausteine zusätzlich brauchen.
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
}

/** Betriebsmodus plus die Rechte der angemeldeten Person (#2180). */
export interface HomeBlockContext extends HomeBlockModeContext {
  readonly access: HomeBlockAccess;
}

export interface HomeBlockDefinition {
  readonly key: HomeBlockKey;
  readonly kind: HomeBlockKind;
  /** Name im Dialog "Startseite anpassen". */
  readonly label: string;
  /** Ein Satz, was der Baustein zeigt. */
  readonly description: string;
  /** In welcher Zone der Startseite der Baustein steht. */
  readonly zone: HomeBlockZone;
  /**
   * Darf die Person die Daten des Bausteins abrufen? Spiegelt das Gate des
   * Endpunkts dahinter; ohne das Recht liefert der Server ohnehin nichts.
   */
  readonly permitted: (access: HomeBlockAccess) => boolean;
  /** Gibt es den Baustein in dieser Schule? Reiner Betriebsmodus. */
  readonly available: (ctx: HomeBlockModeContext) => boolean;
  /** Sichtbar, solange niemand etwas anderes entschieden hat. */
  readonly defaultVisible: boolean;
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
  | "section.open_requests";

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

export const HOME_BLOCKS: readonly HomeBlockDefinition[] = [
  // ---- Kennzahlen ---------------------------------------------------------
  {
    key: "tile.students_present",
    kind: "tile",
    label: "Kinder anwesend",
    description: "Wie viele Kinder gerade eingecheckt sind.",
    zone: "operations",
    permitted: operationalNumbers,
    available: always,
    defaultVisible: true,
  },
  {
    key: "tile.students_in_rooms",
    kind: "tile",
    label: "In Räumen",
    description: "Anwesende Kinder, die gerade in einem Raum sind.",
    zone: "operations",
    permitted: operationalNumbers,
    available: roomSurfaces,
    defaultVisible: true,
  },
  {
    key: "tile.students_in_transit",
    kind: "tile",
    label: "Unterwegs",
    description: "Anwesende Kinder ohne Raum, zum Beispiel auf dem Weg.",
    zone: "operations",
    permitted: operationalNumbers,
    available: roomSurfaces,
    defaultVisible: true,
  },
  {
    key: "tile.students_on_playground",
    kind: "tile",
    label: "Schulhof",
    description: "Kinder, die gerade auf dem Schulhof sind.",
    zone: "operations",
    permitted: operationalNumbers,
    available: always,
    defaultVisible: true,
  },
  {
    key: "tile.students_sick",
    kind: "tile",
    label: "Krank",
    description: "Kinder, die heute krank gemeldet sind.",
    zone: "operations",
    permitted: operationalNumbers,
    available: always,
    defaultVisible: true,
  },
  {
    key: "tile.students_excused",
    kind: "tile",
    label: "Entschuldigt",
    description: "Kinder, die heute entschuldigt fehlen.",
    zone: "operations",
    permitted: operationalNumbers,
    available: always,
    defaultVisible: true,
  },
  {
    key: "tile.students_home",
    kind: "tile",
    label: "Zuhause",
    description: "Kinder, die heute nicht in der Betreuung sind.",
    zone: "operations",
    permitted: operationalNumbers,
    available: always,
    defaultVisible: true,
  },
  {
    key: "tile.active_activities",
    kind: "tile",
    label: "Aktive Aktivitäten",
    description: "Wie viele Aktivitäten gerade laufen.",
    zone: "operations",
    permitted: operationalNumbers,
    available: activitySurfaces,
    defaultVisible: true,
  },
  {
    key: "tile.capacity_utilization",
    kind: "tile",
    label: "Auslastung",
    description: "Belegte Plätze in den Räumen in Prozent.",
    zone: "operations",
    permitted: operationalNumbers,
    available: roomSurfaces,
    defaultVisible: true,
  },

  // ---- Bereiche -----------------------------------------------------------
  {
    key: "section.birthdays",
    kind: "section",
    label: "Geburtstage",
    description: "Wer heute oder in den nächsten Tagen Geburtstag hat.",
    zone: "operations",
    permitted: (access) => access.has(PERMISSION.usersRead),
    available: (ctx) => ctx.birthdaysEnabled,
    defaultVisible: true,
  },
  {
    key: "section.recent_activity",
    kind: "section",
    label: "Letzte Bewegungen",
    description: "Welche Gruppen zuletzt den Raum gewechselt haben.",
    zone: "operations",
    permitted: operationalNumbers,
    available: roomSurfaces,
    defaultVisible: true,
  },
  {
    key: "section.current_activities",
    kind: "section",
    label: "Laufende Aktivitäten",
    description: "Welche Aktivitäten gerade stattfinden und wie voll sie sind.",
    zone: "operations",
    permitted: operationalNumbers,
    available: activitySurfaces,
    defaultVisible: true,
  },
  // ---- Heute für mich ----------------------------------------------------
  {
    key: "section.my_day",
    kind: "section",
    label: "Mein Tag",
    description: "Ihre heutigen Einsätze mit Ort, Zeit und Vertretungen.",
    zone: "today",
    // /api/time-tracking/assignments (#1844) liefert nur die eigenen Blöcke.
    permitted: (access) => access.has(PERMISSION.timeTrackingOwn),
    available: (ctx) => ctx.timetableEnabled,
    defaultVisible: true,
  },
  {
    key: "section.staff_notices",
    kind: "section",
    label: "Tagesinformationen",
    description: "Hinweise der Leitung, die heute gelten.",
    zone: "today",
    permitted: (access) => access.has(PERMISSION.usersRead),
    available: always,
    defaultVisible: true,
  },

  // ---- Zu erledigen ------------------------------------------------------
  {
    key: "section.open_requests",
    kind: "section",
    label: "Offene Anfragen",
    description: "Wünsche von Eltern und Anträge des Teams, die auf eine Entscheidung warten.",
    zone: "todo",
    permitted: anyRequestQueue,
    available: always,
    defaultVisible: true,
  },

  {
    key: "section.active_groups",
    kind: "section",
    label: "Aktive Gruppen",
    description: "Welche Gruppen gerade betreut werden und wo.",
    zone: "operations",
    permitted: operationalNumbers,
    available: (ctx) => !ctx.openCareGroupMode,
    defaultVisible: true,
  },
  {
    key: "section.day_flow",
    kind: "section",
    label: "Ablauf des Tages",
    description: "Die Blöcke des Betreuungsplans, die gerade laufen oder als Nächstes anstehen.",
    zone: "operations",
    permitted: (access) => access.has(PERMISSION.schedulesRead),
    // Ohne Betreuungsplan gibt es keinen Ablauf, und im Anwesenheitsmodus
    // "binary" plant die Schule keine Blöcke.
    available: (ctx) => ctx.timetableEnabled && ctx.detailed,
    defaultVisible: true,
  },
];

export interface ResolvedHomeBlocks {
  /** Alles, was es in dieser Schule gibt — auch, was die Leitung festgelegt hat. */
  readonly available: readonly HomeBlockDefinition[];
  /** Was die Person selbst ein- und ausblenden darf. */
  readonly adjustable: readonly HomeBlockDefinition[];
  /** Was gerade gezeigt wird. */
  readonly visible: ReadonlySet<HomeBlockKey>;
  /** Weicht die Person vom Standard ab? Steuert "Zurücksetzen". */
  readonly customized: boolean;
}

/**
 * Wendet Berechtigung, Betriebsmodus, Vorgabe der Schule und persönliche
 * Auswahl an.
 *
 * Die Berechtigung schlägt alles: eine verpflichtende Kachel bleibt
 * unsichtbar, wenn die Person die Daten dahinter nicht abrufen darf.
 *
 * Die Vorgabe der Schule schlägt die persönliche Auswahl: eine verpflichtende
 * Kachel ist sichtbar, eine deaktivierte verschwindet, und beide stehen nicht
 * mehr im Dialog. Ein gespeicherter Eintrag dazu wird ignoriert statt gelöscht,
 * damit die ursprüngliche Wahl wieder gilt, wenn die Leitung ihre Entscheidung
 * zurücknimmt.
 */
export function resolveHomeBlocks(
  ctx: HomeBlockContext,
  overrides: HomeLayoutOverrides | null | undefined,
  policies: HomeBlockPolicies | null | undefined,
): ResolvedHomeBlocks {
  // Berechtigung zuerst: was die Person nicht abrufen darf, steht auch nicht
  // im Dialog und kann nicht per Schulvorgabe eingeblendet werden.
  const available = HOME_BLOCKS.filter(
    (block) => block.permitted(ctx.access) && block.available(ctx),
  );
  const adjustable: HomeBlockDefinition[] = [];
  const visible = new Set<HomeBlockKey>();
  // A stored deviation remains resettable while a school policy or operating
  // mode temporarily hides its block. Otherwise a person could no longer
  // clear a hidden choice that becomes relevant again later.
  const customized = HOME_BLOCKS.some((block) => {
    const override = overrides?.[block.key];
    return override !== undefined && override !== block.defaultVisible;
  });

  for (const block of available) {
    const policy = policies?.[block.key] ?? "optional";

    if (policy === "disabled") continue;
    if (policy === "required") {
      visible.add(block.key);
      continue;
    }

    adjustable.push(block);
    const override = overrides?.[block.key];
    const shown = override ?? block.defaultVisible;
    if (shown) visible.add(block.key);
  }

  return { available, adjustable, visible, customized };
}

/**
 * Sichtbarkeit eines einzelnen Bausteins, ohne die ganze Auflösung zu bemühen.
 *
 * Dafür da, eine Datenabfrage zu verhindern, bevor überhaupt jemand die
 * zugehörige Karte sieht (#2875).
 */
export function isHomeBlockVisible(
  ctx: HomeBlockContext,
  overrides: HomeLayoutOverrides | null | undefined,
  policies: HomeBlockPolicies | null | undefined,
  key: HomeBlockKey,
): boolean {
  return resolveHomeBlocks(ctx, overrides, policies).visible.has(key);
}

function isHomeBlockKey(value: unknown): value is HomeBlockKey {
  return (
    typeof value === "string" &&
    HOME_BLOCKS.some((block) => block.key === value)
  );
}

function isHomeBlockPolicy(value: unknown): value is HomeBlockPolicy {
  return value === "optional" || value === "required" || value === "disabled";
}

/**
 * Verwirft unbekannte Schlüssel und falsche Werte aus gespeicherten Daten.
 *
 * Ein Baustein, den es nicht mehr gibt, darf keine Rolle mehr spielen — weder
 * im Dialog noch beim nächsten Speichern.
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
