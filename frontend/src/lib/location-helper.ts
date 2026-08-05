/**
 * Centralized helpers for parsing, styling, and rendering student location data.
 */

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
  SCHOOLYARD: "Schulhof",
  TRANSIT: "Unterwegs",
  UNKNOWN: "Unbekannt",
  SICK: "Krank",
  EXCUSED: "Entschuldigt",
  CLASS_TRIP: "Klassenfahrt",
  NOT_ARRIVAL: "Kommt heute nicht",
} as const;

/** Shared brand palette. Red follows Phoenix notifications and hours accounts. */
export const MOTO_COLOR_PALETTE = {
  green: {
    soft: "#EEF9E1",
    muted: "#D7E8C3",
    light: "#92D63C",
    base: "#83CD2D",
    vivid: "#5F9F1B",
    hover: "#74B825",
    active: "#6DB118",
    strong: "#3F6F12",
  },
  blue: {
    soft: "#EDF3FC",
    light: "#6B95E0",
    base: "#5080D8",
    hover: "#3B68C0",
    strong: "#315C9B",
  },
  orange: {
    soft: "#FFF3E5",
    base: "#F78C10",
    hover: "#E07400",
    strong: "#9B5609",
  },
  red: {
    soft: "#FEF2F2",
    base: "#DC2626",
    hover: "#B91C1C",
    strong: "#B91C1C",
  },
  teal: {
    soft: "#E8F8F5",
    light: "#5CC8BA",
    base: "#159E90",
    strong: "#0F766E",
  },
  amber: {
    soft: "#FEF3C7",
    light: "#FACC15",
    base: "#EAB308",
    strong: "#92400E",
  },
  purple: {
    soft: "#F3E8FF",
    light: "#A78BFA",
    base: "#7C3AED",
    strong: "#6B21A8",
  },
  magenta: {
    soft: "#FAE8FF",
    light: "#E879F9",
    base: "#D946EF",
    strong: "#86198F",
  },
  indigo: {
    soft: "#EEF2FF",
    light: "#818CF8",
    base: "#4F46E5",
    strong: "#3730A3",
  },
  coral: {
    soft: "#FFF0ED",
    light: "#F29A8D",
    base: "#E85D4A",
    strong: "#A83A2E",
  },
  cyan: {
    soft: "#ECFEFF",
    light: "#67E8F9",
    base: "#0891B2",
    strong: "#155E75",
  },
  navy: {
    soft: "#EEF4F8",
    light: "#7FA6C9",
    base: "#365D83",
    strong: "#1E3A5F",
  },
  mint: {
    soft: "#EAF9F3",
    light: "#8AD9BB",
    base: "#3BAF83",
    strong: "#187255",
  },
  wine: {
    soft: "#FBECEF",
    light: "#CF7180",
    base: "#8F2535",
    strong: "#681A27",
  },
  gold: {
    soft: "#FFF7E6",
    light: "#E6B85C",
    base: "#B7791F",
    strong: "#7A4A0B",
  },
  petrol: {
    soft: "#E9F7F6",
    light: "#72BDB8",
    base: "#217A78",
    strong: "#155A59",
  },
  neutral: {
    soft: "#F3F4F6",
    light: "#9CA3AF",
    base: "#6B7280",
    strong: "#374151",
  },
  stone: {
    soft: "#F5F5F4",
    light: "#A8A29E",
    base: "#78716C",
    strong: "#44403C",
  },
} as const;

export interface LocationBadgeTone {
  backgroundColor: string;
  dotColor: string;
  textColor: string;
}

const LOCATION_BADGE_TONES: Record<string, LocationBadgeTone> = {
  [MOTO_COLOR_PALETTE.green.base]: {
    backgroundColor: MOTO_COLOR_PALETTE.green.soft,
    dotColor: MOTO_COLOR_PALETTE.green.base,
    textColor: MOTO_COLOR_PALETTE.green.strong,
  },
  [MOTO_COLOR_PALETTE.blue.base]: {
    backgroundColor: MOTO_COLOR_PALETTE.blue.soft,
    dotColor: MOTO_COLOR_PALETTE.blue.base,
    textColor: MOTO_COLOR_PALETTE.blue.strong,
  },
  [MOTO_COLOR_PALETTE.orange.base]: {
    backgroundColor: MOTO_COLOR_PALETTE.orange.soft,
    dotColor: MOTO_COLOR_PALETTE.orange.base,
    textColor: MOTO_COLOR_PALETTE.orange.strong,
  },
  [MOTO_COLOR_PALETTE.red.base]: {
    backgroundColor: MOTO_COLOR_PALETTE.red.soft,
    dotColor: MOTO_COLOR_PALETTE.red.base,
    textColor: MOTO_COLOR_PALETTE.red.strong,
  },
  [MOTO_COLOR_PALETTE.navy.base]: {
    backgroundColor: MOTO_COLOR_PALETTE.navy.soft,
    dotColor: MOTO_COLOR_PALETTE.navy.base,
    textColor: MOTO_COLOR_PALETTE.navy.strong,
  },
  [MOTO_COLOR_PALETTE.cyan.base]: {
    backgroundColor: MOTO_COLOR_PALETTE.cyan.soft,
    dotColor: MOTO_COLOR_PALETTE.cyan.base,
    textColor: MOTO_COLOR_PALETTE.cyan.strong,
  },
  [MOTO_COLOR_PALETTE.stone.base]: {
    backgroundColor: MOTO_COLOR_PALETTE.stone.soft,
    dotColor: MOTO_COLOR_PALETTE.stone.base,
    textColor: MOTO_COLOR_PALETTE.stone.strong,
  },
  "#EAB308": {
    backgroundColor: MOTO_COLOR_PALETTE.amber.soft,
    dotColor: MOTO_COLOR_PALETTE.amber.base,
    textColor: MOTO_COLOR_PALETTE.amber.strong,
  },
  "#7C3AED": {
    backgroundColor: MOTO_COLOR_PALETTE.purple.soft,
    dotColor: MOTO_COLOR_PALETTE.purple.base,
    textColor: MOTO_COLOR_PALETTE.purple.strong,
  },
  "#D946EF": {
    backgroundColor: MOTO_COLOR_PALETTE.magenta.soft,
    dotColor: MOTO_COLOR_PALETTE.magenta.base,
    textColor: MOTO_COLOR_PALETTE.magenta.strong,
  },
  "#6B7280": {
    backgroundColor: "#F3F4F6",
    dotColor: "#9CA3AF",
    textColor: "#4B5563",
  },
};

export function getLocationBadgeTone(color: string): LocationBadgeTone {
  return (
    LOCATION_BADGE_TONES[color.toUpperCase()] ?? {
      backgroundColor: "#F9FAFB",
      dotColor: color,
      textColor: "#374151",
    }
  );
}

export const LOCATION_COLORS = {
  GROUP_ROOM: MOTO_COLOR_PALETTE.green.base,
  OTHER_ROOM: MOTO_COLOR_PALETTE.blue.base,
  HOME: MOTO_COLOR_PALETTE.neutral.base,
  SCHOOLYARD: MOTO_COLOR_PALETTE.amber.base,
  TRANSIT: MOTO_COLOR_PALETTE.magenta.base,
  UNKNOWN: MOTO_COLOR_PALETTE.stone.base,
  SICK: MOTO_COLOR_PALETTE.red.base,
  EXCUSED: MOTO_COLOR_PALETTE.purple.base, // Purple - excused absence (kind is not attending today)
  CLASS_TRIP: MOTO_COLOR_PALETTE.cyan.base,
  NOT_ARRIVAL: MOTO_COLOR_PALETTE.navy.base,
  DANGER: MOTO_COLOR_PALETTE.red.base,
} as const;

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
  text: MOTO_COLOR_PALETTE.green.strong,
  bgHover: MOTO_COLOR_PALETTE.green.soft,
  bgActive: "#D7E8C3",
} as const;

const LOCATION_SEPARATOR = "-";
const UNKNOWN_STATUS = LOCATION_STATUSES.UNKNOWN;

const LEGACY_STATUS_MAP: Record<string, string> = {
  abwesend: LOCATION_STATUSES.HOME,
  zuhause: LOCATION_STATUSES.HOME,
  home: LOCATION_STATUSES.HOME,
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
 *   3. OTHER_ROOM blue fallback — bestehende Räume ohne gesetzte Farbe sehen
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
    return roomColor;
  }

  // Student in any other room
  return LOCATION_COLORS.OTHER_ROOM; // Blue - in external/supervised room
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
  // Backend grants full read access via student_data_scope setting
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
 * Generates a soft glow box-shadow string for the provided hex color.
 */
export function getLocationGlowEffect(color: string): string {
  const rgb = hexToRgb(color) ?? hexToRgb(LOCATION_COLORS.UNKNOWN);
  if (!rgb) {
    return "0 8px 25px rgba(107, 114, 128, 0.4)";
  }
  return `0 8px 25px rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, 0.4)`;
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

/**
 * Soft gradient utility classes for a student presence card's background,
 * one per LOCATION_COLORS status family. Derived from the moto-* design
 * tokens (not generic Tailwind palette colors) so a card's accent always
 * matches the color family used by the matching location badge. Shared by
 * the OGS-groups and active-supervisions student grids so this mapping only
 * lives in one place.
 */
export const PRESENCE_CARD_GRADIENTS = {
  groupRoom: "from-moto-green-soft to-moto-green-light/70",
  otherRoom: "from-moto-blue-soft to-moto-blue-light/70",
  schoolyard: "from-moto-amber-soft to-moto-amber-light/70",
  transit: "from-moto-magenta-soft to-moto-magenta-light/70",
  // No moto-neutral token exists; HOME/unknown fall back to plain gray so
  // the card reads as neutral instead of borrowing another status's color.
  home: "from-gray-50/80 to-gray-100/80",
  unknown: "from-gray-50/80 to-gray-100/80",
} as const;

/**
 * Resolves the presence-card gradient for a student location string.
 * Mirrors getLocationColor's priority order: own group room, then
 * schoolyard/transit/home status, then present-with-or-without-room,
 * falling back to neutral for anything unrecognised.
 */
export function getPresenceCardGradient(
  location: string | null | undefined,
  isGroupRoom: boolean,
): string {
  if (isGroupRoom) {
    return PRESENCE_CARD_GRADIENTS.groupRoom;
  }
  if (isSchoolyardLocation(location)) {
    return PRESENCE_CARD_GRADIENTS.schoolyard;
  }
  if (isTransitLocation(location)) {
    return PRESENCE_CARD_GRADIENTS.transit;
  }
  if (isHomeLocation(location)) {
    return PRESENCE_CARD_GRADIENTS.home;
  }

  const parsed = parseLocation(location);
  if (parsed.room || parsed.status === LOCATION_STATUSES.PRESENT) {
    return PRESENCE_CARD_GRADIENTS.otherRoom;
  }
  return PRESENCE_CARD_GRADIENTS.unknown;
}

interface RgbColor {
  r: number;
  g: number;
  b: number;
}

function hexToRgb(hexColor: string): RgbColor | null {
  const sanitized = hexColor.trim().replace("#", "");
  if (sanitized.length !== 6) {
    return null;
  }

  const r = Number.parseInt(sanitized.slice(0, 2), 16);
  const g = Number.parseInt(sanitized.slice(2, 4), 16);
  const b = Number.parseInt(sanitized.slice(4, 6), 16);

  if (Number.isNaN(r) || Number.isNaN(g) || Number.isNaN(b)) {
    return null;
  }

  return { r, g, b };
}
