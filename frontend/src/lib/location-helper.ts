/**
 * Centralized helpers for parsing, styling, and rendering student location data.
 */

export interface ParsedLocation {
  status: string;
  room?: string;
}

export type DisplayMode = 'groupName' | 'roomName' | 'contextAware';

export interface LocationStyle {
  color: string;
  glowEffect: string;
  label: string;
}

export interface StudentLocationContext {
  current_location?: string | null;
  group_id?: string | null;
  group_name?: string | null;
}

export const LOCATION_COLORS = {
  GROUP_ROOM: '#83CD2D',
  OTHER_ROOM: '#5080D8',
  HOME: '#FF3130',
  SCHOOLYARD: '#F78C10',
  TRANSIT: '#D946EF',
  UNKNOWN: '#6B7280',
} as const;

const LOCATION_SEPARATOR = '-';
const UNKNOWN_STATUS = 'Unbekannt';
const STATUS_PRESENT = 'Anwesend';
const STATUS_HOME = 'Zuhause';
const STATUS_SCHOOLYARD = 'Schulhof';
const STATUS_TRANSIT = 'Unterwegs';

/**
 * Splits a location string into its status and optional room component.
 */
export function parseLocation(location?: string | null): ParsedLocation {
  const raw = (location ?? '').trim();
  if (raw.length === 0) {
    return { status: UNKNOWN_STATUS };
  }

  const separatorIndex = raw.indexOf(LOCATION_SEPARATOR);
  if (separatorIndex === -1) {
    return { status: raw };
  }

  const status = raw.slice(0, separatorIndex).trim() || UNKNOWN_STATUS;
  const room = raw.slice(separatorIndex + 1).trim();

  return room.length > 0 ? { status, room } : { status };
}

/**
 * Determines the hex color for a given location value and context.
 */
export function getLocationColor(
  location?: string | null,
  isGroupRoom?: boolean,
): string {
  const parsed = parseLocation(location);
  const status = parsed.status;

  if (status === STATUS_HOME) {
    return LOCATION_COLORS.HOME;
  }
  if (status === STATUS_SCHOOLYARD) {
    return LOCATION_COLORS.SCHOOLYARD;
  }
  if (status === STATUS_TRANSIT) {
    return LOCATION_COLORS.TRANSIT;
  }

  if (status === STATUS_PRESENT) {
    if (parsed.room) {
      return isGroupRoom === true
        ? LOCATION_COLORS.GROUP_ROOM
        : LOCATION_COLORS.OTHER_ROOM;
    }
    return LOCATION_COLORS.OTHER_ROOM;
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
): string {
  const parsed = parseLocation(student.current_location);

  if (displayMode === 'groupName') {
    return (student.group_name ?? '').trim() || UNKNOWN_STATUS;
  }

  if (displayMode === 'roomName') {
    return parsed.room ?? parsed.status ?? UNKNOWN_STATUS;
  }

  if (displayMode === 'contextAware') {
    const canSeeDetails = canSeeDetailedLocation(student, userGroups);
    if (canSeeDetails) {
      return parsed.room ?? parsed.status ?? UNKNOWN_STATUS;
    }
    return parsed.status ?? UNKNOWN_STATUS;
  }

  return UNKNOWN_STATUS;
}

/**
 * Checks whether the viewer can see a student's detailed location information.
 */
export function canSeeDetailedLocation(
  student: StudentLocationContext,
  userGroups?: string[],
): boolean {
  if (!student.group_id || !Array.isArray(userGroups) || userGroups.length === 0) {
    return false;
  }

  return userGroups.includes(student.group_id);
}

/**
 * Generates a soft glow box-shadow string for the provided hex color.
 */
export function getLocationGlowEffect(color: string): string {
  const rgb = hexToRgb(color) ?? hexToRgb(LOCATION_COLORS.UNKNOWN);
  return `0 8px 25px rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, 0.4)`;
}

interface RgbColor {
  r: number;
  g: number;
  b: number;
}

function hexToRgb(hexColor: string): RgbColor | null {
  const sanitized = hexColor.trim().replace('#', '');
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
