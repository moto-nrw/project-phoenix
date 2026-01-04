/**
 * Centralized helpers for parsing, styling, and rendering student location data.
 */

export interface ParsedLocation {
  status: string;
  room?: string;
}

export type DisplayMode = "groupName" | "roomName" | "contextAware";

export interface LocationStyle {
  color: string;
  glowEffect: string;
  label: string;
}

export interface StudentLocationContext {
  current_location?: string | null;
  location_since?: string | null;
  group_id?: string | null;
  group_name?: string | null;
}

export const LOCATION_STATUSES = {
  PRESENT: "Anwesend",
  HOME: "Zuhause",
  SCHOOLYARD: "Schulhof",
  TRANSIT: "Unterwegs",
  UNKNOWN: "Unbekannt",
} as const;

export const LOCATION_COLORS = {
  GROUP_ROOM: "#83CD2D", // Green - student in their group's assigned room
  OTHER_ROOM: "#5080D8", // Blue - student in external/other room
  HOME: "#FF3130",
  SCHOOLYARD: "#F78C10",
  TRANSIT: "#D946EF",
  UNKNOWN: "#6B7280",
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
    return `${LOCATION_STATUSES.PRESENT} - WC`;
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

  return `${normalizedStatus} - ${room}`;
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
 */
function getColorForPresentWithRoom(
  room: string,
  isGroupRoom?: boolean,
  groupRooms?: string[],
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

  // Student in any other room
  return LOCATION_COLORS.OTHER_ROOM; // Blue - in external/supervised room
}

/**
 * Determines the hex color for a given location value and context.
 *
 * Color rules:
 * - GREEN: Student in their OGS group's room OR "Anwesend" without room details
 * - BLUE: Student in any other room (external room or supervised room)
 * - RED/ORANGE/MAGENTA: Status-based (Home, Schoolyard, Transit)
 */
export function getLocationColor(
  location?: string | null,
  isGroupRoom?: boolean,
  groupRooms?: string[],
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
      return getColorForPresentWithRoom(parsed.room, isGroupRoom, groupRooms);
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
  // Check if student is in user's OGS group (ONLY way to get detailed location)
  if (
    student.group_id &&
    Array.isArray(userGroups) &&
    userGroups.length > 0 &&
    userGroups.includes(student.group_id)
  ) {
    return true;
  }

  // Supervisors do NOT get detailed location for students outside their OGS groups
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
