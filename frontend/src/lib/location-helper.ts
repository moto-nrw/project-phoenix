/**
 * Centralized helpers for parsing, styling, and rendering student location data.
 */

import { MOTO_COLOR_PALETTE } from "~/components/ui/kit-palette";

interface ParsedLocation {
  status: string;
  room?: string;
}

export type DisplayMode = "groupName" | "roomName" | "contextAware";

export interface StudentLocationContext {
  current_location?: string | null;
  location_since?: string | null;
  /**
   * Hex color of the student's current room when set. Backend serves this in
   * `current_room_color` for visit/student responses; the active-supervisions
   * page derives it from the current room object directly. Drives the room
   * badge color so the OGS list isn't a wall of identical blue badges anymore.
   * Nil for status-only locations (Schulhof / Unterwegs / Zuhause / sick / etc.)
   * and for rooms without a custom color set.
   *
   * **Maintenance note**: any new SWR cache that stores rows containing this
   * field (or any other room-derived attribute like the room name baked into
   * `current_location`) MUST register its key substring in
   * `lib/swr/room-derived-caches.ts`. Otherwise badge colours stay stale
   * after an admin saves a room until the cache happens to refetch.
   */
  current_room_color?: string | null;
  group_id?: string | null;
  group_name?: string | null;
  sick?: boolean;
  sick_since?: string | null;
  excused?: boolean;
  excused_since?: string | null;
  class_trip?: boolean;
  class_trip_since?: string | null;
  has_full_access?: boolean;
  /** Today has an arrival-schedule exception with null expected time — student is not coming */
  not_arrival_today?: boolean;
  not_arrival_reason?: string | null;
}

export const LOCATION_STATUSES = {
  PRESENT: "Anwesend",
  HOME: "Zuhause",
  // Erwartetes Kind vor dem ersten Check-in des Tages (#3260): es sitzt noch
  // im Unterricht, nicht zu Hause. Das Backend liefert den Wert selbst.
  AT_SCHOOL: "Schule",
  // Mit „Geht zum Unterricht“ abgemeldet und noch nicht zurück (#5656). Anders
  // als AT_SCHOOL steht dahinter eine erfasste Abmeldung, keine Planung.
  IN_CLASS: "Im Unterricht",
  SCHOOLYARD: "Schulhof",
  TRANSIT: "Unterwegs",
  UNKNOWN: "Unbekannt",
  SICK: "Krank",
  EXCUSED: "Entschuldigt",
  CLASS_TRIP: "Klassenfahrt",
  NOT_ARRIVAL: "Kommt heute nicht",
  UNPLANNED_PRESENT: "Ungeplant anwesend",
} as const;

export const LOCATION_COLORS = {
  GROUP_ROOM: MOTO_COLOR_PALETTE.green.base,
  OTHER_ROOM: MOTO_COLOR_PALETTE.blue.base,
  HOME: MOTO_COLOR_PALETTE.neutral.base,
  SCHOOLYARD: MOTO_COLOR_PALETTE.orange.base,
  TRANSIT: MOTO_COLOR_PALETTE.magenta.base,
  UNKNOWN: MOTO_COLOR_PALETTE.stone.base,
  SICK: MOTO_COLOR_PALETTE.red.base,
  EXCUSED: MOTO_COLOR_PALETTE.purple.base, // Purple - excused absence (kind is not attending today)
  CLASS_TRIP: MOTO_COLOR_PALETTE.cyan.base,
  NOT_ARRIVAL: MOTO_COLOR_PALETTE.navy.base,
  AT_SCHOOL: MOTO_COLOR_PALETTE.petrol.base,
  IN_CLASS: MOTO_COLOR_PALETTE.indigo.base,
  DANGER: MOTO_COLOR_PALETTE.red.base,
  // Amber "needs attention, but nothing is wrong yet": a pending request, an
  // unstaffed slot, an open invitation. Exists because SICK used to be amber
  // and half the app borrowed it as the warning hue; SICK is red now, so those
  // call sites would otherwise be indistinguishable from DANGER.
  WARNING: MOTO_COLOR_PALETTE.amber.base,
} as const;

/**
 * Legacy room colours (#6573) whose badge keeps a label the kit palette no
 * longer derives. #FF3130 was the Zuhause colour before the palette move; the
 * backend still reserves it (`reservedRoomColors` in
 * `backend/modules/facilities/room_lifecycle.go`), so rooms created before
 * that can carry it. Their label has always read #c62826 (5.4:1 on the gray-50
 * pill). The badge hands the kit that shade instead of the room hex: it lies
 * outside the palette and clears the contrast target, so the kit keeps it.
 */
const LEGACY_ROOM_BADGE_COLORS: Readonly<Record<string, string>> = {
  "#FF3130": "#c62826",
};

/**
 * The colour a location badge shows for a room's own colour: the room hex,
 * except for the legacy colours above.
 */
export function getRoomBadgeColor(roomColor: string): string {
  return LEGACY_ROOM_BADGE_COLORS[roomColor.trim().toUpperCase()] ?? roomColor;
}

/**
 * Hover/active/text/background shade variants of the brand green
 * (LOCATION_COLORS.GROUP_ROOM). Centralised so the school check-in toggle and
 * any future "primary action" buttons stop hardcoding hex literals — see
 * CLAUDE.md §0 (route every brand color through LOCATION_COLORS).
 */
export const GROUP_ROOM_SHADES = {
  base: MOTO_COLOR_PALETTE.green.base,
  hover: MOTO_COLOR_PALETTE.green.hover,
  active: MOTO_COLOR_PALETTE.green.active,
  fill: MOTO_COLOR_PALETTE.green.fill,
  text: MOTO_COLOR_PALETTE.green.strong,
  bgHover: MOTO_COLOR_PALETTE.green.soft,
  bgActive: MOTO_COLOR_PALETTE.green.muted,
} as const;

const LOCATION_SEPARATOR = "-";
const UNKNOWN_STATUS = LOCATION_STATUSES.UNKNOWN;

const LEGACY_STATUS_MAP: Record<string, string> = {
  abwesend: LOCATION_STATUSES.HOME,
  zuhause: LOCATION_STATUSES.HOME,
  home: LOCATION_STATUSES.HOME,
  schule: LOCATION_STATUSES.AT_SCHOOL,
  "im unterricht": LOCATION_STATUSES.IN_CLASS,
  unbekannt: LOCATION_STATUSES.UNKNOWN,
  unknown: LOCATION_STATUSES.UNKNOWN,
  anwesend: LOCATION_STATUSES.PRESENT,
  "in house": LOCATION_STATUSES.PRESENT,
  unterwegs: LOCATION_STATUSES.TRANSIT,
  bus: LOCATION_STATUSES.TRANSIT,
  schulhof: LOCATION_STATUSES.SCHOOLYARD,
  "school yard": LOCATION_STATUSES.SCHOOLYARD,
  schoolyard: LOCATION_STATUSES.SCHOOLYARD,
};

function normalizeStatusKeyword(rawStatus: string): string {
  const trimmed = rawStatus.trim();
  if (trimmed.length === 0) {
    return UNKNOWN_STATUS;
  }

  const key = trimmed.toLowerCase();

  if (key === "wc" || key === "bathroom" || key === "toilette") {
    return `${LOCATION_STATUSES.PRESENT} - Toilette`;
  }

  const mapped = LEGACY_STATUS_MAP[key];
  if (mapped) {
    return mapped;
  }

  return trimmed.charAt(0).toUpperCase() + trimmed.slice(1);
}

export function normalizeLocation(location?: string | null): string {
  const raw = (location ?? "").trim();
  if (raw.length === 0) {
    return UNKNOWN_STATUS;
  }

  const separatorIndex = raw.indexOf(LOCATION_SEPARATOR);
  if (separatorIndex === -1) {
    return normalizeStatusKeyword(raw);
  }

  const status = raw.slice(0, separatorIndex).trim();
  const room = raw.slice(separatorIndex + 1).trim();
  const normalizedStatus = normalizeStatusKeyword(status);

  if (room.length === 0) {
    return normalizedStatus;
  }

  const normalizedRoom = room.toLowerCase() === "wc" ? "Toilette" : room;

  return `${normalizedStatus} - ${normalizedRoom}`;
}

/**
 * Splits a location string into its status and optional room component.
 */
export function parseLocation(location?: string | null): ParsedLocation {
  const normalized = normalizeLocation(location);
  const raw = normalized.trim();

  const separatorIndex = raw.indexOf(LOCATION_SEPARATOR);
  if (separatorIndex === -1) {
    return { status: raw };
  }

  const status = raw.slice(0, separatorIndex).trim() || UNKNOWN_STATUS;
  const room = raw.slice(separatorIndex + 1).trim();

  return room.length > 0 ? { status, room } : { status };
}

// Status-based color lookup for simple cases
const STATUS_COLOR_MAP: Record<string, string> = {
  [LOCATION_STATUSES.HOME]: LOCATION_COLORS.HOME,
  [LOCATION_STATUSES.AT_SCHOOL]: LOCATION_COLORS.AT_SCHOOL,
  [LOCATION_STATUSES.IN_CLASS]: LOCATION_COLORS.IN_CLASS,
  [LOCATION_STATUSES.SCHOOLYARD]: LOCATION_COLORS.SCHOOLYARD,
  [LOCATION_STATUSES.TRANSIT]: LOCATION_COLORS.TRANSIT,
};

/**
 * Checks if a student's room matches any of the provided group rooms.
 */
function isStudentInGroupRoom(
  studentRoom: string,
  groupRooms: string[],
): boolean {
  const normalizedStudentRoom = studentRoom.trim().toLowerCase();
  return groupRooms.some(
    (groupRoom) => groupRoom.trim().toLowerCase() === normalizedStudentRoom,
  );
}

/**
 * Determines the color for a student who is present with a room assignment.
 *
 * Priority is intentional:
 *   1. Eigener-Gruppenraum-Grün — viewer-relative signal, kept as-is per the
 *      hard constraint that all non-blue colors keep their meaning.
 *   2. Per-room hex from the backend (`roomColor`) — the new differentiator
 *      that replaces the painful "every other room is the same blue" symptom.
 *   3. Schulhof-Orange — the yard is a real room in detailed mode, so without
 *      this it would fall through to the generic room blue and read
 *      differently from the same child in binary mode (#2405).
 *   4. OTHER_ROOM blue fallback — bestehende Räume ohne gesetzte Farbe sehen
 *      genauso aus wie heute, kein visueller Sprung.
 */
function getColorForPresentWithRoom(
  room: string,
  isGroupRoom?: boolean,
  groupRooms?: string[],
  roomColor?: string | null,
): string {
  // Check if room is one of the user's OGS group rooms
  if (
    groupRooms &&
    groupRooms.length > 0 &&
    isStudentInGroupRoom(room, groupRooms)
  ) {
    return LOCATION_COLORS.GROUP_ROOM; // Green - in their group's room
  }

  // Fallback to isGroupRoom prop if groupRooms not provided
  if (isGroupRoom) {
    return LOCATION_COLORS.GROUP_ROOM; // Green - in their group's room
  }

  // Per-room override — empty string is treated like null (the rooms.config
  // form sends "" when the user clears the picker, the backend translates
  // both to NULL; either way the badge falls back to OTHER_ROOM blue).
  if (roomColor && roomColor.length > 0) {
    return getRoomBadgeColor(roomColor);
  }

  // The Schulhof arrives here as an ordinary room ("Anwesend - Schulhof"),
  // so its documented default has to be applied explicitly — otherwise the
  // yard is indistinguishable from any other uncoloured room.
  if (isSchulhofRoomName(room)) {
    return LOCATION_COLORS.SCHOOLYARD;
  }

  // Student in any other room
  return LOCATION_COLORS.OTHER_ROOM; // Blue - in external/supervised room
}

/**
 * Matches the canonical Schulhof room name.
 *
 * Case-insensitive on purpose: this is a display decision, not the backend's
 * exact-case reservation check, and the location string is assembled from a
 * room name that older data may hold in a different casing.
 */
function isSchulhofRoomName(room: string): boolean {
  return room.trim().toLowerCase() === "schulhof";
}

/**
 * Determines the hex color for a given location value and context.
 *
 * Color rules:
 * - GREEN: Student in their OGS group's room OR "Anwesend" without room details
 * - BLUE: Student in any other room (external room or supervised room)
 * - GRAY/ORANGE/MAGENTA: Status-based (Home, Schoolyard, Transit)
 */
export function getLocationColor(
  location?: string | null,
  isGroupRoom?: boolean,
  groupRooms?: string[],
  roomColor?: string | null,
): string {
  const parsed = parseLocation(location);
  const status = parsed.status;

  // The yard is a configurable room since #2405, so a status-only "Schulhof"
  // (binary mode has no room visit behind it — the backend resolves the room
  // once and ships its hex in `current_room_color`) still follows the school's
  // colour. Unset falls through to the orange default below.
  if (
    status === LOCATION_STATUSES.SCHOOLYARD &&
    roomColor &&
    roomColor.length > 0
  ) {
    return getRoomBadgeColor(roomColor);
  }

  // Check status-based colors first (Home, Schoolyard, Transit)
  const statusColor = STATUS_COLOR_MAP[status];
  if (statusColor) {
    return statusColor;
  }

  // Handle "Anwesend" status
  if (status === LOCATION_STATUSES.PRESENT) {
    if (parsed.room) {
      return getColorForPresentWithRoom(
        parsed.room,
        isGroupRoom,
        groupRooms,
        roomColor,
      );
    }
    // "Anwesend" without room details (GDPR-reduced) - show green (present in building)
    return LOCATION_COLORS.GROUP_ROOM;
  }

  return LOCATION_COLORS.UNKNOWN;
}

/**
 * Resolves the display label for a student based on the desired display mode.
 */
export function getLocationDisplay(
  student: StudentLocationContext,
  displayMode: DisplayMode,
  userGroups?: string[],
  supervisedRooms?: string[],
): string {
  const parsed = parseLocation(student.current_location);

  if (displayMode === "groupName") {
    return (student.group_name ?? "").trim() || UNKNOWN_STATUS;
  }

  if (displayMode === "roomName") {
    return parsed.room ?? parsed.status ?? UNKNOWN_STATUS;
  }

  if (displayMode === "contextAware") {
    const canSeeDetails = canSeeDetailedLocation(
      student,
      userGroups,
      supervisedRooms,
    );
    if (canSeeDetails) {
      return parsed.room ?? parsed.status ?? UNKNOWN_STATUS;
    }
    // Without detailed access, show only basic presence status
    // "Unterwegs" is a detail (checked in but not in a room) - map to "Anwesend"
    // This ensures non-supervisors only see Anwesend/Abwesend, not location details
    const status = parsed.status ?? UNKNOWN_STATUS;
    if (status === LOCATION_STATUSES.TRANSIT) {
      return LOCATION_STATUSES.PRESENT;
    }
    return status;
  }

  return UNKNOWN_STATUS;
}

/**
 * Checks whether the viewer can see a student's detailed location information.
 * Access is granted ONLY if:
 * Student is in one of the user's OGS groups (userGroups)
 *
 * Note: Supervisors of rooms do NOT get full location access for students
 * who are not in their OGS groups (GDPR compliance - supervisor role has limited access).
 */
export function canSeeDetailedLocation(
  student: StudentLocationContext,
  userGroups?: string[],
  _supervisedRooms?: string[],
): boolean {
  // Backend grants full read access to admins and verified staff (#2329)
  if (student.has_full_access) {
    return true;
  }

  // Check if student is in user's OGS group
  if (
    student.group_id &&
    Array.isArray(userGroups) &&
    userGroups.length > 0 &&
    userGroups.includes(student.group_id)
  ) {
    return true;
  }

  return false;
}

/**
 * Indicates whether the provided location means the student is present in the building.
 */
export function isPresentLocation(location?: string | null): boolean {
  return parseLocation(location).status === LOCATION_STATUSES.PRESENT;
}

/**
 * Indicates whether the student is explicitly marked as being at home.
 */
export function isHomeLocation(location?: string | null): boolean {
  return parseLocation(location).status === LOCATION_STATUSES.HOME;
}

/**
 * Indicates whether an expected student has not checked in yet today (#3260).
 */
export function isAtSchoolLocation(location?: string | null): boolean {
  return parseLocation(location).status === LOCATION_STATUSES.AT_SCHOOL;
}

/**
 * Indicates whether the student left for class and is not back yet (#5656).
 */
export function isInClassLocation(location?: string | null): boolean {
  return parseLocation(location).status === LOCATION_STATUSES.IN_CLASS;
}

const NOT_CHECKED_IN_STATUSES: ReadonlySet<string> = new Set([
  LOCATION_STATUSES.HOME,
  LOCATION_STATUSES.AT_SCHOOL,
  LOCATION_STATUSES.IN_CLASS,
]);

/**
 * Indicates whether the student has no open check-in: at home, still in class
 * before the first check-in, checked out to class, or without any location. Use
 * this, not isHomeLocation, wherever the question is "check in or check out?".
 */
export function isNotCheckedInLocation(location?: string | null): boolean {
  if (!location?.trim()) return true;
  return NOT_CHECKED_IN_STATUSES.has(parseLocation(location).status);
}

const CHECKED_IN_STATUSES: ReadonlySet<string> = new Set([
  LOCATION_STATUSES.PRESENT,
  LOCATION_STATUSES.SCHOOLYARD,
  LOCATION_STATUSES.TRANSIT,
]);

/**
 * Ob das Kind einen offenen Check-in hat: im Raum („Anwesend“), auf dem
 * Schulhof oder unterwegs. Gegenstück im Backend:
 * `studentpresence.IsCheckedInLocation`.
 */
export function isCheckedInLocation(location?: string | null): boolean {
  return CHECKED_IN_STATUSES.has(parseLocation(location).status);
}

export type CountedDayStatus = "sick" | "class_trip" | "excused";

/**
 * Der Tagesstatus, nach dem Zahlen, Statusfilter und Statusgruppen ein Kind
 * einordnen; krank vor Klassenfahrt vor entschuldigt.
 *
 * Ort vor Status (#6371): Ein eingechecktes Kind zählt in jeder Ansicht als
 * anwesend, genau einmal. Seine Krankmeldung, Entschuldigung oder Klassenfahrt
 * ist dann nur ein Hinweis am Ortsbadge und zählt nirgends mit. Ein Kind ohne
 * Check-in behält seinen Status.
 */
export function countedDayStatus(student: {
  readonly sick?: boolean | null;
  readonly class_trip?: boolean | null;
  readonly excused?: boolean | null;
  readonly current_location?: string | null;
}): CountedDayStatus | null {
  if (isCheckedInLocation(student.current_location)) return null;
  if (student.sick) return "sick";
  if (student.class_trip) return "class_trip";
  if (student.excused) return "excused";
  return null;
}

/**
 * Indicates whether the student is currently on the school yard.
 */
export function isSchoolyardLocation(location?: string | null): boolean {
  return parseLocation(location).status === LOCATION_STATUSES.SCHOOLYARD;
}

/**
 * Indicates whether the student is marked as being in transit (Unterwegs).
 */
export function isTransitLocation(location?: string | null): boolean {
  return parseLocation(location).status === LOCATION_STATUSES.TRANSIT;
}
