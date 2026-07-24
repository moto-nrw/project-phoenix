// Helper functions for staff data transformation and status determination

import type { Staff } from "./staff-api";

// Location status type matching the pattern from OGS groups
interface LocationStatus {
  label: string;
  badgeColor: string;
  cardGradient: string;
  customBgColor: string;
  customShadow: string;
}

// Shared badge color used by all location statuses
const BADGE_COLOR = "text-white backdrop-blur-sm";

// Location color config: [cardGradient, customBgColor, rgbaValues]
type LocationColorConfig = [string, string, string];

const LOCATION_COLORS: Record<string, LocationColorConfig> = {
  Abwesend: ["from-red-50/80 to-rose-100/80", "#FF3130", "255, 49, 48"],
  Anwesend: ["from-green-50/80 to-emerald-100/80", "#83CD2D", "131, 205, 45"],
  Homeoffice: ["from-sky-50/80 to-sky-100/80", "#0EA5E9", "14, 165, 233"],
};

// Absence types all share the same gray styling
const ABSENCE_LOCATIONS = new Set([
  "Krank",
  "Urlaub",
  "Fortbildung",
  "Freizeitausgleich",
]);
const ABSENCE_COLOR: LocationColorConfig = [
  "from-gray-50/80 to-slate-100/80",
  "#6B7280",
  "107, 114, 128",
];

function buildLocationStatus(
  label: string,
  [cardGradient, customBgColor, rgba]: LocationColorConfig,
): LocationStatus {
  return {
    label,
    badgeColor: BADGE_COLOR,
    cardGradient,
    customBgColor,
    customShadow: `0 8px 25px rgba(${rgba}, 0.4)`,
  };
}

// Get location status for a staff member based on their clock-in status
export function getStaffLocationStatus(staff: Staff): LocationStatus {
  const location = staff.currentLocation ?? "Abwesend";

  // Check direct matches first (Abwesend, Anwesend, Homeoffice)
  const directMatch = LOCATION_COLORS[location];
  if (directMatch) {
    return buildLocationStatus(location, directMatch);
  }

  // Absence types get gray styling.
  if (ABSENCE_LOCATIONS.has(location)) {
    return buildLocationStatus(location, ABSENCE_COLOR);
  }

  // Any other location (in a room, supervising) means they're present → green
  return buildLocationStatus("Anwesend", LOCATION_COLORS.Anwesend!);
}

// Map auth role names to German display labels
const ROLE_DISPLAY_NAMES: Record<string, string> = {
  admin: "Admin",
  user: "Betreuer",
  guest: "Gast",
  guardian: "Erziehungsberechtigte/r",
};

function formatAccountRole(role: string): string {
  return ROLE_DISPLAY_NAMES[role] ?? role;
}

// Get a display-friendly role/type for staff
export function getStaffDisplayType(staff: Staff): string {
  // Check custom position first (set on teacher profile)
  if (staff.role) {
    return staff.role;
  }
  // Fall back to specialization if available
  if (staff.isTeacher && staff.specialization) {
    return staff.specialization;
  }
  // Fall back to auth role with display name
  if (staff.accountRole) {
    return formatAccountRole(staff.accountRole);
  }
  return "";
}

// Get additional info to display on card
export function getStaffCardInfo(staff: Staff): string[] {
  const info: string[] = [];

  // Add qualifications if available
  if (staff.qualifications) {
    info.push(staff.qualifications);
  }

  // Add supervision role if currently supervising
  if (staff.isSupervising && staff.supervisionRole) {
    if (staff.supervisionRole === "primary") {
      info.push("Hauptbetreuer");
    } else if (staff.supervisionRole === "assistant") {
      info.push("Assistenz");
    }
  }

  return info;
}

// Format staff notes for display (truncate if needed)
export function formatStaffNotes(
  notes?: string,
  maxLength = 100,
): string | undefined {
  if (!notes || notes.trim().length === 0) {
    return undefined;
  }

  const trimmed = notes.trim();
  if (trimmed.length <= maxLength) {
    return trimmed;
  }

  return trimmed.substring(0, maxLength - 3) + "...";
}

// Sort staff by supervision status and name
export function sortStaff(staff: Staff[]): Staff[] {
  return [...staff].sort((a, b) => {
    // First sort by supervision status (supervising staff first)
    if (a.isSupervising && !b.isSupervising) return -1;
    if (!a.isSupervising && b.isSupervising) return 1;

    // Then sort alphabetically by last name
    return a.lastName.localeCompare(b.lastName, "de");
  });
}
