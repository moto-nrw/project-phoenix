// The student as the backend sends and receives it, and the one translation
// to and from the browser's Student. The BFF routes are its callers; the
// browser never sees these shapes.

import type { ClassHandover } from "~/lib/class-handover";
import {
  parseAttendanceActor,
  type AttendanceActor,
} from "~/lib/attendance-actor";
import type { CompanionExtensionConfirmation } from "./student-companion-api";
import { normalizeLocation } from "./location-helper";
import {
  allowedDepartureModesFromDeparture,
  allowedDepartureModesFromLegacy,
  busDaysHaveAny,
  departureDaysFromLegacy,
  normalizeAllowedDepartureModes,
  normalizeBusDays,
  normalizeDepartureDays,
  normalizePickupDays,
  type AllowedDepartureModes,
  type BusDays,
  type DepartureDays,
  type DepartureMode,
  type PickupDays,
} from "./departure-plan";
import type {
  Student,
  StudentConsent,
  StudentUpdate,
  SupervisorContact,
  TeacherHint,
} from "./student-helpers";

/** `GET /api/students?view=names` (#4996). */
export interface BackendNameStudent {
  id: number;
  first_name: string;
  last_name: string;
}

/** The fields the Kindersuche projection (`view=slim`, #2097) carries. */
export interface BackendSlimStudent extends BackendNameStudent {
  school_class: string;
  current_location?: string | null;
  location_since?: string | null;
  current_room_color?: string | null;
  group_id?: number;
  group_name?: string;
  sick?: boolean;
  sick_since?: string;
  excused?: boolean;
  excused_since?: string;
  class_trip?: boolean;
  class_trip_since?: string;
  day_planning_status?: "comes_today" | "not_coming_today";
  day_planning_reason?: string;
  day_planning_label?: string;
  // Parent's note for a still-pending absence request covering today.
  // Informational only — the child stays "expected" until the OGS confirms.
  pending_excused_note?: string;
  // Hinweise der Lehrkraft für heute (#4795); nur mit Lesezugriff.
  teacher_hints?: TeacherHint[];
  // Übergabe nach dem Unterricht (#4794).
  class_handover?: ClassHandover;
  departure_modes?: DepartureMode[];
  departure_label?: string;
  departure_is_exception?: boolean;
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
  has_full_access?: boolean;
}

/** The full student row (`GET /api/students`, writes). */
export interface BackendStudent extends BackendSlimStudent {
  person_id: number;
  tag_id?: string;
  bus?: boolean;
  bus_days?: BusDays;
  pickup_days?: PickupDays;
  departure_days?: DepartureDays;
  allowed_departure_modes?: AllowedDepartureModes;
  // Letzter Betreuungstag (#2487); care_ended sagt, ob er schon vorbei ist,
  // care_exit_recorded, ob die Schule das Ende eingetragen hat.
  care_ends_on?: string;
  care_ended?: boolean;
  care_exit_recorded?: boolean;
  address_street?: string;
  address_city?: string;
  address_postal_code?: string;
  extra_info?: string;
  departure_companion_note?: string;
  birthday?: string;
  health_info?: string;
  pickup_status?: string;
  actual_pickup_note?: string;
  actual_arrival_by?: AttendanceActor | null;
  actual_pickup_by?: AttendanceActor | null;
  photo_consent_given_at?: string;
  photo_consent_given_by?: number;
  agb_accepted_at?: string;
  data_processing_accepted_at?: string;
  email_contact_accepted_at?: string;
  consents?: StudentConsent[];
  /** The write's own verdict on the "läuft mit" links (#1694). */
  companions_changed?: boolean;
  created_at: string;
  updated_at: string;
}

/** `GET /api/students/{id}?include_privacy_consent=true`. */
export interface BackendStudentDetail extends BackendStudent {
  has_full_access: boolean;
  has_write_access: boolean;
  /** Class-trip authority; sick/excused reports carry a separate capability. */
  has_absence_write_access: boolean;
  has_sick_excused_write_access: boolean;
  group_supervisors?: SupervisorContact[];
  attendance_log_enabled: boolean;
  feedback_enabled: boolean;
  privacy_consent: { accepted: boolean; data_retention_days: number };
}

function nameFields(row: BackendNameStudent) {
  const first = row.first_name ?? "";
  const last = row.last_name ?? "";
  return {
    id: String(row.id),
    name: `${first} ${last}`.trim(),
    first_name: row.first_name,
    second_name: row.last_name,
  };
}

/** The names-only row: no class and no location (#4996). */
function mapNameStudent(row: BackendNameStudent): Student {
  return { ...nameFields(row), school_class: "", current_location: "" };
}

/**
 * The Kindersuche projection, without synthesizing the omitted full-detail
 * fields: the route serializes the mapped value back to the browser.
 */
function mapSlimStudent(row: BackendSlimStudent): Student {
  return {
    ...nameFields(row),
    school_class: row.school_class,
    group_name: row.group_name,
    group_id: row.group_id ? String(row.group_id) : undefined,
    current_location: normalizeLocation(row.current_location),
    location_since: row.location_since ?? undefined,
    current_room_color: row.current_room_color ?? null,
    sick: row.sick,
    sick_since: row.sick_since,
    excused: row.excused,
    excused_since: row.excused_since,
    class_trip: row.class_trip,
    class_trip_since: row.class_trip_since,
    day_planning_status: row.day_planning_status,
    day_planning_reason: row.day_planning_reason,
    day_planning_label: row.day_planning_label,
    pending_excused_note: row.pending_excused_note,
    teacher_hints: row.teacher_hints,
    class_handover: row.class_handover,
    departure_modes: row.departure_modes,
    departure_label: row.departure_label,
    departure_is_exception: row.departure_is_exception,
    pickup_time: row.pickup_time,
    pickup_is_exception: row.pickup_is_exception,
    pickup_notes: row.pickup_notes,
    arrival_time: row.arrival_time,
    arrival_is_exception: row.arrival_is_exception,
    arrival_notes: row.arrival_notes,
    actual_arrival_time: row.actual_arrival_time,
    actual_pickup_time: row.actual_pickup_time,
    // Backend int64 ids → strings; ids above Number.MAX_SAFE_INTEGER must
    // never live as JS numbers.
    companion_student_ids: row.companion_student_ids?.map(String),
    photo_url: row.photo_url,
    photo_consent_given: row.photo_consent_given,
    has_full_access: row.has_full_access,
  };
}

/** The full row. */
export function mapStudentResponse(row: BackendStudent): Student {
  return {
    ...mapSlimStudent(row),
    sick: row.sick ?? false,
    excused: row.excused ?? false,
    class_trip: row.class_trip ?? false,
    studentId: row.tag_id,
    bus: busDaysHaveAny(row.bus_days),
    bus_days: normalizeBusDays(row.bus_days),
    pickup_days: normalizePickupDays(row.pickup_days),
    // A child whose plan predates allowed_departure_modes (#1694) or
    // departure_days (#1610) still carries only the older maps.
    allowed_departure_modes: row.allowed_departure_modes
      ? normalizeAllowedDepartureModes(row.allowed_departure_modes)
      : row.departure_days
        ? allowedDepartureModesFromDeparture(row.departure_days)
        : allowedDepartureModesFromLegacy(row.bus_days, row.pickup_days),
    departure_days: row.departure_days
      ? normalizeDepartureDays(row.departure_days)
      : departureDaysFromLegacy(row.bus_days, row.pickup_days),
    care_ends_on: row.care_ends_on,
    care_ended: row.care_ended ?? false,
    care_exit_recorded: row.care_exit_recorded ?? false,
    address_street: row.address_street,
    address_city: row.address_city,
    address_postal_code: row.address_postal_code,
    extra_info: row.extra_info,
    departure_companion_note: row.departure_companion_note,
    birthday: row.birthday,
    health_info: row.health_info,
    pickup_status: row.pickup_status,
    actual_pickup_note: row.actual_pickup_note,
    actual_arrival_by: parseAttendanceActor(row.actual_arrival_by),
    actual_pickup_by: parseAttendanceActor(row.actual_pickup_by),
    photo_consent_given_at: row.photo_consent_given_at,
    photo_consent_given_by: row.photo_consent_given_by,
    agb_accepted_at: row.agb_accepted_at,
    data_processing_accepted_at: row.data_processing_accepted_at,
    email_contact_accepted_at: row.email_contact_accepted_at,
    consents: row.consents,
  };
}

export type StudentView = "full" | "slim" | "names";

const LIST_ROW_MAPPERS: Record<StudentView, (row: never) => Student> = {
  full: mapStudentResponse,
  slim: mapSlimStudent,
  names: mapNameStudent,
};

/** Maps a list row of the requested projection. */
export function mapStudentListRow(
  row: BackendNameStudent,
  view: StudentView,
): Student {
  return LIST_ROW_MAPPERS[view](row as never);
}

/** The detail read: the full row plus access, feature flags and consent. */
export function mapStudentDetailResponse(row: BackendStudentDetail): Student {
  return {
    ...mapStudentResponse(row),
    has_full_access: row.has_full_access,
    has_write_access: row.has_write_access,
    has_absence_write_access: row.has_absence_write_access,
    has_sick_excused_write_access: row.has_sick_excused_write_access,
    group_supervisors: row.group_supervisors ?? [],
    attendance_log_enabled: row.attendance_log_enabled,
    feedback_enabled: row.feedback_enabled,
    privacy_consent_accepted: row.privacy_consent.accepted,
    data_retention_days: row.privacy_consent.data_retention_days,
  };
}

/**
 * Whether a student write can change the "läuft mit" links, for answers that
 * carry no verdict of the backend. A submitted list replaces the stored one, a
 * confirmed extension widens a linked child's plan, and every departure map
 * prepareStudentForBackend sends makes the backend trim links off weekdays
 * the new plan no longer allows.
 */
export function writeMayChangeCompanions(student: StudentUpdate): boolean {
  return (
    student.companions !== undefined ||
    (student.confirmed_companion_extensions?.length ?? 0) > 0 ||
    student.allowed_departure_modes !== undefined ||
    student.departure_days !== undefined ||
    student.bus_days !== undefined ||
    student.pickup_days !== undefined
  );
}

/** The backend body of a student write. Omitted keys leave stored values. */
export function prepareStudentForBackend(
  student: StudentUpdate,
): Partial<BackendStudent> & {
  companions?: StudentUpdate["companions"];
  companions_fingerprint?: string;
  extend_companion_plans?: boolean;
  confirmed_companion_extensions?: CompanionExtensionConfirmation[];
} {
  return {
    first_name: student.first_name,
    last_name: student.second_name,
    school_class: student.school_class,
    current_location: student.current_location
      ? normalizeLocation(student.current_location)
      : undefined,
    // bus_days is the single source of truth (#1582); the legacy bus boolean
    // is not sent. Each map only goes out when the caller set it, so partial
    // updates (sick/excused toggles) leave the stored plan untouched.
    bus_days:
      student.bus_days === undefined
        ? undefined
        : normalizeBusDays(student.bus_days),
    pickup_days:
      student.pickup_days === undefined
        ? undefined
        : normalizePickupDays(student.pickup_days),
    allowed_departure_modes:
      student.allowed_departure_modes === undefined
        ? undefined
        : normalizeAllowedDepartureModes(student.allowed_departure_modes),
    // When sent, the backend derives bus_days/pickup_days/pickup_status from
    // departure_days (#1610).
    departure_days:
      student.departure_days === undefined
        ? undefined
        : normalizeDepartureDays(student.departure_days),
    group_id: student.group_id
      ? Number.parseInt(String(student.group_id), 10)
      : undefined,
    address_street: student.address_street,
    address_city: student.address_city,
    address_postal_code: student.address_postal_code,
    extra_info: student.extra_info,
    departure_companion_note: student.departure_companion_note,
    // Laufgemeinschaft ("läuft mit"): an omitted key leaves the stored links
    // untouched, [] clears them. The id travels as the string the frontend
    // holds: the backend accepts a quoted decimal, and a JS number would round
    // an id beyond Number.MAX_SAFE_INTEGER to a DIFFERENT child's id.
    companions: student.companions?.map((companion) => ({
      companion_student_id: companion.companion_student_id,
      weekdays: companion.weekdays,
    })),
    // The fingerprint of the list the form LOADED. Sent without a list too:
    // the backend trims links the new departure plan no longer allows, so a
    // plan saved from a stale load deletes a fresh link exactly like a stale
    // list would, and the backend refuses that unless this claim matches.
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
    // The Go backend takes null or a valid date, not "".
    birthday: student.birthday?.trim() ? student.birthday : undefined,
    health_info: student.health_info,
    pickup_status: student.pickup_status,
    sick: student.sick,
    excused: student.excused,
    // The backend stamps a false→true change and clears photo and metadata on
    // true→false.
    photo_consent_given: student.photo_consent_given,
  };
}
