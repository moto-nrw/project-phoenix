// app/api/staff/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPutHandler,
  createDeleteHandler,
} from "~/lib/route-wrapper.server";
import { createLogger } from "~/lib/logger";
import type { WireID } from "~/lib/wire-id";
import { toIdString } from "~/lib/wire-id";

const logger = createLogger({ component: "StaffDetailRoute" });

/**
 * Type definition for staff member response from backend
 */
interface BackendStaffResponse {
  id: number;
  // Decimal string since #2222 — see the note on the mapping below.
  person_id: WireID;
  staff_notes?: string;
  is_teacher: boolean;
  teacher_id?: number;
  specialization?: string;
  role?: string;
  qualifications?: string;
  employment_type?: string;
  account_role?: string;
  was_present_today?: boolean;
  work_status?: string;
  absence_type?: string;
  person?: {
    id: number;
    first_name: string;
    last_name: string;
    email?: string;
    tag_id?: string;
    account_id?: number;
    created_at: string;
    updated_at: string;
  };
  created_at: string;
  updated_at: string;
}

/**
 * Type definition for staff update request
 *
 * person_id arrives as the decimal string the edit screen loaded and is
 * forwarded unchanged; the backend field takes a number or a string
 * (common.JSONID). Narrowing it to a JS number here would undo the precaution,
 * because this route parses the body and re-serializes it (#2222).
 */
interface StaffUpdateRequest {
  person_id?: WireID;
  staff_notes?: string | null;
  is_teacher?: boolean;
  specialization?: string | null;
  role?: string;
  qualifications?: string;
}

/**
 * Type definition for API response format
 */
interface ApiStaffResponse {
  status: string;
  data: BackendStaffResponse;
}

/**
 * Handler for GET /api/staff/[id]
 * Returns a single staff member by ID
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Staff ID is required");
    }

    try {
      // Fetch staff member from backend API
      const response = await apiGet<ApiStaffResponse>(
        `/api/staff/${id}`,
        token,
      );

      // Handle null or undefined response
      if (!response?.data) {
        logger.warn("API returned null response for staff member", {
          staff_id: id,
        });
        throw new Error("Staff member not found");
      }

      const staff = response.data;

      // Map the response data to match the Teacher interface from teacher-api.ts
      return {
        id: String(staff.id), // This should be the staff ID since that's what we use for the API
        name: staff.person
          ? `${staff.person.first_name} ${staff.person.last_name}`
          : "",
        first_name: staff.person?.first_name ?? "",
        last_name: staff.person?.last_name ?? "",
        email: staff.person?.email ?? undefined, // Include email from person object
        specialization: staff.specialization ?? null,
        role: staff.role ?? null,
        qualifications: staff.qualifications ?? null,
        tag_id: staff.person?.tag_id ?? null,
        staff_notes: staff.staff_notes ?? null,
        created_at: staff.created_at,
        updated_at: staff.updated_at,
        // Include person_id for updates — as the decimal string it arrived as,
        // so a bigint id still addresses the intended person (#2222).
        person_id: toIdString(staff.person_id),
        // Include account_id from person object
        account_id: staff.person?.account_id,
        // Include both IDs for debugging
        staff_id: String(staff.id),
        teacher_id: staff.teacher_id ? String(staff.teacher_id) : undefined,
        // Ob ein Betreuungsprofil (users.teachers) existiert — steuert u. a.,
        // ob das Position-Feld im Edit-Formular angeboten wird (für
        // Lehrkräfte gibt es keinen Speicherort dafür).
        is_teacher: staff.is_teacher,
        // Include employment type
        employment_type: staff.employment_type ?? null,
        // Forward presence/work-status/absence so the location badge on the
        // detail page matches the staff list. Without these the client falls
        // back to "Abwesend" even when the staff is checked in.
        account_role: staff.account_role ?? null,
        was_present_today: staff.was_present_today ?? false,
        work_status: staff.work_status ?? null,
        absence_type: staff.absence_type ?? null,
        // Include person object if available
        person: staff.person,
      };
    } catch (error) {
      logger.error("staff member fetch failed", {
        staff_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
);

// Define the Teacher response type
interface TeacherResponse {
  id: string;
  name: string;
  first_name: string;
  last_name: string;
  email?: string;
  specialization?: string | null;
  role: string | null;
  qualifications: string | null;
  tag_id: string | null;
  staff_notes: string | null;
  created_at: string;
  updated_at: string;
  person_id?: string;
  account_id?: number;
  staff_id?: string;
  teacher_id?: string;
  person?: {
    id: number;
    first_name: string;
    last_name: string;
    email?: string;
    tag_id?: string;
    account_id?: number;
    created_at: string;
    updated_at: string;
  };
}

/** Normalize string fields by trimming whitespace (only if defined) */
function normalizeStaffBody(body: StaffUpdateRequest): StaffUpdateRequest {
  const result: StaffUpdateRequest = { ...body };

  if (body.specialization !== undefined) {
    result.specialization = body.specialization?.trim();
  }
  if (body.role !== undefined) {
    result.role = body.role?.trim();
  }
  if (body.qualifications !== undefined) {
    result.qualifications = body.qualifications?.trim();
  }
  if (body.staff_notes !== undefined) {
    result.staff_notes = body.staff_notes?.trim();
  }

  return result;
}

/** Map backend staff response to frontend TeacherResponse */
function mapStaffResponse(response: BackendStaffResponse): TeacherResponse {
  return {
    id: String(response.id),
    name: response.person
      ? `${response.person.first_name} ${response.person.last_name}`
      : "",
    first_name: response.person?.first_name ?? "",
    last_name: response.person?.last_name ?? "",
    email: response.person?.email ?? undefined,
    specialization: response.specialization ?? null,
    role: response.role ?? null,
    qualifications: response.qualifications ?? null,
    tag_id: response.person?.tag_id ?? null,
    staff_notes: response.staff_notes ?? null,
    created_at: response.created_at,
    updated_at: response.updated_at,
    account_id: response.person?.account_id,
    person_id: toIdString(response.person_id),
    person: response.person,
  };
}

/**
 * Handler for PUT /api/staff/[id]
 * Updates an existing staff member
 */
export const PUT = createPutHandler<TeacherResponse, StaffUpdateRequest>(
  async (
    _request: NextRequest,
    body: StaffUpdateRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;
    if (!id) throw new Error("Staff ID is required");

    try {
      const response = await apiPut<BackendStaffResponse>(
        `/api/staff/${id}`,
        token,
        normalizeStaffBody(body),
      );
      return mapStaffResponse(response);
    } catch (error) {
      if (!(error instanceof Error)) throw error;

      if (error.message.includes("403")) {
        throw new Error(
          "Sie dürfen Mitarbeiterdaten nicht ändern. Bitte wenden Sie sich an Ihre OGS-Leitung.",
          { cause: error },
        );
      }
      if (
        error.message.includes("400") &&
        error.message.includes("staff member not found")
      ) {
        throw new Error("Staff member not found", { cause: error });
      }
      throw error;
    }
  },
);

/**
 * Handler for DELETE /api/staff/[id]
 * Deletes a staff member
 */
export const DELETE = createDeleteHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Staff ID is required");
    }

    try {
      // Delete the staff member via the API
      await apiDelete(`/api/staff/${id}`, token);

      // Return null to indicate success with no content
      return null;
    } catch (error) {
      // Check for permission errors (403 Forbidden)
      if (error instanceof Error && error.message.includes("403")) {
        logger.error("permission denied when deleting staff", {
          staff_id: id,
          error: error instanceof Error ? error.message : String(error),
        });
        throw new Error(
          "Permission denied: You need the 'users:delete' permission to delete staff members.",
          { cause: error },
        );
      }

      // Check for not found errors
      if (error instanceof Error && error.message.includes("404")) {
        throw new Error("Staff member not found", { cause: error });
      }

      // Re-throw other errors
      throw error;
    }
  },
);
