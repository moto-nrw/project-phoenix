// Helper functions for staff data transformation and status determination

import { EXTERNAL_CAREGIVER_LABEL, getRoleDisplayName } from "./auth-helpers";
import type { ExternalAccountStatus, Staff } from "./staff-api";
import { LOCATION_COLORS, MOTO_COLOR_PALETTE } from "./location-helper";

// Location status type matching the pattern from OGS groups
interface LocationStatus {
  label: string;
  /** Fläche der Pille — StatusColorBadge leitet die Schriftfarbe daraus ab. */
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
// a raw system role name never reaches the UI in its stored spelling. Other
// system roles („ag-leitung“, ADR 0091) take their shared label.
function formatAccountRole(role: string): string {
  return ROLE_DISPLAY_NAMES[role.toLowerCase()] ?? getRoleDisplayName(role);
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

/**
 * Account mark of an external caregiver (#4573): "Einladung offen" (orange)
 * while the invitation waits, "Kein Konto" (gray) without one, the access
 * „Kein Zugang“ (ADR 0091). An account needs no mark, the person signs in
 * like everyone else.
 */
export interface ExternalAccountMark {
  readonly label: string;
  readonly tone: "orange" | "gray";
}

const EXTERNAL_ACCOUNT_MARKS: Readonly<
  Record<Exclude<ExternalAccountStatus, "account">, ExternalAccountMark>
> = {
  invitation_pending: { label: "Einladung offen", tone: "orange" },
  none: { label: "Kein Konto", tone: "gray" },
};

/** The account mark for a status, or null when the person has an account. */
export function externalAccountMark(
  status: ExternalAccountStatus | null | undefined,
): ExternalAccountMark | null {
  if (status === "invitation_pending" || status === "none") {
    return EXTERNAL_ACCOUNT_MARKS[status];
  }
  return null;
}

/** Label text of the account mark, or null when the person has an account. */
export function externalAccountLabel(
  status: ExternalAccountStatus | null | undefined,
): string | null {
  return externalAccountMark(status)?.label ?? null;
}

/**
 * „Extern · Musikschule“, or „Extern“ without organization: how an external
 * caregiver is marked in lists, badges and select options.
 */
export function externalStaffLabel(organization?: string | null): string {
  const trimmed = organization?.trim();
  return trimmed
    ? `${EXTERNAL_CAREGIVER_LABEL} · ${trimmed}`
    : EXTERNAL_CAREGIVER_LABEL;
}

/**
 * Text label for an external caregiver in places that only take strings
 * (select options): "Extern · Musikschule, Einladung offen". Empty for
 * everyone else.
 */
export function externalStaffNote(
  staff: Pick<
    Staff,
    "isExternal" | "externalOrganization" | "externalAccountStatus"
  >,
): string {
  if (!staff.isExternal) return "";
  const base = externalStaffLabel(staff.externalOrganization);
  const account = externalAccountLabel(staff.externalAccountStatus);
  return account ? `${base}, ${account}` : base;
}

/**
 * Display name of a staff member: the full name from the API, else first and
 * last name. Empty when none of them is set.
 */
export function staffDisplayName(
  name: string | null | undefined,
  firstName: string | null | undefined,
  lastName: string | null | undefined,
): string {
  return name?.trim() || `${firstName ?? ""} ${lastName ?? ""}`.trim();
}
