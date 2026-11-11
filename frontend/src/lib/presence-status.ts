/**
 * The presence status table (#6572): every day status a child can carry
 * besides its location, described once. `LocationBadge` and `PresenceBadge`
 * render what `resolvePresence` returns, and the day-status figures of lists
 * and filters count by `countedDayStatus`. A new status is one more row in
 * `DAY_STATUSES`; nothing else needs to know it.
 */
import type { MotoConcept } from "~/lib/moto-concepts";
import { formatTime } from "~/lib/date-helpers";
import {
  getStudentPresenceBadgePlanning,
  type DayPlanningStudent,
} from "~/lib/day-planning-helper";
import {
  LOCATION_COLORS,
  LOCATION_STATUSES,
  canSeeDetailedLocation,
  getLocationColor,
  getLocationDisplay,
  isAtSchoolLocation,
  isCheckedInLocation,
  isInClassLocation,
  isNotCheckedInLocation,
  isSchoolyardLocation,
  isTransitLocation,
  parseLocation,
  type DisplayMode,
  type StudentLocationContext,
} from "~/lib/location-helper";

/** Everything the badges read: location, day statuses and today's plan. */
export type PresenceStudent = StudentLocationContext & DayPlanningStudent;

/** What today's plan says about the child, resolved from its planning fields. */
interface PlanningFacts {
  readonly notArrivalToday: boolean;
  readonly notArrivalReason: string | null;
}

/** The day-status fields a count needs; lists hand them in with nulls. */
interface DayStatusFlags {
  readonly sick?: boolean | null;
  readonly class_trip?: boolean | null;
  readonly excused?: boolean | null;
}

interface DayStatusRow {
  readonly key: string;
  /** Whether the child carries the status today. */
  readonly isSet: (
    student: DayStatusFlags,
    planning?: PlanningFacts,
  ) => boolean;
  /** Replaces the location pill while the child is not checked in. */
  readonly pill: { readonly label: string; readonly color: string };
  /** The neutral line below the location while the child is checked in (#4371). */
  readonly hint: {
    readonly label: string;
    /**
     * Key of the hint's concept glyph. Only the key lives here: list counts
     * import this table, and the glyph modules stay out of their bundles.
     * `PresenceHint` maps the key to its module, and the compiler asks for
     * the glyph of a new key there.
     */
    readonly concept: MotoConcept["key"];
    readonly dataAttr: string;
  };
  /** Tooltip for pill and hint. */
  readonly title?: (planning: PlanningFacts) => string | null;
  /**
   * The status' own start time. A status with one shows „seit …“ whatever
   * the location; one without leaves the line to the location's rule.
   */
  readonly since?: (
    student: StudentLocationContext,
  ) => string | null | undefined;
  /**
   * Lower wins. `pill`: which status replaces the location at home; also the
   * order of `countedDayStatus`. `hint`: which status becomes the line under
   * a checked-in child; a contradiction to the plan is the actionable one.
   */
  readonly rank: { readonly pill: number; readonly hint: number };
  /** Counts in the day-status figures, filters and groups of the lists. */
  readonly counted: boolean;
  /** A counted status' text in the absence line of the day columns. */
  readonly absence?: string;
}

const DAY_STATUS_TABLE = {
  sick: {
    key: "sick",
    isSet: (student) => !!student.sick,
    pill: { label: LOCATION_STATUSES.SICK, color: LOCATION_COLORS.SICK },
    hint: {
      label: LOCATION_STATUSES.SICK,
      concept: "sick",
      dataAttr: "data-sick-indicator",
    },
    since: (student) => student.sick_since,
    rank: { pill: 0, hint: 1 },
    counted: true,
    absence: "krank gemeldet",
  },
  class_trip: {
    key: "class_trip",
    isSet: (student) => !!student.class_trip,
    pill: {
      label: LOCATION_STATUSES.CLASS_TRIP,
      color: LOCATION_COLORS.CLASS_TRIP,
    },
    hint: {
      label: LOCATION_STATUSES.CLASS_TRIP,
      concept: "classTrip",
      dataAttr: "data-class-trip-indicator",
    },
    since: (student) => student.class_trip_since,
    rank: { pill: 1, hint: 2 },
    counted: true,
    absence: "Klassenfahrt",
  },
  excused: {
    key: "excused",
    isSet: (student) => !!student.excused,
    pill: { label: LOCATION_STATUSES.EXCUSED, color: LOCATION_COLORS.EXCUSED },
    hint: {
      label: LOCATION_STATUSES.EXCUSED,
      concept: "excused",
      dataAttr: "data-excused-indicator",
    },
    since: (student) => student.excused_since,
    rank: { pill: 2, hint: 3 },
    counted: true,
    absence: "entschuldigt",
  },
  // From today's plan, not from a report: "Kommt heute nicht" at home,
  // "Ungeplant anwesend" once the child is here anyway. The hint borrows the
  // glyph of the plan it contradicts.
  not_arrival: {
    key: "not_arrival",
    isSet: (_student, planning) => planning?.notArrivalToday ?? false,
    pill: {
      label: LOCATION_STATUSES.NOT_ARRIVAL,
      color: LOCATION_COLORS.NOT_ARRIVAL,
    },
    hint: {
      label: LOCATION_STATUSES.UNPLANNED_PRESENT,
      concept: "notArrival",
      dataAttr: "data-not-arrival-indicator",
    },
    title: (planning) => planning.notArrivalReason,
    rank: { pill: 3, hint: 0 },
    counted: false,
  },
} as const satisfies Record<string, DayStatusRow>;

export type DayStatusKey = keyof typeof DAY_STATUS_TABLE;

/** The concept keys the table uses for its hint glyphs. */
export type PresenceHintConcept =
  (typeof DAY_STATUS_TABLE)[DayStatusKey]["hint"]["concept"];

/** The statuses `countedDayStatus` can return. */
export type CountedDayStatus = {
  [K in DayStatusKey]: (typeof DAY_STATUS_TABLE)[K]["counted"] extends true
    ? K
    : never;
}[DayStatusKey];

/** The table as a list, in `pill` rank order. */
export const DAY_STATUSES: readonly (DayStatusRow & {
  readonly key: DayStatusKey;
})[] = [...Object.values(DAY_STATUS_TABLE)].sort(
  (a, b) => a.rank.pill - b.rank.pill,
);

const BY_HINT_RANK = [...DAY_STATUSES].sort(
  (a, b) => a.rank.hint - b.rank.hint,
);

/** The hint line for one status; also used by `PresenceHint` on its own. */
export function dayStatusHint(
  key: DayStatusKey,
): (typeof DAY_STATUS_TABLE)[DayStatusKey]["hint"] {
  return DAY_STATUS_TABLE[key].hint;
}

/** The pill text of a status, e.g. for a status column in a list. */
export function dayStatusLabel(key: DayStatusKey): string {
  return DAY_STATUS_TABLE[key].pill.label;
}

/** The absence line text of a counted status („krank gemeldet“). */
export function dayStatusAbsenceLabel(key: CountedDayStatus): string {
  return DAY_STATUS_TABLE[key].absence;
}

/**
 * Der Tagesstatus, nach dem Zahlen, Statusfilter und Statusgruppen ein Kind
 * einordnen, in der Rangfolge der Tabelle (krank vor Klassenfahrt vor
 * entschuldigt).
 *
 * Ort vor Status (#6371): Ein eingechecktes Kind zählt in jeder Ansicht als
 * anwesend, genau einmal. Seine Krankmeldung, Entschuldigung oder Klassenfahrt
 * ist dann nur ein Hinweis am Ortsbadge und zählt nirgends mit. Ein Kind ohne
 * Check-in behält seinen Status.
 */
export function countedDayStatus(
  student: DayStatusFlags & { readonly current_location?: string | null },
): CountedDayStatus | null {
  if (isCheckedInLocation(student.current_location)) return null;
  const row = DAY_STATUSES.find(
    (status) => status.counted && status.isSet(student),
  );
  return (row?.key as CountedDayStatus | undefined) ?? null;
}

/** How the page shows a location: rooms and modes, or only present/absent. */
export type PresenceView =
  | { readonly mode: "binary" }
  | {
      readonly mode: "detailed";
      readonly displayMode: DisplayMode;
      readonly userGroups?: readonly string[];
      /** Rooms of the viewer's own OGS groups (green). */
      readonly groupRooms?: readonly string[];
      readonly supervisedRooms?: readonly string[];
      readonly isGroupRoom?: boolean;
      /**
       * Sonderfall bis #6663: Liegt ein stärkerer Status (krank …) über
       * „Kommt heute nicht“, trägt die Pille trotzdem den Grund des Plans als
       * Tooltip. Ohne die Option gehört der Tooltip zum angezeigten Status.
       * Nur `LocationBadge` setzt sie, damit sich nichts Sichtbares ändert.
       */
      readonly planReasonTitle?: boolean;
    };

type BinaryPresenceState = "anwesend" | "schulhof" | "abwesend";

interface LocationPill {
  readonly label: string;
  readonly color: string;
  /** The location status the „seit“ rule reads. */
  readonly sinceStatus: string;
  readonly presenceState?: BinaryPresenceState;
}

export interface ResolvedPresence {
  readonly pill: {
    readonly label: string;
    readonly color: string;
    readonly title: string | undefined;
    /** `data-location-status`: the parsed status of the raw location. */
    readonly locationStatus: string;
    /** `data-presence-state`, binary view only. */
    readonly presenceState: BinaryPresenceState | undefined;
  };
  /** The clock time for „seit … Uhr“, Berlin wall clock; null shows no line. */
  readonly since: string | null;
  /** The line below the pill, at most one (#4371). */
  readonly hint: {
    readonly status: DayStatusKey;
    readonly title: string | undefined;
  } | null;
}

/**
 * Pill, hint and „seit“ time of one child. Mixes today's plan in from the
 * child's own planning fields, so callers pass the child as they load it.
 */
export function resolvePresence(
  student: PresenceStudent,
  view: PresenceView,
): ResolvedPresence {
  const location =
    view.mode === "binary"
      ? resolveBinaryLocation(student)
      : resolveDetailedLocation(student, view);
  const planning = getStudentPresenceBadgePlanning(student);
  const atHome = isNotCheckedInLocation(student.current_location);
  const isSet = (row: DayStatusRow) => row.isSet(student, planning);
  const titleOf = (row: DayStatusRow) => row.title?.(planning) ?? undefined;

  const replacing = atHome ? DAY_STATUSES.find(isSet) : undefined;
  const planRow = DAY_STATUS_TABLE.not_arrival;
  const pillTitle = () => {
    if (!replacing) return undefined;
    if (view.mode === "detailed" && view.planReasonTitle) {
      return isSet(planRow) ? titleOf(planRow) : undefined;
    }
    return titleOf(replacing);
  };
  const hintRow = atHome ? undefined : BY_HINT_RANK.find(isSet);

  const ownSince = replacing?.since;
  const sinceTime = formatSince(
    (ownSince ? ownSince(student) : null) ?? student.location_since,
  );
  const showsSince =
    ownSince !== undefined || SINCE_STATUSES.has(location.sinceStatus);

  return {
    pill: {
      label: replacing?.pill.label ?? location.label,
      color: replacing?.pill.color ?? location.color,
      title: pillTitle(),
      locationStatus: parseLocation(student.current_location).status,
      presenceState: location.presenceState,
    },
    since: showsSince ? sinceTime : null,
    hint: hintRow ? { status: hintRow.key, title: titleOf(hintRow) } : null,
  };
}

/**
 * „seit …“ belongs to a stay in a room, at home or in class. Never to
 * Schulhof, Unterwegs or Schule (#6563).
 */
const SINCE_STATUSES: ReadonlySet<string> = new Set([
  LOCATION_STATUSES.PRESENT,
  LOCATION_STATUSES.HOME,
  LOCATION_STATUSES.IN_CLASS,
]);

/** Berlin wall clock, whatever the browser's zone; null when unreadable. */
function formatSince(isoTimestamp: string | null | undefined): string | null {
  if (!isoTimestamp || Number.isNaN(Date.parse(isoTimestamp))) return null;
  return formatTime(isoTimestamp);
}

function resolveDetailedLocation(
  student: StudentLocationContext,
  view: Extract<PresenceView, { mode: "detailed" }>,
): LocationPill {
  const userGroups = view.userGroups ? [...view.userGroups] : undefined;
  const groupRooms = view.groupRooms ? [...view.groupRooms] : undefined;
  const supervisedRooms = view.supervisedRooms
    ? [...view.supervisedRooms]
    : undefined;
  const label = getLocationDisplay(
    student,
    view.displayMode,
    userGroups,
    supervisedRooms,
  );
  const sinceStatus = parseLocation(student.current_location).status;

  if (view.displayMode === "groupName") {
    return { label, color: LOCATION_COLORS.GROUP_ROOM, sinceStatus };
  }

  const ownChild =
    view.displayMode !== "contextAware" ||
    canSeeDetailedLocation(student, userGroups, supervisedRooms);
  // A viewer without detailed access sees only the coarse status, so the
  // colour follows that label (Anwesend green, Zuhause gray, never a room
  // colour). A plain "Schulhof" is a status, not room detail, and keeps the
  // school's yard colour (#2405).
  const color = ownChild
    ? getLocationColor(
        student.current_location,
        view.isGroupRoom,
        groupRooms,
        student.current_room_color,
      )
    : getLocationColor(label, false, [], student.current_room_color);
  return { label, color, sinceStatus };
}

/**
 * Binary mode knows three states: Anwesend (green), Schulhof (orange) and
 * absent, which reads Zuhause, Schule before the first check-in (#3260) or
 * Im Unterricht. Detailed-mode locations (rooms, Unterwegs) collapse into
 * Anwesend.
 */
function resolveBinaryLocation(student: StudentLocationContext): LocationPill {
  const location = student.current_location;
  const parsed = parseLocation(location);
  const presenceState: BinaryPresenceState = isNotCheckedInLocation(location)
    ? "abwesend"
    : isSchoolyardLocation(location)
      ? "schulhof"
      : "anwesend";
  const { label, color } = binaryLabel(presenceState, student);
  // A room name has no status prefix but reads „Anwesend“ here. Unknown and
  // missing locations must not inherit that time.
  const sinceStatus =
    parsed.status === LOCATION_STATUSES.UNKNOWN
      ? parsed.status
      : isTransitLocation(location)
        ? LOCATION_STATUSES.TRANSIT
        : label;
  return { label, color, sinceStatus, presenceState };
}

function binaryLabel(
  state: BinaryPresenceState,
  student: StudentLocationContext,
): { label: string; color: string } {
  switch (state) {
    case "anwesend":
      return {
        label: LOCATION_STATUSES.PRESENT,
        color: LOCATION_COLORS.GROUP_ROOM,
      };
    case "schulhof":
      return {
        label: LOCATION_STATUSES.SCHOOLYARD,
        // The yard follows the Schulhof room's own colour when one is set
        // (#2405); the backend ships it in `current_room_color`.
        color: student.current_room_color?.trim()
          ? student.current_room_color
          : LOCATION_COLORS.SCHOOLYARD,
      };
    case "abwesend":
      if (isAtSchoolLocation(student.current_location)) {
        return {
          label: LOCATION_STATUSES.AT_SCHOOL,
          color: LOCATION_COLORS.AT_SCHOOL,
        };
      }
      if (isInClassLocation(student.current_location)) {
        return {
          label: LOCATION_STATUSES.IN_CLASS,
          color: LOCATION_COLORS.IN_CLASS,
        };
      }
      return { label: LOCATION_STATUSES.HOME, color: LOCATION_COLORS.HOME };
  }
}
