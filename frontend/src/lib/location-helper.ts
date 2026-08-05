/**
 * Centralized helpers for parsing, styling, and rendering student location data.
 */

interface ParsedLocation {
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

export const LOCATION_COLORS = {
  GROUP_ROOM: "#83CD2D", // Green - student in their group's assigned room
  OTHER_ROOM: "#5080D8", // Blue - student in external/other room
  HOME: "#FF3130",
  SCHOOLYARD: "#F78C10",
  TRANSIT: "#D946EF",
  UNKNOWN: "#6B7280",
  SICK: "#EAB308", // Amber - medical/sick status
  EXCUSED: "#7C3AED", // Purple - excused absence (kind is not attending today)
  CLASS_TRIP: "#5080D8", // Blue - known absence for class trips
  NOT_ARRIVAL: "#6B7280", // Gray - planned absence via arrival-schedule exception
} as const;

/**
 * Hover/active/text/background shade variants of the brand green
 * (LOCATION_COLORS.GROUP_ROOM). Centralised so the school check-in toggle and
 * any future "primary action" buttons stop hardcoding hex literals — see
 * CLAUDE.md §0 (route every brand color through LOCATION_COLORS).
 */
export const GROUP_ROOM_SHADES = {
  base: LOCATION_COLORS.GROUP_ROOM, // #83CD2D
  hover: "#74b827", // darker green for primary-button hover
  active: "#669f21", // even darker for :active
  text: "#4a7a15", // accessible text-on-white
  bgHover: "#f0f9e4", // tinted background for ghost-button hover
  bgActive: "#e4f3d3", // slightly darker tint for ghost-button :active
} as const;

/**
 * Text shades of the palette above, dark enough to carry a label on a white or
 * gray-50 surface. The raw brand hexes are fills: on white #83CD2D reaches
 * 2.0:1, #F78C10 2.4:1 and #EAB308 1.9:1, all under the 4.5:1 WCAG minimum for
 * normal text. These keep the hue and darken until the ratio holds at any font
 * size (measured contrast on white in the comments).
 *
 * Keyed by the lowercased base hex, so a component only knows its brand color
 * and asks `getAccessibleTextColor()` for the matching label color. Colors that
 * are not part of the palette (per-room hexes, the Homeoffice blue in
 * staff-helpers) are darkened arithmetically by the same function.
 *
 * These are NOT the StatusBadge label colors: those are darkened further to sit
 * on a tinted pill, which reads muddy on a large figure on white (#8A5600 turns
 * brown). Both tables exist on purpose.
 */
const ACCESSIBLE_TEXT_COLORS: Record<string, string> = {
  "#83cd2d": GROUP_ROOM_SHADES.text, // 5.1:1 — #4a7a15
  "#5080d8": "#3d6ab8", // 5.3:1
  "#ff3130": "#c62826", // 5.6:1
  "#f78c10": "#ad6100", // 4.7:1
  "#d946ef": "#af39c1", // 5.0:1
  "#eab308": "#8b6a05", // 5.1:1
  "#6b7280": "#374151", // 10.3:1 — neutral figures read as gray-700, not gray-500
  // EXCUSED #7C3AED is absent on purpose: at 5.7:1 the raw purple already
  // clears the target and is returned unchanged.
};

/** WCAG AA for normal text; holds for large text too, so one target covers both. */
const TEXT_CONTRAST_TARGET = 4.5;

/** Used when there is no usable color to darken. */
const NEUTRAL_TEXT_COLOR = "#374151";

const accessibleTextCache = new Map<string, string>();

/**
 * Returns a text color for `color` that clears {@link TEXT_CONTRAST_TARGET}
 * against white. Palette colors resolve through {@link ACCESSIBLE_TEXT_COLORS};
 * anything else (a per-room hex, a status color outside the palette) is scaled
 * down channel-wise until the ratio holds, which keeps the hue and only takes
 * brightness away. Colors that already pass are returned unchanged.
 *
 * Use it for the LABEL only. Dots, bars, and other fills keep the raw color —
 * they carry no text and contrast rules do not apply the same way.
 *
 * Accepts a missing color: an indexed palette lookup is `string | undefined`
 * under `noUncheckedIndexedAccess`, and a status the backend adds before the
 * frontend knows it must not blow up a badge. It falls back to neutral gray.
 */
export function getAccessibleTextColor(
  color: string | null | undefined,
): string {
  if (color === null || color === undefined || color.trim() === "") {
    return NEUTRAL_TEXT_COLOR;
  }

  const key = color.trim().toLowerCase();

  const cached = accessibleTextCache.get(key);
  if (cached !== undefined) {
    return cached;
  }

  const resolved = resolveAccessibleTextColor(key, color);
  accessibleTextCache.set(key, resolved);
  return resolved;
}

function resolveAccessibleTextColor(key: string, color: string): string {
  const curated = ACCESSIBLE_TEXT_COLORS[key];
  if (curated) {
    return curated;
  }

  const rgb = hexToRgb(color);
  if (!rgb) {
    // Non-hex input (a CSS keyword, a var()): darkening it is not possible, so
    // fall back to the neutral text color rather than passing it through.
    return NEUTRAL_TEXT_COLOR;
  }

  if (contrastOnWhite(rgb) >= TEXT_CONTRAST_TARGET) {
    return color;
  }

  return rgbToHex(darkenToContrast(rgb, TEXT_CONTRAST_TARGET));
}

/**
 * Largest brightness factor that still clears `target`, found by bisection.
 * The rounding to integer channels happens inside the predicate — rounding
 * afterwards can push a borderline result back below the target.
 */
function darkenToContrast(rgb: RgbColor, target: number): RgbColor {
  let tooDark = 0;
  let tooBright = 1;

  for (let i = 0; i < 24; i++) {
    const factor = (tooDark + tooBright) / 2;
    if (contrastOnWhite(scaleChannels(rgb, factor)) >= target) {
      tooDark = factor;
    } else {
      tooBright = factor;
    }
  }

  return scaleChannels(rgb, tooDark);
}

function scaleChannels(rgb: RgbColor, factor: number): RgbColor {
  return {
    r: Math.round(rgb.r * factor),
    g: Math.round(rgb.g * factor),
    b: Math.round(rgb.b * factor),
  };
}

/** WCAG relative luminance (sRGB). */
function relativeLuminance({ r, g, b }: RgbColor): number {
  const channel = (value: number): number => {
    const srgb = value / 255;
    return srgb <= 0.03928
      ? srgb / 12.92
      : Math.pow((srgb + 0.055) / 1.055, 2.4);
  };

  return 0.2126 * channel(r) + 0.7152 * channel(g) + 0.0722 * channel(b);
}

/** WCAG contrast ratio against #FFFFFF (luminance 1). */
function contrastOnWhite(rgb: RgbColor): number {
  return 1.05 / (relativeLuminance(rgb) + 0.05);
}

function rgbToHex({ r, g, b }: RgbColor): string {
  return `#${[r, g, b].map((v) => v.toString(16).padStart(2, "0")).join("")}`;
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
