// Helper functions for staff data transformation and status determination

import type { Staff } from "./staff-api";
import { LOCATION_COLORS, MOTO_COLOR_PALETTE } from "./location-helper";

// Location status type matching the pattern from OGS groups
interface LocationStatus {
  label: string;
  /** Fläche der Pille — StatusDotBadge leitet Punkt und Schriftfarbe daraus ab. */
  customBgColor: string;
}

/**
 * Statusfarben des Personals — dieselbe Sprache wie beim Kind.
 *
 * Vorher war sie genau umgekehrt: „Abwesend" trug das Rot von Krank/Fehler,
 * „Krank" trug Grau. Auf einer Liste mit 24 Personen las sich der normale
 * Feierabend damit wie ein Notfall. Jetzt gilt in beiden Listen:
 * grün = da, grau = nicht da, rot = krank, lila = genehmigt abwesend.
 */
const STAFF_STATUS_COLORS: Record<string, string> = {
  Anwesend: LOCATION_COLORS.GROUP_ROOM,
  Abwesend: LOCATION_COLORS.HOME,
  // Eigener Ton, weil Homeoffice weder Anwesenheit vor Ort noch Abwesenheit
  // ist. Der Hex muss byte-gleich zu MOTO_COLOR_PALETTE.timeTracking.base
  // bleiben, denn LOCATION_BADGE_TONES ist genau darauf geschlüsselt.
  Homeoffice: MOTO_COLOR_PALETTE.timeTracking.base,
};

// Absence types all share the "genehmigt abwesend" purple — dasselbe Lila,
// das beim Kind „Entschuldigt" trägt.
const ABSENCE_LOCATIONS = new Set([
  "Krank",
  "Urlaub",
  "Fortbildung",
  "Freizeitausgleich",
]);
// Every label the badge can show BECAUSE of an absence — the set above
// plus "Abwesend", which is what an absence of type "other" resolves to (and
// also what "not clocked in at all" resolves to, hence the second condition
// below).
const ABSENCE_BADGE_LABELS = new Set([...ABSENCE_LOCATIONS, "Abwesend"]);

function staffAbsenceColor(location: string): string {
  return location === "Krank" ? LOCATION_COLORS.SICK : LOCATION_COLORS.EXCUSED;
}

function buildLocationStatus(
  label: string,
  customBgColor: string,
): LocationStatus {
  return { label, customBgColor };
}

// Get location status for a staff member based on their clock-in status
export function getStaffLocationStatus(staff: Staff): LocationStatus {
  const location = staff.currentLocation ?? "Abwesend";
  // #2403: an absence filed under a school-defined Abwesenheitsart shows that
  // name on the badge. Only the LABEL changes — the color and `currentLocation`
  // itself stay the standard type's, so the "Abwesend" location filter and the
  // absence styling keep working unchanged.
  // Both conditions are needed: the badge must currently be showing an absence
  // (not "Anwesend"/"Homeoffice", which outrank it), and there must be a
  // school-defined wording to show — an empty one means a plain "not clocked
  // in", which keeps reading "Abwesend".
  const label =
    staff.absenceTypeLabel && ABSENCE_BADGE_LABELS.has(location)
      ? staff.absenceTypeLabel
      : location;

  // Check direct matches first (Abwesend, Anwesend, Homeoffice)
  const directMatch = STAFF_STATUS_COLORS[location];
  if (directMatch) {
    return buildLocationStatus(label, directMatch);
  }

  if (ABSENCE_LOCATIONS.has(location)) {
    return buildLocationStatus(label, staffAbsenceColor(location));
  }

  // Any other location (in a room, supervising) means they're present → green
  return buildLocationStatus("Anwesend", LOCATION_COLORS.GROUP_ROOM);
}

// Map auth role names to German display labels
const ROLE_DISPLAY_NAMES: Record<string, string> = {
  admin: "Admin",
  user: "Betreuer",
  guest: "Gast",
  guardian: "Erziehungsberechtigte/r",
  lehrkraft: "Lehrkraft",
};

// Role names are stored lowercase in auth.roles; lowercase before the lookup so
// a raw system role name never reaches the UI in its stored spelling.
function formatAccountRole(role: string): string {
  return ROLE_DISPLAY_NAMES[role.toLowerCase()] ?? role;
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

  return trimmed.substring(0, maxLength - 1) + "…";
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

/**
 * Employment types as stored by the backend (users.staff.employment_type).
 * Shared by the staff detail header and the Zeitkonten filter on /staff so
 * both spell the German labels the same way.
 */
export const employmentTypeLabels: Record<string, string> = {
  full_time: "Vollzeit",
  part_time: "Teilzeit",
  minijob: "Minijob",
};
