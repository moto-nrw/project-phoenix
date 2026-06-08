// lib/student-helpers.ts
// Type definitions and helper functions for students

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

export function formatBusDays(value?: BusDays | null): string {
  const labels = BUS_WEEKDAYS.filter((day) => Boolean(value?.[day.key])).map(
    (day) => day.label.slice(0, 2),
  );
  return labels.length > 0 ? labels.join(", ") : "Keine Bus-Tage";
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
  sick?: boolean;
  sick_since?: string;
  excused?: boolean;
  excused_since?: string;
  day_planning_status?: "comes_today" | "not_coming_today";
  day_planning_reason?: string;
  day_planning_label?: string;
  guardian_name?: string; // Optional: Legacy field, use guardian_profiles instead
  guardian_contact?: string; // Optional: Legacy field, use guardian_profiles instead
  guardian_email?: string;
  guardian_phone?: string;
  group_id?: number;
  group_name?: string;
  scheduled_checkout?: ScheduledCheckoutInfo;
  extra_info?: string;
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
  created_at: string;
  updated_at: string;
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
  // Sickness status (only visible to supervisors/admins)
  sick?: boolean;
  sick_since?: string;
  // Excused status (kind is not attending the OGS today, only visible to supervisors/admins)
  excused?: boolean;
  excused_since?: string;
  day_planning_status?: "comes_today" | "not_coming_today";
  day_planning_reason?: string;
  day_planning_label?: string;
  name_lg?: string;
  contact_lg?: string;
  guardian_email?: string;
  guardian_phone?: string;
  custom_users_id?: string;
  // Privacy consent data (fetched separately)
  privacy_consent?: PrivacyConsent;
  // Privacy consent fields for form handling
  privacy_consent_accepted?: boolean;
  data_retention_days?: number;
  // Additional fields for access control
  has_full_access?: boolean;
  has_write_access?: boolean;
  group_supervisors?: SupervisorContact[];
  // Feature flag: tenant has attendance log enabled
  attendance_log_enabled?: boolean;
  // Extra information visible only to supervisors
  extra_info?: string;
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
    bus: backendStudent.bus ?? false, // Administrative permission flag (Buskind)
    bus_days: normalizeBusDays(backendStudent.bus_days),
    sick: backendStudent.sick ?? false, // Sickness status
    sick_since: backendStudent.sick_since,
    excused: backendStudent.excused ?? false, // Excused from attending today
    excused_since: backendStudent.excused_since,
    day_planning_status: backendStudent.day_planning_status,
    day_planning_reason: backendStudent.day_planning_reason,
    day_planning_label: backendStudent.day_planning_label,
    name_lg: backendStudent.guardian_name ?? undefined,
    contact_lg: backendStudent.guardian_contact ?? undefined,
    guardian_email: backendStudent.guardian_email,
    guardian_phone: backendStudent.guardian_phone,
    custom_users_id: undefined, // Not provided by backend
    extra_info: backendStudent.extra_info,
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
    extra_info?: string;
    birthday?: string;
    health_info?: string;
    supervisor_notes?: string;
    pickup_status?: string;
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
    // Only send bus when explicitly provided so partial updates (e.g. sick/excused
    // toggles) don't clobber the persisted Buskind flag. The backend field is
    // *bool with omitempty — omitting the key leaves the DB value untouched.
    bus: student.bus,
    bus_days:
      student.bus_days !== undefined
        ? normalizeBusDays(student.bus_days)
        : undefined,
    // REMOVED: guardian_name and guardian_contact - deprecated fields
    // Use guardian_profiles system instead
    group_id: student.group_id
      ? Number.parseInt(student.group_id, 10)
      : undefined,
    tag_id: student.tag_id,
    guardian_email: student.guardian_email,
    guardian_phone: student.guardian_phone,
    extra_info: student.extra_info,
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
  extra_info?: string;
  birthday?: string;
  health_info?: string;
  supervisor_notes?: string;
  pickup_status?: string;
  bus?: boolean;
  bus_days?: BusDays;
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
  group_id?: number;
  extra_info?: string;
  birthday?: string;
  health_info?: string;
  supervisor_notes?: string;
  pickup_status?: string;
  bus?: boolean;
  bus_days?: BusDays;
  sick?: boolean;
  sick_reason?: string;
  excused?: boolean;
  photo_consent_given?: boolean;
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
  { source: "extra_info", target: "extra_info" },
  { source: "birthday", target: "birthday" },
  { source: "health_info", target: "health_info" },
  { source: "supervisor_notes", target: "supervisor_notes" },
  { source: "pickup_status", target: "pickup_status" },
  { source: "bus", target: "bus" },
  { source: "bus_days", target: "bus_days" },
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
