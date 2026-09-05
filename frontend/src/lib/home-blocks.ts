/**
 * Baustein-Register der Startseite (#2875).
 *
 * Die Startseite ist keine feste Bildschirmfläche, sondern eine Auswahl aus
 * Bausteinen. Jeder Baustein sagt hier, in welchem Betriebsmodus er überhaupt
 * Sinn ergibt (Anwesenheitsmodus, offene Betreuung, NFC, Geburtstage) und ob er
 * ohne eigene Entscheidung sichtbar ist.
 *
 * Drei Ebenen entscheiden, in dieser Reihenfolge:
 *
 *   1. Verfügbarkeit  — Betriebsmodus der Schule. Was hier wegfällt, gibt es
 *                       für diese Schule schlicht nicht.
 *   2. Vorgabe        — die Einrichtung kann einen Baustein verpflichtend
 *                       machen oder ganz abschalten.
 *   3. Eigene Auswahl — alles, was danach noch offen ist.
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

/** Was ein Baustein zum Entscheiden braucht, ohne Session- oder Kontext-Typen. */
export interface HomeBlockContext {
  /** Anwesenheitsmodus "detailed" (Räume, Wege) statt "binary" (da/nicht da). */
  readonly detailed: boolean;
  /** Offene Betreuung ohne feste Gruppen (#1544). */
  readonly openCareGroupMode: boolean;
  readonly nfcEnabled: boolean;
  /** Geburtstage sind pro Schule abschaltbar und kommen vom Server. */
  readonly birthdaysEnabled: boolean;
}

export interface HomeBlockDefinition {
  readonly key: HomeBlockKey;
  readonly kind: HomeBlockKind;
  /** Name im Dialog "Startseite anpassen". */
  readonly label: string;
  /** Ein Satz, was der Baustein zeigt. */
  readonly description: string;
  /** Gibt es den Baustein in dieser Schule? Reiner Betriebsmodus. */
  readonly available: (ctx: HomeBlockContext) => boolean;
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
  | "section.active_groups";

const always = () => true;
const roomSurfaces = (ctx: HomeBlockContext) => ctx.detailed;
const activitySurfaces = (ctx: HomeBlockContext) =>
  ctx.detailed && ctx.nfcEnabled;

export const HOME_BLOCKS: readonly HomeBlockDefinition[] = [
  // ---- Kennzahlen ---------------------------------------------------------
  {
    key: "tile.students_present",
    kind: "tile",
    label: "Kinder anwesend",
    description: "Wie viele Kinder gerade eingecheckt sind.",
    available: always,
    defaultVisible: true,
  },
  {
    key: "tile.students_in_rooms",
    kind: "tile",
    label: "In Räumen",
    description: "Anwesende Kinder, die gerade in einem Raum sind.",
    available: roomSurfaces,
    defaultVisible: true,
  },
  {
    key: "tile.students_in_transit",
    kind: "tile",
    label: "Unterwegs",
    description: "Anwesende Kinder ohne Raum, zum Beispiel auf dem Weg.",
    available: roomSurfaces,
    defaultVisible: true,
  },
  {
    key: "tile.students_on_playground",
    kind: "tile",
    label: "Schulhof",
    description: "Kinder, die gerade auf dem Schulhof sind.",
    available: always,
    defaultVisible: true,
  },
  {
    key: "tile.students_sick",
    kind: "tile",
    label: "Krank",
    description: "Kinder, die heute krank gemeldet sind.",
    available: always,
    defaultVisible: true,
  },
  {
    key: "tile.students_excused",
    kind: "tile",
    label: "Entschuldigt",
    description: "Kinder, die heute entschuldigt fehlen.",
    available: always,
    defaultVisible: true,
  },
  {
    key: "tile.students_home",
    kind: "tile",
    label: "Zuhause",
    description: "Kinder, die heute nicht in der Betreuung sind.",
    available: always,
    defaultVisible: true,
  },
  {
    key: "tile.active_activities",
    kind: "tile",
    label: "Aktive Aktivitäten",
    description: "Wie viele Aktivitäten gerade laufen.",
    available: activitySurfaces,
    defaultVisible: true,
  },
  {
    key: "tile.capacity_utilization",
    kind: "tile",
    label: "Auslastung",
    description: "Belegte Plätze in den Räumen in Prozent.",
    available: roomSurfaces,
    defaultVisible: true,
  },

  // ---- Bereiche -----------------------------------------------------------
  {
    key: "section.birthdays",
    kind: "section",
    label: "Geburtstage",
    description: "Wer heute oder in den nächsten Tagen Geburtstag hat.",
    available: (ctx) => ctx.birthdaysEnabled,
    defaultVisible: true,
  },
  {
    key: "section.recent_activity",
    kind: "section",
    label: "Letzte Bewegungen",
    description: "Welche Gruppen zuletzt den Raum gewechselt haben.",
    available: roomSurfaces,
    defaultVisible: true,
  },
  {
    key: "section.current_activities",
    kind: "section",
    label: "Laufende Aktivitäten",
    description: "Welche Aktivitäten gerade stattfinden und wie voll sie sind.",
    available: activitySurfaces,
    defaultVisible: true,
  },
  {
    key: "section.active_groups",
    kind: "section",
    label: "Aktive Gruppen",
    description: "Welche Gruppen gerade betreut werden und wo.",
    available: (ctx) => !ctx.openCareGroupMode,
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
 * Wendet Betriebsmodus, Vorgabe der Schule und persönliche Auswahl an.
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
  const available = HOME_BLOCKS.filter((block) => block.available(ctx));
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

export function isHomeBlockKey(value: unknown): value is HomeBlockKey {
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
