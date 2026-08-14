// app/api/staff/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";
import { createLogger } from "~/lib/logger";
import type { WireID } from "~/lib/wire-id";
import { toIdString } from "~/lib/wire-id";

const logger = createLogger({ component: "StaffRoute" });

/**
 * Type definition for staff member response from backend
 */
interface BackendStaffResponse {
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
    account_id?: number;
    created_at: string;
    updated_at: string;
  };
  created_at: string;
  updated_at: string;
  was_present_today?: boolean;
  work_status?: string;
  absence_type?: string;
}

/**
 * Type definition for staff creation request
 *
 * person_id is accepted as a decimal string as well as a number, and is
 * forwarded as a string. It is a PostgreSQL bigint, and this route parses the
 * body and re-serializes it on the way to the backend, so a number has already
 * been through `JSON.parse` — exactly the step that rounds a value past 2^53
 * into a valid id for a different person (#2222). A string crosses that hop
 * intact, and the backend field takes either form.
 */
interface StaffCreateRequest {
  person_id: number | string;
  staff_notes?: string | null;
  is_teacher?: boolean;
  specialization?: string | null;
  role?: string | null;
  qualifications?: string | null;
}

/**
 * Resolves the submitted person id to the decimal string that goes on the wire,
 * or null when it is not a positive integer id.
 *
 * The value stays a string for the whole hop. Person ids are PostgreSQL
 * bigints, and this route parses the body and re-serializes it — a number would
 * pass through JSON.parse, which rounds anything above Number.MAX_SAFE_INTEGER
 * into a valid id for a *different* person. The backend accepts the string, so
 * there is no point in the chain where the digits have to fit in a JS number,
 * and no id this route has to refuse for being too large.
 *
 * A number is still accepted for callers that send one, and is only safe to
 * take when it is exactly representable — past 2^53 it has already been rounded
 * before this function ever sees it, and nothing here can recover it.
 */
function safePersonId(value: number | string | undefined): string | null {
  if (typeof value === "string") {
    const digits = value.trim();
    // No leading zeros, so one id has exactly one spelling on the wire.
    if (!/^[1-9]\d*$/.test(digits)) {
      return null;
    }
    return digits;
  }
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value <= 0) {
    return null;
  }
  return String(value);
}

/**
 * Normalizes an optional string field for API submission.
 * - If original is undefined, returns undefined (field not provided)
 * - If trimmed value is empty, returns empty string (field explicitly cleared)
 * - Otherwise returns the trimmed value
 */
function normalizeOptionalString(
  original: string | null | undefined,
  trimmed: string | undefined,
): string | undefined {
  if (original === undefined) {
    return undefined;
  }
  return trimmed === "" ? "" : trimmed;
}

/**
 * Type definition for API response format
 */
interface ApiStaffResponse {
  status: string;
  data: BackendStaffResponse[];
}

/**
 * Maps a backend staff response to the frontend representation
 * Always uses staff.id as unique identifier (NEVER teacher_id to avoid duplicates)
 */
function mapBackendStaff(staff: BackendStaffResponse) {
  return {
    id: String(staff.id),
    name: staff.person
      ? `${staff.person.first_name} ${staff.person.last_name}`
      : "",
    firstName: staff.person?.first_name ?? "",
    lastName: staff.person?.last_name ?? "",
    email: staff.person?.email ?? null,
    avatar: staff.person?.avatar ?? null,
    account_id: staff.person?.account_id,
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
  };
}

/**
 * Handler for GET /api/staff
 * Returns a list of staff members, optionally filtered by query parameters
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    // `strict=1` is an internal opt-out of the graceful empty-list fallback
    // below: with it a backend failure propagates as a real error response so
    // callers can distinguish a failed fetch from a genuinely empty staff list
    // (#1840). It is NOT a backend filter, so it is stripped before forwarding.
    const strict = request.nextUrl.searchParams.get("strict") === "1";

    // Build URL with any query parameters
    const queryParams = new URLSearchParams();
    request.nextUrl.searchParams.forEach((value, key) => {
      if (key === "strict") return;
      queryParams.append(key, value);
    });

    const endpoint = `/api/staff${queryParams.toString() ? "?" + queryParams.toString() : ""}`;

    logger.debug("requesting staff from backend", { endpoint });

    try {
      // Fetch staff from backend API
      const response = await apiGet<BackendStaffResponse[] | ApiStaffResponse>(
        endpoint,
        token,
      );

      // Handle null or undefined response
      if (!response) {
        logger.warn("API returned null response for staff");
        return [];
      }

      logger.debug("staff API response received");

      // Check if the response is already an array (common pattern)
      if (Array.isArray(response)) {
        return response.map(mapBackendStaff);
      }

      // Check for nested data structure
      if ("data" in response && Array.isArray(response.data)) {
        return response.data.map(mapBackendStaff);
      }

      // If the response doesn't have the expected structure, return an empty array
      logger.warn("staff API response has unexpected structure");
      return [];
    } catch (error) {
      logger.error("staff fetch failed", {
        error: error instanceof Error ? error.message : String(error),
      });
      // In strict mode surface the failure to the caller (createGetHandler maps
      // it to a non-2xx response); the lenient default returns [] so existing
      // consumers keep degrading gracefully.
      if (strict) throw error;
      return [];
    }
  },
);

// Define the Teacher response type
interface TeacherResponse {
  id: string;
  name: string;
  firstName: string;
  lastName: string;
  specialization?: string | null;
  role: string | null;
  qualifications: string | null;
  tag_id: string | null;
  staff_notes: string | null;
  created_at: string;
  updated_at: string;
  staff_id?: string;
  teacher_id?: string;
}

/**
 * Handler for POST /api/staff
 * Creates a new staff member (potentially a teacher)
 */
export const POST = createPostHandler<TeacherResponse, StaffCreateRequest>(
  async (_request: NextRequest, body: StaffCreateRequest, token: string) => {
    // Validate required fields
    const personId = safePersonId(body.person_id);
    if (!personId) {
      throw new Error(
        "Missing required field: person_id must be a positive integer id",
      );
    }

    try {
      const trimmedNotes = body.staff_notes?.trim();
      const trimmedSpecialization = body.specialization?.trim();
      const trimmedRole = body.role?.trim();
      const trimmedQualifications = body.qualifications?.trim();

      const normalizedBody = {
        ...body,
        person_id: personId,
        staff_notes: normalizeOptionalString(body.staff_notes, trimmedNotes),
        specialization: normalizeOptionalString(
          body.specialization,
          trimmedSpecialization,
        ),
        role: normalizeOptionalString(body.role, trimmedRole),
        qualifications: normalizeOptionalString(
          body.qualifications,
          trimmedQualifications,
        ),
      };

      // Create the staff member via the API
      const response = await apiPost<BackendStaffResponse>(
        "/api/staff",
        token,
        normalizedBody,
      );

      return mapBackendStaff(response);
    } catch (error) {
      // Check for permission errors (403 Forbidden)
      if (error instanceof Error && error.message.includes("403")) {
        logger.error("permission denied when creating staff", {
          error: error instanceof Error ? error.message : String(error),
        });
        throw new Error(
          "Permission denied: You need the 'users:create' permission to create staff members.",
          { cause: error },
        );
      }

      // Check for validation errors
      if (error instanceof Error && error.message.includes("400")) {
        const errorMessage = error.message;
        logger.error("validation error when creating staff", {
          error: errorMessage,
        });

        // Extract specific error message if possible
        if (errorMessage.includes("person not found")) {
          throw new Error("Person not found with the specified ID", {
            cause: error,
          });
        }
        // No additional specialization validation anymore
      }

      // Re-throw other errors
      throw error;
    }
  },
);
