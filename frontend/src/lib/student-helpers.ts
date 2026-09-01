// lib/student-helpers.ts
// Type definitions and helper functions for students

import type {
  CompanionExtensionConfirmation,
  StudentCompanion,
} from "./student-companion-api";
import type { ConsentKey, ConsentRecord } from "./consent-types";
import {
  LOCATION_STATUSES,
  parseLocation,
  isHomeLocation,
  isPresentLocation,
  isSchoolyardLocation,
  isTransitLocation,
  normalizeLocation,
} from "./location-helper";

/**
 * Shared filter options for school year (Klassenstufe) filter
 * Used in student search and OGS groups pages
 */
export const SCHOOL_YEAR_FILTER_OPTIONS = [
  { value: "all", label: "Alle" },
  { value: "1", label: "1" },
  { value: "2", label: "2" },
  { value: "3", label: "3" },
  { value: "4", label: "4" },
] as const;

export type BusDayKey = "mon" | "tue" | "wed" | "thu" | "fri";
export type BusDays = Partial<Record<BusDayKey, boolean>>;

export const BUS_WEEKDAYS: ReadonlyArray<{ key: BusDayKey; label: string }> = [
  { key: "mon", label: "Montag" },
  { key: "tue", label: "Dienstag" },
  { key: "wed", label: "Mittwoch" },
  { key: "thu", label: "Donnerstag" },
  { key: "fri", label: "Freitag" },
] as const;

export function normalizeBusDays(value?: BusDays | null): BusDays {
  const out: BusDays = {};
  for (const day of BUS_WEEKDAYS) {
    if (value?.[day.key]) out[day.key] = true;
  }
  return out;
}

export function busDaysHaveAny(value?: BusDays | null): boolean {
  return BUS_WEEKDAYS.some((day) => Boolean(value?.[day.key]));
}

/**
 * Resolves bus_days from a simple Buskind Ja/Nein toggle without losing
 * per-day granularity. Mirrors the backend legacy-bus alias (#1582): turning
 * the toggle off clears all days; turning it on keeps the existing per-day
 * selection when present, otherwise defaults to all weekdays (Mo–Fr). Used by
 * the simplified personal-info form, where only a boolean is captured.
 */
export function busDaysFromToggle(
  enabled: boolean,
  existing?: BusDays | null,
): BusDays {
  if (!enabled) return {};
  const normalized = normalizeBusDays(existing);
  if (busDaysHaveAny(normalized)) return normalized;
  return BUS_WEEKDAYS.reduce<BusDays>((acc, day) => {
    acc[day.key] = true;
    return acc;
  }, {});
}

export function formatBusDays(value?: BusDays | null): string {
  const labels = BUS_WEEKDAYS.filter((day) => Boolean(value?.[day.key])).map(
    (day) => day.label.slice(0, 2),
  );
  return labels.length > 0 ? labels.join(", ") : "Keine Bus-Tage";
}

// Pickup days mirror bus days: per-weekday "wird abgeholt" flags. A day not
// selected means the child goes home alone that day. Reuses the same weekday
// keys as BusDays.
export type PickupDayKey = BusDayKey;
export type PickupDays = Partial<Record<PickupDayKey, boolean>>;

export const PICKUP_WEEKDAYS: ReadonlyArray<{
  key: PickupDayKey;
  label: string;
}> = [
  { key: "mon", label: "Montag" },
  { key: "tue", label: "Dienstag" },
  { key: "wed", label: "Mittwoch" },
  { key: "thu", label: "Donnerstag" },
  { key: "fri", label: "Freitag" },
] as const;

export function normalizePickupDays(value?: PickupDays | null): PickupDays {
  const out: PickupDays = {};
  for (const day of PICKUP_WEEKDAYS) {
    if (value?.[day.key]) out[day.key] = true;
  }
  return out;
}

export function pickupDaysHaveAny(value?: PickupDays | null): boolean {
  return PICKUP_WEEKDAYS.some((day) => Boolean(value?.[day.key]));
}

export function formatPickupDays(value?: PickupDays | null): string {
  const labels = PICKUP_WEEKDAYS.filter((day) => Boolean(value?.[day.key])).map(
    (day) => day.label.slice(0, 2),
  );
  return labels.length > 0 ? labels.join(", ") : "Keine Abhol-Tage";
}

// Departure days (#1610) unify Buskind + Abholregelung into one mutually
// exclusive choice per weekday: the child goes home alone ("alleine"), rides
// the bus ("bus"), or is collected by a person ("pickup"). departure_days is
// the single source of truth on the backend; bus_days/pickup_days are derived.
export type DepartureMode = "alone" | "bus" | "pickup" | "accompanied";
export type DepartureDayKey = BusDayKey;
export type DepartureDays = Partial<Record<DepartureDayKey, DepartureMode>>;
export type AllowedDepartureModes = Partial<
  Record<DepartureDayKey, DepartureMode[]>
>;

export const DEPARTURE_WEEKDAYS: ReadonlyArray<{
  key: DepartureDayKey;
  label: string;
}> = [
  { key: "mon", label: "Montag" },
  { key: "tue", label: "Dienstag" },
  { key: "wed", label: "Mittwoch" },
  { key: "thu", label: "Donnerstag" },
  { key: "fri", label: "Freitag" },
] as const;

/** German labels for the departure modes (shown in forms and badges). */
export const DEPARTURE_MODE_LABELS: Record<DepartureMode, string> = {
  alone: "Geht zu Fuß",
  bus: "Fährt Bus",
  pickup: "Wird abgeholt",
  accompanied: "Mit anderem Kind",
};

/** Short labels for the departure modes, used in compact badges (editor pills
 *  and the read-only Stammdaten matrix). */
export const DEPARTURE_MODE_SHORT_LABELS: Record<DepartureMode, string> = {
  alone: "Zu Fuß",
  bus: "Bus",
  pickup: "Abgeholt",
  accompanied: "Anderes Kind",
};

/**
 * Quiet read-only pill for displaying a selected departure arrangement in the
 * Stammdaten view, so the matrix sits calmly next to the plain text fields
 * around it instead of shouting in color (the per-mode colors, incl. the
 * accompanied magenta from #1694, were dropped — the mode is conveyed by its
 * label, not a color). The editor keeps its checkboxes; only the active fill
 * was neutralized there.
 */
export const CARE_DISPLAY_PILL =
  "inline-flex items-center rounded-full bg-gray-100 px-2.5 py-0.5 text-xs font-medium text-gray-700";

/** Canonical render order for departure modes (stable badge ordering). */
const DEPARTURE_MODE_ORDER: readonly DepartureMode[] = [
  "alone",
  "bus",
  "pickup",
  "accompanied",
];

export function normalizeDepartureDays(
  value?: DepartureDays | null,
): DepartureDays {
  const out: DepartureDays = {};
  for (const day of DEPARTURE_WEEKDAYS) {
    const mode = value?.[day.key];
    if (mode === "bus" || mode === "pickup" || mode === "accompanied")
      out[day.key] = mode;
  }
  return out;
}

export function normalizeAllowedDepartureModes(
  value?: AllowedDepartureModes | null,
): AllowedDepartureModes {
  const out: AllowedDepartureModes = {};
  for (const day of DEPARTURE_WEEKDAYS) {
    const raw = value?.[day.key];
    if (!Array.isArray(raw)) continue;
    const modes = (["alone", "bus", "pickup", "accompanied"] as const).filter(
      (mode) => raw.includes(mode),
    );
    if (modes.length > 0) out[day.key] = modes;
  }
  return out;
}

/**
 * Stable identity of a departure plan, equal exactly when
 * allowedDepartureModesEqual is (both read the normalized plan, so missing days
 * and unordered mode arrays produce the same string).
 *
 * Needed where a plan has to be COMPARED AGAINST ITS OWN EARLIER SELF across an
 * async boundary rather than against another value: the companion refresh hook
 * remembers what a running save submitted, and a plan edited while that save is
 * in flight must not be mistaken for the one it carried (#1694).
 */
export function allowedDepartureModesFingerprint(
  value?: AllowedDepartureModes | null,
): string {
  const normalized = normalizeAllowedDepartureModes(value);
  return DEPARTURE_WEEKDAYS.map(
    (day) => `${day.key}:${(normalized[day.key] ?? []).join(",")}`,
  ).join("|");
}

/** Semantic equality of two departure plans: same modes on every weekday after
 *  normalization, so shape noise (missing days, duplicate or unordered mode
 *  arrays) does not read as a change. Used for dirty checks and for deciding
 *  whether a save can have touched the Laufgemeinschaft links (#1694). */
export function allowedDepartureModesEqual(
  a?: AllowedDepartureModes | null,
  b?: AllowedDepartureModes | null,
): boolean {
  const left = normalizeAllowedDepartureModes(a);
  const right = normalizeAllowedDepartureModes(b);
  return DEPARTURE_WEEKDAYS.every((day) => {
    const leftModes = left[day.key] ?? [];
    const rightModes = right[day.key] ?? [];
    return (
      leftModes.length === rightModes.length &&
      leftModes.every((mode, idx) => mode === rightModes[idx])
    );
  });
}

/** Reports whether any weekday allows the accompanied ("Mit anderem Kind")
 *  departure mode. Reads the unified allowed_departure_modes when present and
 *  falls back to the exclusive departure_days, so it works on every form shape
 *  (#1694). */
export function allowedModesIncludeAccompanied(
  allowed?: AllowedDepartureModes | null,
  departureDays?: DepartureDays | null,
): boolean {
  return accompaniedWeekdayKeys(allowed, departureDays).length > 0;
}

/** The weekdays on which the plan allows the accompanied ("Mit anderem Kind")
 *  departure mode, in Mon..Fri order. The "mit wem" requirement is checked PER
 *  DAY — a Monday link answers nothing for an accompanied Tuesday — so callers
 *  validating companion coverage need the days, not just a boolean (#1694). */
export function accompaniedWeekdayKeys(
  allowed?: AllowedDepartureModes | null,
  departureDays?: DepartureDays | null,
): DepartureDayKey[] {
  return DEPARTURE_WEEKDAYS.filter(
    (day) =>
      (allowed?.[day.key]?.includes("accompanied") ?? false) ||
      departureDays?.[day.key] === "accompanied",
  ).map((day) => day.key);
}

/** Folds the legacy bus/pickup maps into the unified map. Pickup wins on a
 *  contradictory day, mirroring the backend (#1610). */
export function departureDaysFromLegacy(
  bus?: BusDays | null,
  pickup?: PickupDays | null,
): DepartureDays {
  const out: DepartureDays = {};
  for (const day of DEPARTURE_WEEKDAYS) {
    if (pickup?.[day.key]) out[day.key] = "pickup";
    else if (bus?.[day.key]) out[day.key] = "bus";
  }
  return out;
}

export function allowedDepartureModesFromDeparture(
  value?: DepartureDays | null,
): AllowedDepartureModes {
  const out: AllowedDepartureModes = {};
  for (const day of DEPARTURE_WEEKDAYS) {
    const mode = value?.[day.key];
    if (mode === "bus" || mode === "pickup" || mode === "accompanied")
      out[day.key] = [mode];
  }
  return out;
}

function allowedDepartureModesFromLegacy(
  bus?: BusDays | null,
  pickup?: PickupDays | null,
): AllowedDepartureModes {
  const out: AllowedDepartureModes = {};
  for (const day of DEPARTURE_WEEKDAYS) {
    const modes: DepartureMode[] = [];
    if (bus?.[day.key]) modes.push("bus");
    if (pickup?.[day.key]) modes.push("pickup");
    if (modes.length > 0) out[day.key] = modes;
  }
  return out;
}

export function allowedDepartureToBusDays(
  value?: AllowedDepartureModes | null,
): BusDays {
  const out: BusDays = {};
  for (const day of DEPARTURE_WEEKDAYS) {
    if (value?.[day.key]?.includes("bus")) out[day.key] = true;
  }
  return out;
}

export function allowedDepartureToPickupDays(
  value?: AllowedDepartureModes | null,
): PickupDays {
  const out: PickupDays = {};
  for (const day of DEPARTURE_WEEKDAYS) {
    if (value?.[day.key]?.includes("pickup")) out[day.key] = true;
  }
  return out;
}

export function allowedDepartureToDepartureDays(
  value?: AllowedDepartureModes | null,
): DepartureDays {
  const out: DepartureDays = {};
  for (const day of DEPARTURE_WEEKDAYS) {
    const modes = value?.[day.key] ?? [];
    if (modes.includes("pickup")) out[day.key] = "pickup";
    else if (modes.includes("bus")) out[day.key] = "bus";
    else if (modes.includes("accompanied")) out[day.key] = "accompanied";
  }
  return out;
}

export function formatAllowedDepartureModes(
  value?: AllowedDepartureModes | null,
): string {
  const normalized = normalizeAllowedDepartureModes(value);
  const parts = DEPARTURE_WEEKDAYS.flatMap((day) => {
    const modes = normalized[day.key] ?? [];
    if (modes.length === 0) return [];
    return [
      `${day.label.slice(0, 2)}: ${modes
        .map((mode) => DEPARTURE_MODE_LABELS[mode])
        .join(", ")}`,
    ];
  });
  return parts.length > 0 ? parts.join("; ") : "Geht immer alleine";
}

/**
 * Compact day-only summary for the "Erlaubte Heimwege" badge. Lists just the
 * weekdays that have any departure mode configured (e.g. "Mo, Di, Fr"), mirroring
 * formatBusDays/formatPickupDays. Unlike formatAllowedDepartureModes it omits the
 * per-day modes, so the badge stays single-line even when every weekday has bus
 * and pickup — the full breakdown lives in the editor rows right below it.
 */
export function formatAllowedDepartureDays(
  value?: AllowedDepartureModes | null,
): string {
  const normalized = normalizeAllowedDepartureModes(value);
  const labels = DEPARTURE_WEEKDAYS.filter(
    (day) => (normalized[day.key]?.length ?? 0) > 0,
  ).map((day) => day.label.slice(0, 2));
  return labels.length > 0 ? labels.join(", ") : "Geht immer alleine";
}

export interface DepartureMatrixRow {
  readonly key: DepartureDayKey;
  /** Full weekday label, e.g. "Montag". */
  readonly label: string;
  /** Two-letter weekday label, e.g. "Mo". */
  readonly short: string;
  /** Allowed modes for the day in canonical order; never empty — a day with no
   *  configured arrangement folds to ["alone"] (the child leaves on its own). */
  readonly modes: DepartureMode[];
}

/**
 * Builds the per-weekday rows for the read-only "Erlaubte Heimwege" matrix
 * (Mo–Fr). A weekday with no configured mode folds to ["alone"], mirroring
 * formatAllowedDepartureModes' "goes alone" semantics. Modes are returned in the
 * canonical DEPARTURE_MODE_ORDER so badges render in a stable order.
 */
export function departureMatrixRows(
  value?: AllowedDepartureModes | null,
): DepartureMatrixRow[] {
  const normalized = normalizeAllowedDepartureModes(value);
  return DEPARTURE_WEEKDAYS.map((day) => {
    const modes = normalized[day.key] ?? [];
    const ordered = DEPARTURE_MODE_ORDER.filter((mode) => modes.includes(mode));
    return {
      key: day.key,
      label: day.label,
      short: day.label.slice(0, 2),
      modes: ordered.length > 0 ? ordered : ["alone"],
    };
  });
}

/**
 * True when at least one weekday has a non-alone arrangement (bus, pickup or
 * accompanied), i.e. the child does not simply go home alone every day. Used to
 * collapse the matrix to a single "Geht immer alleine" line when there is
 * nothing meaningful to show.
 */
export function hasAnyDepartureArrangement(
  value?: AllowedDepartureModes | null,
): boolean {
  const normalized = normalizeAllowedDepartureModes(value);
  return DEPARTURE_WEEKDAYS.some((day) =>
    (normalized[day.key] ?? []).some(
      (mode) => mode === "bus" || mode === "pickup" || mode === "accompanied",
    ),
  );
}

/**
 * Extract the school year (Klassenstufe) from a school_class string.
 * E.g. "Klasse 3a" → "3", "2b" → "2", "unknown" → null
 */
export function getSchoolYear(schoolClass: string): string | null {
  const match = /(\d+)/.exec(schoolClass);
  return match ? match[1]! : null;
}

// Scheduled checkout information
interface ScheduledCheckoutInfo {
  id: number;
  scheduled_for: string;
  reason?: string;
  scheduled_by: string;
}

// Backend types (from Go structs)
export interface BackendStudent {
  id: number;
  person_id: number;
  first_name: string;
  last_name: string;
  tag_id?: string;
  school_class: string;
  current_location?: string | null;
  location_since?: string | null; // When student entered current location (ISO timestamp)
  /** Hex color of the current room when set (Issue #1324). Drives badge color in LocationBadge. */
  current_room_color?: string | null;
  bus?: boolean;
  bus_days?: BusDays;
  pickup_days?: PickupDays;
  departure_days?: DepartureDays;
  allowed_departure_modes?: AllowedDepartureModes;
  sick?: boolean;
  sick_since?: string;
  excused?: boolean;
  excused_since?: string;
  class_trip?: boolean;
  class_trip_since?: string;
  day_planning_status?: "comes_today" | "not_coming_today";
  day_planning_reason?: string;
  day_planning_label?: string;
  // Letzter Betreuungstag (#2487) als "YYYY-MM-DD", leer wenn kein Ende
  // hinterlegt ist. care_ended sagt, ob dieser Tag schon vorbei ist.
  care_ends_on?: string;
  care_ended?: boolean;
  // Das Ende wurde von der Schule eingetragen ("Betreuung beenden"), es ist
  // also kein bloßes Ende der Anmeldephase. Nur ein solcher Austritt
  // lässt sich ändern oder stornieren (#2487).
  care_exit_recorded?: boolean;
  // Parent's note for a still-pending absence request covering today.
  // Informational only — it does
  // NOT change day_planning_status; the child stays "expected" until the OGS
  // confirms. Absent when there is no pending absence request for today.
  pending_excused_note?: string;
  guardian_name?: string; // Optional: Legacy field, use guardian_profiles instead
  guardian_contact?: string; // Optional: Legacy field, use guardian_profiles instead
  guardian_email?: string;
  guardian_phone?: string;
  group_id?: number;
  group_name?: string;
  address_street?: string;
  address_city?: string;
  address_postal_code?: string;
  scheduled_checkout?: ScheduledCheckoutInfo;
  extra_info?: string;
  departure_companion_note?: string;
  companion_student_ids?: number[];
  /** Write-only: the complete "läuft mit" list submitted with the plan. The id
   *  stays the string the frontend holds — the backend accepts a quoted
   *  decimal, which is exact for every int64. */
  companions?: { companion_student_id: string; weekdays: string[] }[];
  /**
   * Write-only: fingerprint of the list the client loaded before it built
   * `companions`. The backend compares it against the stored links under the
   * child's row lock and refuses a replacement built on a snapshot someone else
   * has since replaced (409 `companions_changed`) instead of deleting their
   * links.
   */
  companions_fingerprint?: string;
  /** Write-only: confirms widening a linked child's own departure plan. */
  extend_companion_plans?: boolean;
  /** Write-only: the children and weekdays that confirmation actually covered. */
  confirmed_companion_extensions?: {
    companion_student_id: string;
    weekdays: string[];
  }[];
  birthday?: string;
  health_info?: string;
  supervisor_notes?: string;
  pickup_status?: string;
  pickup_time?: string; // Today's effective pickup time (HH:MM)
  pickup_is_exception?: boolean; // True if today's pickup time is an exception
  pickup_notes?: string; // Exception reason or schedule notes
  arrival_time?: string; // Today's effective arrival time (HH:MM)
  arrival_is_exception?: boolean; // True if today's arrival time is an exception
  arrival_notes?: string; // Exception reason or schedule notes
  actual_arrival_time?: string; // Today's actual arrival time from attendance (HH:MM)
  actual_pickup_time?: string; // Today's actual pickup time from attendance (HH:MM)
  has_full_access?: boolean;
  // Photo (gated by operations.student_photos_enabled). Empty string or
  // undefined when no photo / feature off / no consent — the <Avatar>
  // component falls back to initials in any of those cases.
  photo_url?: string;
  photo_consent_given?: boolean;
  photo_consent_given_at?: string;
  photo_consent_given_by?: number;
  // Other consents the parent ticked at enrollment time. Stamped by
  // the decision service on approval from request.consent_flags.
  // Null/undefined = no consent recorded; ISO timestamp = consent given.
  agb_accepted_at?: string;
  data_processing_accepted_at?: string;
  email_contact_accepted_at?: string;
  consents?: StudentConsent[];
  created_at: string;
  updated_at: string;
}

export type StudentConsentKey = ConsentKey;

export type StudentConsent = ConsentRecord;

/** Backend wire shape for `GET /api/students?view=slim`. */
export interface BackendSlimStudent {
  id: number;
  first_name: string;
  last_name: string;
  school_class: string;
  current_location: string;
  location_since?: string;
  current_room_color?: string | null;
  group_id?: number;
  group_name?: string;
  sick: boolean;
  sick_since?: string;
  excused: boolean;
  excused_since?: string;
  class_trip: boolean;
  class_trip_since?: string;
  day_planning_status?: "comes_today" | "not_coming_today";
  day_planning_reason?: string;
  day_planning_label?: string;
  pending_excused_note?: string;
  departure_modes?: DepartureMode[];
  departure_label?: string;
  pickup_time?: string;
  pickup_is_exception?: boolean;
  pickup_notes?: string;
  arrival_time?: string;
  arrival_is_exception?: boolean;
  arrival_notes?: string;
  actual_arrival_time?: string;
  actual_pickup_time?: string;
  companion_student_ids?: number[];
  photo_url?: string;
  photo_consent_given?: boolean;
  has_full_access: boolean;
}

// Supervisor contact information
export interface SupervisorContact {
  id: number;
  first_name: string;
  last_name: string;
  email?: string;
  phone?: string;
  role: string;
}

// Detailed student response with access control
export interface BackendStudentDetail extends BackendStudent {
  has_full_access: boolean;
  has_write_access: boolean;
  /** Absence actions only (krank / entschuldigt / Klassenfahrt) — a superset of
   *  has_write_access in a school without fixed groups (#2232). */
  has_absence_write_access?: boolean;
  group_supervisors?: SupervisorContact[];
  attendance_log_enabled: boolean;
}

// Privacy consent types
export interface BackendPrivacyConsent {
  id: number;
  student_id: number;
  policy_version: string;
  accepted: boolean;
  accepted_at?: string;
  expires_at?: string;
  duration_days?: number;
  renewal_required: boolean;
  data_retention_days: number;
  details?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface PrivacyConsent {
  id: string;
  studentId: string;
  policyVersion: string;
  accepted: boolean;
  acceptedAt?: Date;
  expiresAt?: Date;
  durationDays?: number;
  renewalRequired: boolean;
  dataRetentionDays: number;
  details?: Record<string, unknown>;
  createdAt: Date;
  updatedAt: Date;
}

// Student attendance status (updated to use attendance-based terminology)
// Now includes specific location details from backend
// Note: StudentLocation is a simple string type, no separate alias needed

// Frontend types (mapped from backend)
export interface Student {
  id: string;
  name: string; // Derived from FirstName + SecondName
  first_name?: string;
  second_name?: string;
  school_class: string;
  grade?: string;
  studentId?: string;
  group_name?: string;
  group_id?: string;
  // Current attendance status of student (location string)
  current_location: string;
  // When student entered current location (only for hasFullAccess users)
  location_since?: string;
  /** Hex color of the current room when set (Issue #1324). Empty string treated as null. */
  current_room_color?: string | null;
  // Transportation method (separate from attendance)
  takes_bus?: boolean;
  bus?: boolean; // Administrative permission flag (Buskind), not attendance status
  bus_days?: BusDays;
  pickup_days?: PickupDays;
  // Unified per-weekday departure mode (alone/bus/pickup), the source of truth
  // for how the child leaves; bus_days/pickup_days are derived from it (#1610).
  departure_days?: DepartureDays;
  allowed_departure_modes?: AllowedDepartureModes;
  departure_modes?: DepartureMode[];
  departure_label?: string;
  // Sickness status (only visible to supervisors/admins)
  sick?: boolean;
  sick_since?: string;
  // Excused status (kind is not attending the OGS today, only visible to supervisors/admins)
  excused?: boolean;
  excused_since?: string;
  // Class trip status, counted as a known absence but rendered separately
  class_trip?: boolean;
  class_trip_since?: string;
  day_planning_status?: "comes_today" | "not_coming_today";
  day_planning_reason?: string;
  day_planning_label?: string;
  // Letzter Betreuungstag (#2487) als "YYYY-MM-DD", leer wenn kein Ende
  // hinterlegt ist. care_ended sagt, ob dieser Tag schon vorbei ist.
  care_ends_on?: string;
  care_ended?: boolean;
  // Das Ende wurde von der Schule eingetragen ("Betreuung beenden"), es ist
  // also kein bloßes Ende der Anmeldephase. Nur ein solcher Austritt
  // lässt sich ändern oder stornieren (#2487).
  care_exit_recorded?: boolean;
  // Parent's note for a still-pending "entschuldigt" request covering today.
  // Informational only — the child stays "expected" (day_planning_status
  // unchanged) until an office/admin confirms the request.
  pending_excused_note?: string;
  name_lg?: string;
  contact_lg?: string;
  guardian_email?: string;
  guardian_phone?: string;
  address_street?: string;
  address_city?: string;
  address_postal_code?: string;
  custom_users_id?: string;
  // Privacy consent data (fetched separately)
  privacy_consent?: PrivacyConsent;
  // Privacy consent fields for form handling
  privacy_consent_accepted?: boolean;
  data_retention_days?: number;
  // Additional fields for access control
  has_full_access?: boolean;
  has_write_access?: boolean;
  /** May report/clear absences for this child, even without Stammdaten write
   *  access (open care, #2232). */
  has_absence_write_access?: boolean;
  group_supervisors?: SupervisorContact[];
  // Feature flag: tenant has attendance log enabled
  attendance_log_enabled?: boolean;
  // Extra information visible only to supervisors
  extra_info?: string;
  // Free-text "mit wem" for the accompanied departure mode (#1694)
  departure_companion_note?: string;
  /** Children this child walks home with on the shown day (Laufgemeinschaft).
   *  Only present when the list was fetched with include_companions. Backend
   *  int64 ids, carried as strings per the type-mapping rule. */
  companion_student_ids?: string[];
  /** The child's Laufgemeinschaft. Loaded from the companions endpoint and
   *  submitted back with the departure plan it belongs to; omitting it leaves
   *  the stored links untouched. */
  companions?: StudentCompanion[];
  /** Write-only: fingerprint of the LOADED list, so the backend can refuse a
   *  replacement built on a snapshot someone else has since replaced instead of
   *  deleting their links (409 `companions_changed`). */
  companions_fingerprint?: string;
  /** Write-only: confirms widening a linked child's own departure plan. */
  extend_companion_plans?: boolean;
  /** Write-only: the children and weekdays that confirmation actually covered.
   *  Without them the backend treats the confirmation as unanswered. */
  confirmed_companion_extensions?: CompanionExtensionConfirmation[];
  birthday?: string;
  health_info?: string;
  supervisor_notes?: string;
  pickup_status?: string;
  pickup_time?: string; // Today's effective pickup time (HH:MM)
  pickup_is_exception?: boolean; // True if today's pickup time is an exception
  pickup_notes?: string; // Exception reason or schedule notes
  arrival_time?: string; // Today's effective arrival time (HH:MM)
  arrival_is_exception?: boolean; // True if today's arrival time is an exception
  arrival_notes?: string; // Exception reason or schedule notes
  actual_arrival_time?: string; // Today's actual arrival time from attendance (HH:MM)
  actual_pickup_time?: string; // Today's actual pickup time from attendance (HH:MM)
  // Photo + consent (gated server-side by operations.student_photos_enabled
  // setting). photo_url empty/missing is the universal "no photo to show"
  // signal; consumers do NOT need to read the setting separately — when the
  // setting is off the backend simply returns photo_url unset.
  photo_url?: string;
  photo_consent_given?: boolean;
  photo_consent_given_at?: string;
  photo_consent_given_by?: number;
  agb_accepted_at?: string;
  data_processing_accepted_at?: string;
  email_contact_accepted_at?: string;
  consents?: StudentConsent[];
}

// Mapping functions
export function mapStudentResponse(
  backendStudent: BackendStudent,
): Student & { scheduled_checkout?: ScheduledCheckoutInfo } {
  // Construct the full name from first and last name
  const firstName = backendStudent.first_name || "";
  const lastName = backendStudent.last_name || "";
  const name = `${firstName} ${lastName}`.trim();

  // Map backend attendance status with normalization for legacy values (e.g., "Abwesend")
  const current_location = normalizeLocation(backendStudent.current_location);

  const mapped = {
    id: String(backendStudent.id),
    name: name,
    first_name: backendStudent.first_name,
    second_name: backendStudent.last_name, // Map last_name to second_name for frontend compatibility
    school_class: backendStudent.school_class,
    grade: undefined, // Not provided by backend
    studentId: backendStudent.tag_id,
    group_name: backendStudent.group_name,
    group_id: backendStudent.group_id
      ? String(backendStudent.group_id)
      : undefined,
    // New attendance-based system
    current_location,
    location_since: backendStudent.location_since ?? undefined,
    current_room_color: backendStudent.current_room_color ?? null,
    takes_bus: undefined,
    // bus is derived from bus_days, the single source of truth (#1582). The
    // backend already derives it, but we recompute here so the legacy boolean
    // stays correct even if an older payload omits it.
    bus: busDaysHaveAny(backendStudent.bus_days),
    bus_days: normalizeBusDays(backendStudent.bus_days),
    pickup_days: normalizePickupDays(backendStudent.pickup_days),
    allowed_departure_modes: backendStudent.allowed_departure_modes
      ? normalizeAllowedDepartureModes(backendStudent.allowed_departure_modes)
      : backendStudent.departure_days
        ? allowedDepartureModesFromDeparture(backendStudent.departure_days)
        : allowedDepartureModesFromLegacy(
            backendStudent.bus_days,
            backendStudent.pickup_days,
          ),
    // departure_days is authoritative; fall back to folding the legacy maps for
    // older payloads that predate it (#1610).
    departure_days: backendStudent.departure_days
      ? normalizeDepartureDays(backendStudent.departure_days)
      : departureDaysFromLegacy(
          backendStudent.bus_days,
          backendStudent.pickup_days,
        ),
    sick: backendStudent.sick ?? false, // Sickness status
    sick_since: backendStudent.sick_since,
    excused: backendStudent.excused ?? false, // Excused from attending today
    excused_since: backendStudent.excused_since,
    class_trip: backendStudent.class_trip ?? false,
    class_trip_since: backendStudent.class_trip_since,
    day_planning_status: backendStudent.day_planning_status,
    day_planning_reason: backendStudent.day_planning_reason,
    day_planning_label: backendStudent.day_planning_label,
    care_ends_on: backendStudent.care_ends_on,
    care_ended: backendStudent.care_ended ?? false,
    care_exit_recorded: backendStudent.care_exit_recorded ?? false,
    pending_excused_note: backendStudent.pending_excused_note,
    name_lg: backendStudent.guardian_name ?? undefined,
    contact_lg: backendStudent.guardian_contact ?? undefined,
    guardian_email: backendStudent.guardian_email,
    guardian_phone: backendStudent.guardian_phone,
    address_street: backendStudent.address_street,
    address_city: backendStudent.address_city,
    address_postal_code: backendStudent.address_postal_code,
    custom_users_id: undefined, // Not provided by backend
    extra_info: backendStudent.extra_info,
    departure_companion_note: backendStudent.departure_companion_note,
    // Backend int64 ids → strings at the mapping boundary (type-mapping rule);
    // ids above Number.MAX_SAFE_INTEGER must never live as JS numbers.
    companion_student_ids: backendStudent.companion_student_ids?.map(String),
    birthday: backendStudent.birthday,
    health_info: backendStudent.health_info,
    supervisor_notes: backendStudent.supervisor_notes,
    pickup_status: backendStudent.pickup_status,
    pickup_time: backendStudent.pickup_time,
    pickup_is_exception: backendStudent.pickup_is_exception,
    pickup_notes: backendStudent.pickup_notes,
    arrival_time: backendStudent.arrival_time,
    arrival_is_exception: backendStudent.arrival_is_exception,
    arrival_notes: backendStudent.arrival_notes,
    actual_arrival_time: backendStudent.actual_arrival_time,
    actual_pickup_time: backendStudent.actual_pickup_time,
    has_full_access: backendStudent.has_full_access,
    photo_url: backendStudent.photo_url,
    photo_consent_given: backendStudent.photo_consent_given,
    photo_consent_given_at: backendStudent.photo_consent_given_at,
    photo_consent_given_by: backendStudent.photo_consent_given_by,
    agb_accepted_at: backendStudent.agb_accepted_at,
    data_processing_accepted_at: backendStudent.data_processing_accepted_at,
    email_contact_accepted_at: backendStudent.email_contact_accepted_at,
    consents: backendStudent.consents,
  };

  // Add scheduled checkout info if present
  if (backendStudent.scheduled_checkout) {
    return {
      ...mapped,
      scheduled_checkout: backendStudent.scheduled_checkout,
    };
  }

  return mapped;
}

/**
 * Maps the Kindersuche projection without rebuilding the omitted full-detail
 * fields. In particular, this must not synthesize empty weekday departure
 * maps because the route serializes the mapped value back to the browser.
 */
export function mapSlimStudentResponse(
  backendStudent: BackendSlimStudent,
): Student {
  const firstName = backendStudent.first_name || "";
  const lastName = backendStudent.last_name || "";

  return {
    id: String(backendStudent.id),
    name: `${firstName} ${lastName}`.trim(),
    first_name: backendStudent.first_name,
    second_name: backendStudent.last_name,
    school_class: backendStudent.school_class,
    group_name: backendStudent.group_name,
    group_id: backendStudent.group_id
      ? String(backendStudent.group_id)
      : undefined,
    current_location: normalizeLocation(backendStudent.current_location),
    location_since: backendStudent.location_since,
    current_room_color: backendStudent.current_room_color,
    sick: backendStudent.sick,
    sick_since: backendStudent.sick_since,
    excused: backendStudent.excused,
    excused_since: backendStudent.excused_since,
    class_trip: backendStudent.class_trip,
    class_trip_since: backendStudent.class_trip_since,
    day_planning_status: backendStudent.day_planning_status,
    day_planning_reason: backendStudent.day_planning_reason,
    day_planning_label: backendStudent.day_planning_label,
    pending_excused_note: backendStudent.pending_excused_note,
    departure_modes: backendStudent.departure_modes,
    departure_label: backendStudent.departure_label,
    pickup_time: backendStudent.pickup_time,
    pickup_is_exception: backendStudent.pickup_is_exception,
    pickup_notes: backendStudent.pickup_notes,
    arrival_time: backendStudent.arrival_time,
    arrival_is_exception: backendStudent.arrival_is_exception,
    arrival_notes: backendStudent.arrival_notes,
    actual_arrival_time: backendStudent.actual_arrival_time,
    actual_pickup_time: backendStudent.actual_pickup_time,
    companion_student_ids: backendStudent.companion_student_ids?.map(String),
    photo_url: backendStudent.photo_url,
    photo_consent_given: backendStudent.photo_consent_given,
    has_full_access: backendStudent.has_full_access,
  };
}

// Map array of students
export function mapStudentsResponse(
  backendStudents: BackendStudent[],
): Student[] {
  return backendStudents.map(mapStudentResponse);
}

// Map a single student
export function mapSingleStudentResponse(response: {
  data: BackendStudent;
}): Student {
  return mapStudentResponse(response.data);
}

// Map student detail response (includes access control info)
export function mapStudentDetailResponse(
  backendStudent: BackendStudentDetail,
): Student {
  // First map the basic student data
  const student = mapStudentResponse(backendStudent);

  // Then add the additional fields
  student.has_full_access = backendStudent.has_full_access;
  student.has_write_access = backendStudent.has_write_access;
  student.has_absence_write_access =
    backendStudent.has_absence_write_access ?? backendStudent.has_write_access;
  student.group_supervisors = backendStudent.group_supervisors;
  student.attendance_log_enabled = backendStudent.attendance_log_enabled;

  return student;
}

// Prepare frontend student for backend
export function prepareStudentForBackend(
  student: Partial<Student> & {
    tag_id?: string;
    guardian_email?: string;
    guardian_phone?: string;
    address_street?: string;
    address_city?: string;
    address_postal_code?: string;
    extra_info?: string;
    departure_companion_note?: string;
    birthday?: string;
    health_info?: string;
    supervisor_notes?: string;
    pickup_status?: string;
    companions?: { companion_student_id: string; weekdays: string[] }[];
    companions_fingerprint?: string;
    extend_companion_plans?: boolean;
    confirmed_companion_extensions?: CompanionExtensionConfirmation[];
  },
): Partial<BackendStudent> {
  return {
    id: student.id ? Number.parseInt(student.id, 10) : undefined,
    first_name: student.first_name,
    last_name: student.second_name, // Map second_name to last_name for backend
    school_class: student.school_class,
    current_location: student.current_location
      ? normalizeLocation(student.current_location)
      : undefined,
    // bus_days is the single source of truth (#1582); we no longer send the
    // legacy bus boolean. Only send bus_days when explicitly provided so
    // partial updates (e.g. sick/excused toggles) leave the persisted Buskind
    // days untouched (the key is omitted, so the backend keeps the DB value).
    bus_days:
      student.bus_days !== undefined
        ? normalizeBusDays(student.bus_days)
        : undefined,
    pickup_days:
      student.pickup_days !== undefined
        ? normalizePickupDays(student.pickup_days)
        : undefined,
    allowed_departure_modes:
      student.allowed_departure_modes !== undefined
        ? normalizeAllowedDepartureModes(student.allowed_departure_modes)
        : undefined,
    // departure_days is the unified source of truth (#1610); when provided the
    // backend derives bus_days/pickup_days/pickup_status from it.
    departure_days:
      student.departure_days !== undefined
        ? normalizeDepartureDays(student.departure_days)
        : undefined,
    // REMOVED: guardian_name and guardian_contact - deprecated fields
    // Use guardian_profiles system instead
    group_id: student.group_id
      ? Number.parseInt(student.group_id, 10)
      : undefined,
    tag_id: student.tag_id,
    guardian_email: student.guardian_email,
    guardian_phone: student.guardian_phone,
    address_street: student.address_street,
    address_city: student.address_city,
    address_postal_code: student.address_postal_code,
    extra_info: student.extra_info,
    departure_companion_note: student.departure_companion_note,
    // Laufgemeinschaft ("läuft mit"). Only sent when the caller actually
    // edited it — an omitted key leaves the stored links untouched, while []
    // clears them.
    // The id travels as the string the frontend holds: the backend accepts a
    // quoted decimal for exactly this reason. Converting to a JS number would
    // round an id beyond Number.MAX_SAFE_INTEGER to a DIFFERENT child's id.
    companions: student.companions?.map((companion) => ({
      companion_student_id: companion.companion_student_id,
      weekdays: companion.weekdays,
    })),
    // The fingerprint of the list the form LOADED, so the backend can refuse a
    // write built on a snapshot someone else has since replaced instead of
    // silently deleting their links.
    //
    // Sent WITHOUT a list too: the departure plan removes links on its own (the
    // backend trims every weekday the new plan no longer allows), and the forms
    // resubmit their whole plan on every save — so a plan that rode along on a
    // stale load deletes a fresh edge exactly like a stale list would. The
    // backend refuses such a removal unless this claim matches what is stored.
    companions_fingerprint: student.companions_fingerprint,
    extend_companion_plans: student.extend_companion_plans,
    // What the user confirmed, not just that they confirmed something: the
    // backend refuses to widen a conflict these entries do not cover.
    confirmed_companion_extensions: student.confirmed_companion_extensions?.map(
      (entry) => ({
        companion_student_id: entry.companion_student_id,
        weekdays: entry.weekdays,
      }),
    ),
    // Convert empty string to undefined for date fields (Go backend expects null or valid date)
    birthday:
      student.birthday && student.birthday.trim() !== ""
        ? student.birthday
        : undefined,
    health_info: student.health_info,
    supervisor_notes: student.supervisor_notes,
    pickup_status: student.pickup_status,
    sick: student.sick,
    excused: student.excused,
    // Photo consent toggle flows through the regular student PUT — see
    // backend applyPhotoConsent for the false→true (stamp) and true→false
    // (clear photo + metadata) transitions.
    photo_consent_given: student.photo_consent_given,
  };
}

// Request/Response types

export interface UpdateStudentRequest {
  first_name?: string;
  second_name?: string; // Will be mapped to last_name for backend
  school_class?: string;
  group_id?: string;
  name_lg?: string; // Guardian name
  contact_lg?: string; // Guardian contact
  tag_id?: string;
  guardian_email?: string;
  guardian_phone?: string;
  address_street?: string;
  address_city?: string;
  address_postal_code?: string;
  extra_info?: string;
  departure_companion_note?: string;
  birthday?: string;
  health_info?: string;
  supervisor_notes?: string;
  pickup_status?: string;
  bus?: boolean;
  bus_days?: BusDays;
  pickup_days?: PickupDays;
  departure_days?: DepartureDays;
  allowed_departure_modes?: AllowedDepartureModes;
  sick?: boolean;
  /** Optional free-text reason stamped on today's sick day when marking sick. */
  sick_reason?: string;
  excused?: boolean;
  /**
   * Parental photo-consent flag. Send `true` to record consent (server stamps
   * who/when), `false` to withdraw — withdrawing also deletes any existing
   * photo file in the same DB transaction. Omit to leave consent unchanged.
   */
  photo_consent_given?: boolean;
}

// Backend request type (for actual API calls)
export interface BackendUpdateRequest {
  first_name?: string;
  last_name?: string;
  tag_id?: string;
  school_class?: string;
  guardian_name?: string;
  guardian_contact?: string;
  guardian_email?: string;
  guardian_phone?: string;
  address_street?: string;
  address_city?: string;
  address_postal_code?: string;
  group_id?: number;
  extra_info?: string;
  departure_companion_note?: string;
  birthday?: string;
  health_info?: string;
  supervisor_notes?: string;
  pickup_status?: string;
  bus?: boolean;
  bus_days?: BusDays;
  pickup_days?: PickupDays;
  departure_days?: DepartureDays;
  allowed_departure_modes?: AllowedDepartureModes;
  sick?: boolean;
  sick_reason?: string;
  excused?: boolean;
  photo_consent_given?: boolean;
  /** Laufgemeinschaft: the child's complete "läuft mit" list. Omit to leave
   *  the links untouched; [] clears them. */
  companions?: { companion_student_id: string; weekdays: string[] }[];
  /** Fingerprint of the companion list the client loaded before building
   *  `companions`. Mandatory whenever `companions` is present (including []) —
   *  the backend rejects the update without it. Build it with
   *  `companionsFingerprint()`. */
  companions_fingerprint?: string;
  /** Confirms widening a linked child's own departure plan (answer to the 409
   *  conflict). */
  extend_companion_plans?: boolean;
  /** The children and weekdays that confirmation covered. */
  confirmed_companion_extensions?: {
    companion_student_id: string;
    weekdays: string[];
  }[];
}

// Map privacy consent from backend to frontend
export function mapPrivacyConsentResponse(
  backendConsent: BackendPrivacyConsent,
): PrivacyConsent {
  return {
    id: String(backendConsent.id),
    studentId: String(backendConsent.student_id),
    policyVersion: backendConsent.policy_version,
    accepted: backendConsent.accepted,
    acceptedAt: backendConsent.accepted_at
      ? new Date(backendConsent.accepted_at)
      : undefined,
    expiresAt: backendConsent.expires_at
      ? new Date(backendConsent.expires_at)
      : undefined,
    durationDays: backendConsent.duration_days,
    renewalRequired: backendConsent.renewal_required,
    dataRetentionDays: backendConsent.data_retention_days,
    details: backendConsent.details,
    createdAt: new Date(backendConsent.created_at),
    updatedAt: new Date(backendConsent.updated_at),
  };
}

/** Field mappings from UpdateStudentRequest to BackendUpdateRequest */
type FieldMapping = {
  source: keyof UpdateStudentRequest;
  target: keyof BackendUpdateRequest;
};

/** Direct field mappings (no transformation needed) */
const DIRECT_FIELD_MAPPINGS: FieldMapping[] = [
  { source: "first_name", target: "first_name" },
  { source: "second_name", target: "last_name" },
  { source: "tag_id", target: "tag_id" },
  { source: "school_class", target: "school_class" },
  { source: "name_lg", target: "guardian_name" },
  { source: "contact_lg", target: "guardian_contact" },
  { source: "guardian_email", target: "guardian_email" },
  { source: "guardian_phone", target: "guardian_phone" },
  { source: "address_street", target: "address_street" },
  { source: "address_city", target: "address_city" },
  { source: "address_postal_code", target: "address_postal_code" },
  { source: "extra_info", target: "extra_info" },
  { source: "departure_companion_note", target: "departure_companion_note" },
  { source: "birthday", target: "birthday" },
  { source: "health_info", target: "health_info" },
  { source: "supervisor_notes", target: "supervisor_notes" },
  { source: "pickup_status", target: "pickup_status" },
  // departure_days is the single source of truth (#1610); bus_days/pickup_days
  // are still accepted by the backend for partial legacy updates.
  { source: "departure_days", target: "departure_days" },
  { source: "allowed_departure_modes", target: "allowed_departure_modes" },
  { source: "bus_days", target: "bus_days" },
  { source: "pickup_days", target: "pickup_days" },
  { source: "sick", target: "sick" },
  { source: "sick_reason", target: "sick_reason" },
  { source: "excused", target: "excused" },
  { source: "photo_consent_given", target: "photo_consent_given" },
];

/**
 * Copies defined fields from source to target using field mappings
 */
function applyFieldMappings(
  request: UpdateStudentRequest,
  backendRequest: BackendUpdateRequest,
): void {
  for (const { source, target } of DIRECT_FIELD_MAPPINGS) {
    const value = request[source];
    if (value !== undefined) {
      // Type assertion needed due to union type complexity
      (backendRequest as Record<string, unknown>)[target] = value;
    }
  }
}

// Map frontend update request to backend format
export function mapUpdateRequestToBackend(
  request: UpdateStudentRequest,
): BackendUpdateRequest {
  const backendRequest: BackendUpdateRequest = {};

  // Apply all direct field mappings
  applyFieldMappings(request, backendRequest);

  // Handle group_id separately (requires parseInt transformation)
  if (request.group_id !== undefined) {
    backendRequest.group_id = Number.parseInt(request.group_id, 10);
  }

  return backendRequest;
}

// Helper functions
export function formatStudentName(student: Student): string {
  if (student.name) return student.name;

  const fallback =
    [student.first_name, student.second_name].filter(Boolean).join(" ") ||
    "Unnamed Student";

  return fallback;
}

export function formatStudentStatus(student: Student): string {
  const parsed = parseLocation(student.current_location);
  return parsed.room ?? parsed.status ?? LOCATION_STATUSES.UNKNOWN;
}

/**
 * Extracts guardian contact information with clear precedence rules.
 * This function provides a consistent way to determine which contact
 * information to display for a student's guardian.
 *
 * Precedence order:
 * 1. guardian_email - Primary contact method (if available)
 * 2. contact_lg - Legacy contact field (fallback)
 * 3. Empty string - Final fallback when no contact info is available
 *
 * @param studentData - Object containing guardian contact fields
 * @returns The most appropriate guardian contact information
 */
export function extractGuardianContact(studentData: {
  guardian_email?: string;
  contact_lg?: string;
}): string {
  // First priority: Use guardian_email if available
  if (studentData.guardian_email) {
    return studentData.guardian_email;
  }

  // Second priority: Fall back to legacy contact_lg field
  if (studentData.contact_lg) {
    return studentData.contact_lg;
  }

  // Final fallback: Return empty string when no contact info available
  return "";
}

export function getStatusColor(student: Student): string {
  if (isPresentLocation(student.current_location)) return "green";
  if (isSchoolyardLocation(student.current_location)) return "yellow";
  if (isTransitLocation(student.current_location)) return "purple";
  if (isHomeLocation(student.current_location)) return "red";
  return "gray";
}
