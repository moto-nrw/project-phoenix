// Wire mapping shared by the /api/staff route handlers.
import type { WireID } from "~/lib/wire-id";
import { toIdString, toOptionalIdString } from "~/lib/wire-id";

/**
 * Type definition for staff member response from backend
 */
export interface BackendStaffResponse {
  id: number;
  // Decimal string since #2222 — person_id is a bigint the staff screens send
  // back to identify the person they edit, so it must not pass through a JS
  // number. A number is still accepted (older server, test fixture).
  person_id: WireID;
  staff_notes?: string;
  is_teacher: boolean;
  teacher_id?: number;
  specialization?: string;
  role?: string;
  qualifications?: string;
  account_role?: string;
  person?: {
    id: number;
    first_name: string;
    last_name: string;
    email?: string;
    avatar?: string;
    tag_id?: string;
    account_id?: WireID;
    created_at: string;
    updated_at: string;
  };
  created_at: string;
  updated_at: string;
  was_present_today?: boolean;
  work_status?: string;
  absence_type?: string;
  // Externe Betreuungskraft ohne Konto (#3823).
  is_external?: boolean;
  external_organization?: string;
}

/**
 * Maps a backend staff response to the frontend representation
 * Always uses staff.id as unique identifier (NEVER teacher_id to avoid duplicates)
 */
export function mapBackendStaff(staff: BackendStaffResponse) {
  return {
    id: String(staff.id),
    name: staff.person
      ? `${staff.person.first_name} ${staff.person.last_name}`
      : "",
    firstName: staff.person?.first_name ?? "",
    lastName: staff.person?.last_name ?? "",
    email: staff.person?.email ?? null,
    avatar: staff.person?.avatar ?? null,
    account_id: toOptionalIdString(staff.person?.account_id),
    specialization: staff.specialization ?? null,
    role: staff.role ?? null,
    qualifications: staff.qualifications ?? null,
    account_role: staff.account_role ?? null,
    tag_id: staff.person?.tag_id ?? null,
    staff_notes: staff.staff_notes ?? null,
    created_at: staff.created_at,
    updated_at: staff.updated_at,
    staff_id: String(staff.id),
    teacher_id: staff.teacher_id ? String(staff.teacher_id) : undefined,
    // Ob ein Betreuungsprofil (users.teachers) existiert — steuert u. a.,
    // ob das Position-Feld im Edit-Formular angeboten wird (für Lehrkräfte
    // gibt es keinen Speicherort dafür).
    is_teacher: staff.is_teacher,
    // Decimal string, exactly as it arrived: the edit screens send this id back
    // to address the person record, and a JS number would round it past 2^53
    // onto a different person (#2222).
    person_id: toIdString(staff.person_id),
    was_present_today: staff.was_present_today,
    work_status: staff.work_status,
    absence_type: staff.absence_type,
    is_external: staff.is_external ?? false,
    external_organization: staff.external_organization ?? null,
  };
}
