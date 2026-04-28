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
  sick?: boolean;
  sick_since?: string | null;
  excused?: boolean;
  excused_since?: string | null;
  has_full_access?: boolean;
  /** Today has an arrival-schedule exception with null expected time — student is not coming */
  not_arrival_today?: boolean;
  not_arrival_reason?: string | null;
  /** Per-room custom color from the backend (resolved against the active group's room) */
  badge_color?: string | null;
}

export const LOCATION_STATUSES = {
  PRESENT: "Anwesend",
  HOME: "Zuhause",
  SCHOOLYARD: "Schulhof",
  TRANSIT: "Unterwegs",
  UNKNOWN: "Unbekannt",
  SICK: "Krank",
  EXCUSED: "Entschuldigt",
  NOT_ARRIVAL: "Kommt heute nicht",
} as const;

export const LOCATION_COLORS = {
  GROUP_ROOM: "#83CD2D", // Green - student in their group's assigned room
  OTHER_ROOM: "#5080D8", // Blue - student in external/other room
  HOME: "#FF3130",
  SCHOOLYARD: "#F78C10",
  TRANSIT: "#D946EF",
  UNKNOWN: "#6B7280",
  SICK: "#EAB308", // Amber - medical/sick status
  EXCUSED: "#7C3AED", // Purple - excused absence (kind is not attending today)
  NOT_ARRIVAL: "#6B7280", // Gray - planned absence via arrival-schedule exception
} as const;

/**
 * Curated palette for per-room color customization. Picks are chosen from
 * hue zones that do NOT appear in the status badges (no Rot/Orange/Amber/
 * Lime-Grün/Medium-Blau/Violett/Magenta — those are reserved for HOME,
 * SCHOOLYARD, SICK, GROUP_ROOM, OTHER_ROOM, EXCUSED, TRANSIT). Within the
 * remaining safe range we use multiple recognizable shades per family
 * (Mint/Türkis/Cyan and Hellblau/Dunkelblau and Pink/Rosa) so neighboring
 * picks differ in lightness AND hue, not just one or the other.
 */
export const ROOM_COLOR_PALETTE: readonly string[] = [
  "#34D399", // Mint        (emerald-400)  ~158° (light, NOT lime)
  "#14B8A6", // Türkis      (teal-500)     ~173°
  "#06B6D4", // Cyan        (cyan-500)     ~189°
  "#38BDF8", // Hellblau    (sky-400)      ~199° (light)
  "#1D4ED8", // Dunkelblau  (blue-700)     ~224° (deep, NOT OTHER_ROOM)
  "#6366F1", // Indigo      (indigo-500)   ~239°
  "#A855F7", // Lila        (purple-500)   ~271°
  "#EC4899", // Pink        (pink-500)     ~333° (vivid)
  "#F43F5E", // Rosa        (rose-500)     ~350° (red-leaning, NOT HOME)
] as const;

/**
 * Picks a color from the palette using a soft-prefer-unique strategy:
 * if any palette colors are unused by existing rooms, pick from those first;
 * otherwise allow duplicates by picking from the full palette.
 */
export function pickRoomColor(usedColors: readonly string[]): string {
  const usedSet = new Set(usedColors.map((c) => c.toLowerCase()));
  const unused = ROOM_COLOR_PALETTE.filter(
    (c) => !usedSet.has(c.toLowerCase()),
  );
  const pool = unused.length > 0 ? unused : ROOM_COLOR_PALETTE;
  return pool[Math.floor(Math.random() * pool.length)] as string;
}

/**
 * Matches `#RGB` or `#RRGGBB` (case-insensitive).
 *
 * Cross-stack parity: this MUST match the regex in
 * `backend/models/facilities/room.go` (`Room.Validate()`), which is the
 * authoritative gate enforced by `CreateRoom` / `UpdateRoom`. The frontend
 * pattern is defensive (catches legacy/bad data before it hits CSS); the
 * backend pattern is the one that prevents persistence.
 */
const HEX_COLOR_PATTERN = /^#([A-Fa-f0-9]{3}|[A-Fa-f0-9]{6})$/;

/** Returns true if `value` is a syntactically valid hex color string. */
export function isValidRoomColor(value: unknown): value is string {
  return typeof value === "string" && HEX_COLOR_PATTERN.test(value.trim());
}

/**
 * Builds the avatar gradient used by room cards/avatars. Falls back to the
 * OTHER_ROOM brand blue when no valid color is supplied so callers don't
 * each invent their own fallback.
 */
export function getRoomGradient(color?: string | null): string {
  const base = isValidRoomColor(color)
    ? color.trim()
    : LOCATION_COLORS.OTHER_ROOM;
  return `linear-gradient(to bottom right, ${base}, color-mix(in srgb, ${base} 85%, black))`;
}

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
 * Stammraum (own group's room) is hard-reserved as GROUP_ROOM green and
 * always wins over `roomColor`. Otherwise, when the backend supplies the
 * room's customized color via `roomColor`, that color is used; without it
 * we fall back to OTHER_ROOM blue.
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

  // Student is in another room — prefer the room's individual color when
  // it's a syntactically valid hex (defense-in-depth against legacy/bad data).
  if (isValidRoomColor(roomColor)) {
    return roomColor.trim();
  }

  // Fallback when the room has no (valid) color set
  return LOCATION_COLORS.OTHER_ROOM; // Blue - in external/supervised room
}

/**
 * Determines the hex color for a given location value and context.
 *
 * Color rules:
 * - GREEN: Student in their OGS group's room OR "Anwesend" without room details
 * - Per-room color: Student in any other room with a custom color set
 * - BLUE: Student in any other room without a custom color
 * - RED/ORANGE/MAGENTA: Status-based (Home, Schoolyard, Transit)
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
