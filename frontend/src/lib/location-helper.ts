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
export type MotoDuotoneTone =
  | "blue"
  | "timeTracking"
  | "teal"
  | "magenta"
  | "orange"
  | "amber"
  | "purple"
  | "greenDeep"
  | "greenVivid"
  | "indigo"
  | "red"
  | "coral"
  | "cyan"
  | "navy"
  | "mint"
  | "wine"
  | "gold"
  | "petrol"
  | "neutral"
  | "stone";

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
  timeTracking: {
    soft: "#E0F2FE",
    light: "#7DD3FC",
    base: "#0EA5E9",
    strong: "#0369A1",
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

/**
 * Accepts a missing color on purpose: an indexed palette lookup is
 * `string | undefined` under `noUncheckedIndexedAccess`, and a status the
 * backend adds before the frontend knows it must not blow up a badge. Falls
 * back to the neutral tone rather than throwing on `.toUpperCase()`.
 */
export function getLocationBadgeTone(
  color: string | null | undefined,
): LocationBadgeTone {
  if (color === null || color === undefined || color.trim() === "") {
    return {
      backgroundColor: "#F9FAFB",
      dotColor: "#9CA3AF",
      textColor: NEUTRAL_TEXT_COLOR,
    };
  }

  return (
    LOCATION_BADGE_TONES[color.toUpperCase()] ?? {
      backgroundColor: "#F9FAFB",
      dotColor: color,
      textColor: NEUTRAL_TEXT_COLOR,
    }
  );
}

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

/**
 * Text shades of the palette above, dark enough to carry a label. The raw brand
 * hexes are fills: on white #83CD2D reaches 2.0:1, #F78C10 2.4:1 and #EAB308
 * 1.9:1, all far under the WCAG minimum for normal text. These keep the hue and
 * only take brightness away.
 *
 * Calibrated against Tailwind gray-50 (#F9FAFB), the darker of the two surfaces
 * they run on (the badge pill; cards are white), so every entry holds on both.
 * The comments carry the ratio on gray-50; on white each value is a little
 * better still.
 *
 * Keyed by the lowercased base hex, so a component only knows its brand color
 * and asks `getAccessibleTextColor()` for the matching label color. Colors that
 * are not part of the palette (per-room hexes, the Homeoffice blue in
 * staff-helpers) are darkened arithmetically by the same function, against the
 * surface they are actually drawn on.
 *
 * These are NOT the StatusBadge label colors: those are darkened further to sit
 * on a tinted pill, which reads muddy on a large figure on white (#8A5600 turns
 * brown). Both tables exist on purpose.
 */
const ACCESSIBLE_TEXT_COLORS: Record<string, string> = {
  "#83cd2d": GROUP_ROOM_SHADES.text, // 4.9:1 — #4a7a15
  "#5080d8": "#3d6ab8", // 5.1:1
  "#ff3130": "#c62826", // 5.4:1
  "#f78c10": "#a35c0b", // 4.9:1
  "#d946ef": "#af39c1", // 4.8:1
  "#eab308": "#8b6a05", // 4.8:1
  "#6b7280": "#374151", // 9.9:1 — neutral figures read as gray-700, not gray-500
  // EXCUSED #7C3AED is absent on purpose: at 5.5:1 the raw purple already
  // clears the target and is returned unchanged.
};

const SURFACE_WHITE = "#FFFFFF";

/**
 * WCAG AA for normal text is 4.5:1; the target sits above it so a shade never
 * lands on the boundary, where a rounded channel decides pass or fail. Normal
 * text is the strict case, so one target covers large text too.
 */
const TEXT_CONTRAST_TARGET = 4.75;

/** Used when there is no usable color to darken. */
const NEUTRAL_TEXT_COLOR = "#374151";

const accessibleTextCache = new Map<string, string>();

/**
 * Returns a text color for `color` that clears {@link TEXT_CONTRAST_TARGET}
 * against `background` — pass the surface the text actually sits on, since the
 * same shade passes on white and fails on gray-50. Palette colors start from
 * {@link ACCESSIBLE_TEXT_COLORS}, anything else (a per-room hex, a status color
 * outside the palette) from the color itself; either is scaled down
 * channel-wise until the ratio holds, which keeps the hue. Colors that already
 * pass are returned unchanged.
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
  background: string = SURFACE_WHITE,
): string {
  if (color === null || color === undefined || color.trim() === "") {
    return NEUTRAL_TEXT_COLOR;
  }

  const key = `${color.trim().toLowerCase()}|${background.trim().toLowerCase()}`;

  const cached = accessibleTextCache.get(key);
  if (cached !== undefined) {
    return cached;
  }

  const resolved = resolveAccessibleTextColor(color, background);
  accessibleTextCache.set(key, resolved);
  return resolved;
}

function resolveAccessibleTextColor(color: string, background: string): string {
  const candidate = ACCESSIBLE_TEXT_COLORS[color.trim().toLowerCase()] ?? color;

  const rgb = hexToRgb(candidate);
  const backgroundRgb = hexToRgb(background);
  if (!rgb || !backgroundRgb) {
    // Non-hex input (a CSS keyword, a var()): darkening it is not possible, so
    // fall back to the neutral text color rather than passing it through.
    return NEUTRAL_TEXT_COLOR;
  }

  if (contrastRatio(rgb, backgroundRgb) >= TEXT_CONTRAST_TARGET) {
    return candidate;
  }

  return rgbToHex(darkenToContrast(rgb, backgroundRgb, TEXT_CONTRAST_TARGET));
}

/**
 * Largest brightness factor that still clears `target`, found by bisection.
 * The rounding to integer channels happens inside the predicate — rounding
 * afterwards can push a borderline result back below the target.
 */
function darkenToContrast(
  rgb: RgbColor,
  background: RgbColor,
  target: number,
): RgbColor {
  let tooDark = 0;
  let tooBright = 1;

  for (let i = 0; i < 24; i++) {
    const factor = (tooDark + tooBright) / 2;
    if (contrastRatio(scaleChannels(rgb, factor), background) >= target) {
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

/** WCAG contrast ratio between two colors. */
function contrastRatio(a: RgbColor, b: RgbColor): number {
  const lumA = relativeLuminance(a);
  const lumB = relativeLuminance(b);
  return (Math.max(lumA, lumB) + 0.05) / (Math.min(lumA, lumB) + 0.05);
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
