// The weekly Heimweg plan of a child: which weekdays the child rides the bus,
// is picked up, walks with another child or goes alone (#1582, #1610, #1694).

export type DepartureDayKey = "mon" | "tue" | "wed" | "thu" | "fri";
export type DepartureMode = "alone" | "bus" | "pickup" | "accompanied";

/** Per-weekday "fährt Bus" flags, derived from the plan by the backend. */
export type BusDays = Partial<Record<DepartureDayKey, boolean>>;
/** Per-weekday "wird abgeholt" flags; an unselected day means "geht alleine". */
export type PickupDays = Partial<Record<DepartureDayKey, boolean>>;
/** One exclusive mode per weekday, the backend's source of truth (#1610). */
export type DepartureDays = Partial<Record<DepartureDayKey, DepartureMode>>;
/** Every mode a weekday allows (#1694). */
export type AllowedDepartureModes = Partial<
  Record<DepartureDayKey, DepartureMode[]>
>;

export const DEPARTURE_WEEKDAYS: ReadonlyArray<{
  key: DepartureDayKey;
  label: string;
}> = [
  { key: "mon", label: "Montag" },
  { key: "tue", label: "Dienstag" },
  { key: "wed", label: "Mittwoch" },
  { key: "thu", label: "Donnerstag" },
  { key: "fri", label: "Freitag" },
] as const;

/** German labels for the departure modes (shown in forms and badges). */
export const DEPARTURE_MODE_LABELS: Record<DepartureMode, string> = {
  alone: "Geht zu Fuß",
  bus: "Fährt Bus",
  pickup: "Wird abgeholt",
  accompanied: "Mit anderem Kind",
};

/** Short labels for the departure modes, used in compact badges (editor pills
 *  and the read-only Stammdaten matrix). */
export const DEPARTURE_MODE_SHORT_LABELS: Record<DepartureMode, string> = {
  alone: "Zu Fuß",
  bus: "Bus",
  pickup: "Abgeholt",
  accompanied: "Anderes Kind",
};

const ALWAYS_ALONE_LABEL = "Geht immer alleine";

/** Canonical render order for departure modes (stable badge ordering). */
const DEPARTURE_MODE_ORDER: readonly DepartureMode[] = [
  "alone",
  "bus",
  "pickup",
  "accompanied",
];

/** The modes that are an arrangement rather than going home alone. */
function isArrangement(mode: DepartureMode | undefined): boolean {
  return mode === "bus" || mode === "pickup" || mode === "accompanied";
}

function weekdayShort(label: string): string {
  return label.slice(0, 2);
}

function dayFlags(
  test: (key: DepartureDayKey) => boolean,
): Partial<Record<DepartureDayKey, boolean>> {
  const out: Partial<Record<DepartureDayKey, boolean>> = {};
  for (const day of DEPARTURE_WEEKDAYS) {
    if (test(day.key)) out[day.key] = true;
  }
  return out;
}

/**
 * The single exclusive mode of a day that allows several. Pickup wins over the
 * bus, mirroring the backend (#1610).
 */
function exclusiveMode(
  modes: readonly DepartureMode[],
): DepartureMode | undefined {
  if (modes.includes("pickup")) return "pickup";
  if (modes.includes("bus")) return "bus";
  if (modes.includes("accompanied")) return "accompanied";
  return undefined;
}

function normalizeDayFlags(
  value?: Partial<Record<DepartureDayKey, boolean>> | null,
): Partial<Record<DepartureDayKey, boolean>> {
  return dayFlags((key) => Boolean(value?.[key]));
}

function anyDayFlagged(
  value?: Partial<Record<DepartureDayKey, boolean>> | null,
): boolean {
  return DEPARTURE_WEEKDAYS.some((day) => Boolean(value?.[day.key]));
}

export const normalizeBusDays: (value?: BusDays | null) => BusDays =
  normalizeDayFlags;
export const busDaysHaveAny: (value?: BusDays | null) => boolean =
  anyDayFlagged;

export const normalizePickupDays: (value?: PickupDays | null) => PickupDays =
  normalizeDayFlags;
export const pickupDaysHaveAny: (value?: PickupDays | null) => boolean =
  anyDayFlagged;

export function normalizeDepartureDays(
  value?: DepartureDays | null,
): DepartureDays {
  const out: DepartureDays = {};
  for (const day of DEPARTURE_WEEKDAYS) {
    const mode = value?.[day.key];
    if (isArrangement(mode)) out[day.key] = mode;
  }
  return out;
}

export function normalizeAllowedDepartureModes(
  value?: AllowedDepartureModes | null,
): AllowedDepartureModes {
  const out: AllowedDepartureModes = {};
  for (const day of DEPARTURE_WEEKDAYS) {
    const raw = value?.[day.key];
    if (!Array.isArray(raw)) continue;
    const modes = DEPARTURE_MODE_ORDER.filter((mode) => raw.includes(mode));
    if (modes.length > 0) out[day.key] = modes;
  }
  return out;
}

/**
 * Stable identity of a departure plan, equal exactly when
 * allowedDepartureModesEqual is.
 *
 * Needed where a plan has to be COMPARED AGAINST ITS OWN EARLIER SELF across an
 * async boundary rather than against another value: the companion refresh hook
 * remembers what a running save submitted, and a plan edited while that save is
 * in flight must not be mistaken for the one it carried (#1694).
 */
export function allowedDepartureModesFingerprint(
  value?: AllowedDepartureModes | null,
): string {
  const normalized = normalizeAllowedDepartureModes(value);
  return DEPARTURE_WEEKDAYS.map(
    (day) => `${day.key}:${(normalized[day.key] ?? []).join(",")}`,
  ).join("|");
}

/** Semantic equality of two departure plans, so shape noise (missing days,
 *  duplicate or unordered mode arrays) does not read as a change (#1694). */
export function allowedDepartureModesEqual(
  a?: AllowedDepartureModes | null,
  b?: AllowedDepartureModes | null,
): boolean {
  return (
    allowedDepartureModesFingerprint(a) === allowedDepartureModesFingerprint(b)
  );
}

/** Reports whether any weekday allows the accompanied ("Mit anderem Kind")
 *  departure mode, on every form shape (#1694). */
export function allowedModesIncludeAccompanied(
  allowed?: AllowedDepartureModes | null,
  departureDays?: DepartureDays | null,
): boolean {
  return accompaniedWeekdayKeys(allowed, departureDays).length > 0;
}

/** The weekdays on which the plan allows the accompanied departure mode. The
 *  "mit wem" requirement is checked PER DAY — a Monday link answers nothing for
 *  an accompanied Tuesday (#1694). */
export function accompaniedWeekdayKeys(
  allowed?: AllowedDepartureModes | null,
  departureDays?: DepartureDays | null,
): DepartureDayKey[] {
  return DEPARTURE_WEEKDAYS.filter(
    (day) =>
      (allowed?.[day.key]?.includes("accompanied") ?? false) ||
      departureDays?.[day.key] === "accompanied",
  ).map((day) => day.key);
}

/** Folds the legacy bus/pickup maps into the unified map (#1610). */
export function departureDaysFromLegacy(
  bus?: BusDays | null,
  pickup?: PickupDays | null,
): DepartureDays {
  const out: DepartureDays = {};
  for (const day of DEPARTURE_WEEKDAYS) {
    const mode = exclusiveMode([
      ...(bus?.[day.key] ? (["bus"] as const) : []),
      ...(pickup?.[day.key] ? (["pickup"] as const) : []),
    ]);
    if (mode) out[day.key] = mode;
  }
  return out;
}

export function allowedDepartureModesFromDeparture(
  value?: DepartureDays | null,
): AllowedDepartureModes {
  const out: AllowedDepartureModes = {};
  for (const [key, mode] of Object.entries(normalizeDepartureDays(value))) {
    out[key as DepartureDayKey] = [mode];
  }
  return out;
}

export function allowedDepartureModesFromLegacy(
  bus?: BusDays | null,
  pickup?: PickupDays | null,
): AllowedDepartureModes {
  const out: AllowedDepartureModes = {};
  for (const day of DEPARTURE_WEEKDAYS) {
    const modes: DepartureMode[] = [];
    if (bus?.[day.key]) modes.push("bus");
    if (pickup?.[day.key]) modes.push("pickup");
    if (modes.length > 0) out[day.key] = modes;
  }
  return out;
}

export function allowedDepartureToBusDays(
  value?: AllowedDepartureModes | null,
): BusDays {
  return dayFlags((key) => value?.[key]?.includes("bus") ?? false);
}

export function allowedDepartureToPickupDays(
  value?: AllowedDepartureModes | null,
): PickupDays {
  return dayFlags((key) => value?.[key]?.includes("pickup") ?? false);
}

export function allowedDepartureToDepartureDays(
  value?: AllowedDepartureModes | null,
): DepartureDays {
  const out: DepartureDays = {};
  for (const day of DEPARTURE_WEEKDAYS) {
    const mode = exclusiveMode(value?.[day.key] ?? []);
    if (mode) out[day.key] = mode;
  }
  return out;
}

/**
 * Compact day-only summary for the "Erlaubte Heimwege" badge: just the
 * weekdays that have any departure mode configured (e.g. "Mo, Di, Fr").
 */
export function formatAllowedDepartureDays(
  value?: AllowedDepartureModes | null,
): string {
  const normalized = normalizeAllowedDepartureModes(value);
  const labels = DEPARTURE_WEEKDAYS.filter(
    (day) => (normalized[day.key]?.length ?? 0) > 0,
  ).map((day) => weekdayShort(day.label));
  return labels.length > 0 ? labels.join(", ") : ALWAYS_ALONE_LABEL;
}

interface DepartureMatrixRow {
  readonly key: DepartureDayKey;
  /** Full weekday label, e.g. "Montag". */
  readonly label: string;
  /** Two-letter weekday label, e.g. "Mo". */
  readonly short: string;
  /** Allowed modes for the day in canonical order; never empty — a day with no
   *  configured arrangement folds to ["alone"]. */
  readonly modes: DepartureMode[];
}

/** The per-weekday rows of the read-only "Erlaubte Heimwege" matrix (Mo–Fr). */
export function departureMatrixRows(
  value?: AllowedDepartureModes | null,
): DepartureMatrixRow[] {
  const normalized = normalizeAllowedDepartureModes(value);
  return DEPARTURE_WEEKDAYS.map((day) => {
    const modes = normalized[day.key] ?? [];
    return {
      key: day.key,
      label: day.label,
      short: weekdayShort(day.label),
      modes: modes.length > 0 ? modes : ["alone"],
    };
  });
}

/** True when at least one weekday has a non-alone arrangement. */
export function hasAnyDepartureArrangement(
  value?: AllowedDepartureModes | null,
): boolean {
  const normalized = normalizeAllowedDepartureModes(value);
  return DEPARTURE_WEEKDAYS.some((day) =>
    (normalized[day.key] ?? []).some(isArrangement),
  );
}
