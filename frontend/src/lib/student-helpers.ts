// The student as the browser holds it, and the small display helpers every
// student surface shares. The backend wire shape lives in student-wire.ts,
// the Heimweg plan in departure-plan.ts.

import { schoolClassLabel } from "./school-class-label";
import type { ClassHandover } from "~/lib/class-handover";
import type { AttendanceActor } from "~/lib/attendance-actor";
import type {
  CompanionExtensionConfirmation,
  StudentCompanion,
} from "./student-companion-api";
import type { ConsentRecord } from "./consent-types";
import type {
  AllowedDepartureModes,
  BusDays,
  DepartureDays,
  DepartureMode,
  PickupDays,
} from "./departure-plan";

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

/**
 * Extract the school year (Klassenstufe) from a school_class string.
 * E.g. "Klasse 3a" → "3", "2b" → "2", "unknown" → null
 */
export function getSchoolYear(schoolClass: string): string | null {
  const match = /(\d+)/.exec(schoolClass);
  return match ? match[1]! : null;
}

export type StudentConsent = ConsentRecord;

// Supervisor contact information
export interface SupervisorContact {
  id: number;
  first_name: string;
  last_name: string;
  email?: string;
  phone?: string;
  role: string;
}

/** A child's privacy consent, or the school's defaults when none is stored. */
export interface PrivacyConsent {
  accepted: boolean;
  dataRetentionDays: number;
}

/**
 * Hinweis der Lehrkraft für heute an die OGS (#4795), geschrieben in moto
 * schule für das Kind oder seine ganze Klasse.
 */
export interface TeacherHint {
  body: string;
  author_name?: string;
  // Gesetzt, wenn der Hinweis für die ganze Klasse gilt.
  school_class?: string;
}

/** Die Zeile, mit der eine OGS-Ansicht einen Hinweis der Lehrkraft zeigt. */
export function teacherHintText(hint: TeacherHint): string {
  const author = hint.author_name ? ` ${hint.author_name}` : "";
  // schoolClassLabel macht aus „3b“ und „Klasse 3b“ beide „Klasse 3b“.
  const target = hint.school_class
    ? ` an die ${schoolClassLabel(hint.school_class)}`
    : "";
  return `Hinweis der Lehrkraft${author}${target}: ${hint.body}`;
}

/**
 * Der React-Schlüssel eines Hinweises. Je Lehrkraft, Kind und Ziel gibt es
 * einen Hinweis; zwei Lehrkräfte mit demselben Text bleiben getrennt.
 */
export function teacherHintKey(hint: TeacherHint): string {
  return `${hint.school_class ?? ""}|${hint.author_name ?? ""}|${hint.body}`;
}

export interface Student {
  id: string;
  name: string; // Derived from FirstName + SecondName
  first_name?: string;
  second_name?: string;
  school_class: string;
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
  bus?: boolean; // Administrative permission flag (Buskind), not attendance status
  bus_days?: BusDays;
  pickup_days?: PickupDays;
  // Unified per-weekday departure mode (alone/bus/pickup), the source of truth
  // for how the child leaves; bus_days/pickup_days are derived from it (#1610).
  departure_days?: DepartureDays;
  allowed_departure_modes?: AllowedDepartureModes;
  departure_modes?: DepartureMode[];
  departure_label?: string;
  // The listed day's Abholart was set for that day only (#3945, #3946).
  departure_is_exception?: boolean;
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
  // Hinweise der Lehrkraft für heute (#4795); nur mit Lesezugriff.
  teacher_hints?: TeacherHint[];
  // Übergabe nach dem Unterricht (#4794): die Lehrkraft hat das Kind heute
  // mit der Klasse übergeben, „da“ oder „nicht da“.
  class_handover?: ClassHandover;
  address_street?: string;
  address_city?: string;
  address_postal_code?: string;
  // Privacy consent fields for form handling
  privacy_consent_accepted?: boolean;
  data_retention_days?: number | null;
  // Additional fields for access control
  has_full_access?: boolean;
  has_write_access?: boolean;
  /** Class-trip authority, independent of Stammdaten write access. */
  has_absence_write_access?: boolean;
  has_sick_excused_write_access?: boolean;
  group_supervisors?: SupervisorContact[];
  // Feature flags of the tenant, carried by the detail read
  attendance_log_enabled?: boolean;
  feedback_enabled?: boolean;
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
   *  deleting their links (409 `students.companions_changed`). */
  companions_fingerprint?: string;
  /** Write-only: confirms widening a linked child's own departure plan. */
  extend_companion_plans?: boolean;
  /** Write-only: the children and weekdays that confirmation actually covered.
   *  Without them the backend treats the confirmation as unanswered. */
  confirmed_companion_extensions?: CompanionExtensionConfirmation[];
  birthday?: string;
  health_info?: string;
  pickup_status?: string;
  pickup_time?: string; // Today's effective pickup time (HH:MM)
  pickup_is_exception?: boolean; // True if today's pickup time is an exception
  pickup_notes?: string; // Exception reason or schedule notes
  arrival_time?: string; // Today's effective arrival time (HH:MM)
  arrival_is_exception?: boolean; // True if today's arrival time is an exception
  arrival_notes?: string; // Exception reason or schedule notes
  actual_arrival_time?: string; // Today's actual arrival time from attendance (HH:MM)
  actual_pickup_time?: string; // Today's actual pickup time from attendance (HH:MM)
  actual_pickup_note?: string; // Why the child went home early today (#3324)
  actual_arrival_by?: AttendanceActor | null; // Who recorded today's arrival (#4761)
  actual_pickup_by?: AttendanceActor | null; // Who recorded today's pickup (#4761)
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

/** One page of a student list. */
export interface StudentPage {
  data: Student[];
  pagination: {
    current_page: number;
    page_size: number;
    total_pages: number;
    total_records: number;
  };
}

/**
 * What a student write carries. The Laufgemeinschaft rides along with the
 * departure plan it belongs to; its ids stay strings all the way to the
 * backend, which accepts a quoted decimal.
 */
export type StudentUpdate = Omit<Partial<Student>, "companions"> & {
  companions?: { companion_student_id: string; weekdays: string[] }[];
};

/** An active care offering that counts as care, as a selection lists it. */
export interface CareOfferingOption {
  readonly id: string;
  readonly name: string;
}
