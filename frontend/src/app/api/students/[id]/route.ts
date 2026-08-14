// app/api/students/[id]/route.ts
import type { NextRequest } from "next/server";
import { apiGet, apiPut, apiDelete } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPutHandler,
  createDeleteWithBodyHandler,
} from "~/lib/route-wrapper.server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentDetailRoute" });
import type { BackendStudent, Student } from "~/lib/student-helpers";
import {
  mapStudentResponse,
  prepareStudentForBackend,
} from "~/lib/student-helpers";
import {
  fetchPrivacyConsent,
  updatePrivacyConsent,
} from "~/lib/student-privacy-helpers";

/**
 * Detail key the student PUT sets when it failed AFTER the consent write of the
 * same request had already committed. The two writes are separate backend
 * endpoints and cannot share a transaction, so the request is genuinely a
 * partial success — and reporting it as a plain failure is what lets a user
 * cancel the retry believing their consent change never landed. The browser
 * reads it back out via isPrivacyConsentSaved (~/lib/api) and says so.
 */
const PRIVACY_CONSENT_SAVED_DETAIL = "privacy_consent_saved";

/**
 * Re-shapes a failed student PUT into the SAME wire error plus that detail.
 *
 * Everything the client branches on has to survive untouched: the HTTP status
 * (`companions_changed` and the plan conflict are both 409s), the `code`, and
 * the top-level `conflicts` list the confirmation dialog is built from. So the
 * backend payload is parsed and re-emitted rather than replaced, and only
 * `details` grows a key.
 */
function markPrivacyConsentSaved(error: unknown): Error {
  const original = error instanceof Error ? error : new Error(String(error));
  const status = /API error[:\s(]+(\d{3})/.exec(original.message)?.[1] ?? "500";

  const jsonStart = original.message.indexOf("{");
  let payload: Record<string, unknown> = {};
  if (jsonStart !== -1) {
    try {
      payload = JSON.parse(original.message.substring(jsonStart)) as Record<
        string,
        unknown
      >;
    } catch {
      // Not a JSON body (network error, plain-text upstream). The message below
      // then carries the whole original text, which is what handleApiError
      // would have shown anyway.
      payload = {};
    }
  }

  const details = {
    ...((payload.details as Record<string, unknown> | undefined) ?? {}),
    [PRIVACY_CONSENT_SAVED_DETAIL]: true,
  };

  return new Error(
    `API error (${status}): ${JSON.stringify({
      ...payload,
      error: payload.error ?? payload.message ?? original.message,
      details,
    })}`,
    { cause: original },
  );
}

/**
 * Type definition for API response format
 * Backend wraps response in { status: "success", data: {...}, message: "..." }
 */
interface ApiStudentResponse {
  status: string;
  data: StudentResponseFromBackend;
  message: string;
}

/**
 * Type definition for student response from backend
 */
interface StudentResponseFromBackend {
  id: number;
  person_id: number;
  first_name: string;
  last_name: string;
  tag_id?: string;
  school_class: string;
  current_location?: string | null;
  // Hex of the student's current room when set (Issue #1324). The proxy
  // forwards it untouched; mapStudentResponse threads it into the Student
  // type that powers LocationBadge.
  current_room_color?: string | null;
  guardian_name: string;
  guardian_contact: string;
  guardian_email?: string;
  guardian_phone?: string;
  group_id?: number;
  created_at: string;
  updated_at: string;
}

/**
 * Handler for GET /api/students/[id]
 * Returns a single student by ID with privacy consent data
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Student ID is required");
    }

    try {
      // Fetch student from backend API
      // Using unknown type and will validate structure
      const response = await apiGet<unknown>(`/api/students/${id}`, token);

      // Type guard to check response structure
      if (!response || typeof response !== "object" || !("data" in response)) {
        logger.warn("API returned invalid response for student", {
          student_id: id,
        });
        throw new Error("Student not found");
      }

      // Extract student data from the backend response
      // Backend sends: { status: "success", data: { ...student, has_full_access, group_supervisors } }

      // Define type for backend student data including access control fields
      interface BackendStudentData {
        last_name?: string;
        name?: string;
        first_name?: string;
        has_full_access?: boolean;
        has_write_access?: boolean;
        has_absence_write_access?: boolean;
        group_supervisors?: Array<{
          id: number;
          first_name: string;
          last_name: string;
          email?: string;
          role: string;
        }>;
        [key: string]: unknown;
      }

      const backendResponse = response as {
        data: BackendStudentData;
      };

      // Map the backend response to frontend format
      const studentData = backendResponse.data;

      // Extract access control fields from response data
      const hasFullAccess = studentData.has_full_access ?? false;
      const hasWriteAccess = studentData.has_write_access ?? false;
      // Absence actions (krank / entschuldigt / Klassenfahrt) have their own
      // gate: in a school without fixed groups staff may report absences for
      // children whose Stammdaten they cannot edit (#2232). Falls back to the
      // Stammdaten flag so an older backend keeps the previous behavior.
      const hasAbsenceWriteAccess =
        studentData.has_absence_write_access ?? hasWriteAccess;
      const groupSupervisors = studentData.group_supervisors ?? [];
      const attendanceLogEnabled =
        (studentData.attendance_log_enabled as boolean) ?? false;
      const feedbackEnabled =
        (studentData.feedback_enabled as boolean) ?? false;

      // Check if we need to extract last_name from the name field
      if (!studentData.last_name && studentData.name) {
        // Split the name to extract first and last name
        const nameParts = studentData.name.split(" ");
        if (nameParts.length > 1) {
          // If first_name matches the first part, the rest is the last name
          if (studentData.first_name === nameParts[0]) {
            studentData.last_name = nameParts.slice(1).join(" ");
          }
        }
      }

      const mappedStudent = mapStudentResponse(
        studentData as unknown as BackendStudent,
      );

      // Fetch privacy consent data using helper
      const consentData = await fetchPrivacyConsent(id, apiGet, token);

      return {
        ...mappedStudent,
        ...consentData,
        has_full_access: hasFullAccess,
        has_write_access: hasWriteAccess,
        has_absence_write_access: hasAbsenceWriteAccess,
        group_supervisors: groupSupervisors,
        attendance_log_enabled: attendanceLogEnabled,
        feedback_enabled: feedbackEnabled,
      };
    } catch (error) {
      logger.error("student fetch failed", {
        student_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
);

/**
 * Handler for PUT /api/students/[id]
 * Updates an existing student
 */
export const PUT = createPutHandler<
  Student,
  Partial<Student> & {
    privacy_consent_accepted?: boolean;
    data_retention_days?: number;
  }
>(
  async (
    _request: NextRequest,
    body: Partial<Student> & {
      privacy_consent_accepted?: boolean;
      data_retention_days?: number;
    },
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Student ID is required");
    }

    try {
      // Extract privacy consent fields
      const { privacy_consent_accepted, data_retention_days, ...studentData } =
        body;

      // Consent BEFORE the student PUT, deliberately. Both writes can fail, so
      // one of them has to run while the other is still undone — and the two
      // are not interchangeable. The consent PUT is a plain upsert of the two
      // values in this request: failing it here leaves the student untouched,
      // and the client's retry replays the identical, idempotent write. The
      // student PUT is not replayable that cheaply — it carries the companion
      // fingerprint (#1694), so once it commits, a retry of the same payload is
      // refused as `companions_changed` and costs the user a reload. Running it
      // last means a reported failure never hides a committed companion write.
      //
      // What that ordering cannot do is make the pair atomic: they are two
      // backend endpoints with two transactions. So the other half of the deal
      // is that a student PUT failing on top of a COMMITTED consent write says
      // so (markPrivacyConsentSaved below) instead of reporting a blanket
      // failure for a request that half succeeded.
      //
      // The consequence for callers: a consent value in the body is WRITTEN,
      // refused student PUT or not. It may therefore only be sent by a save
      // that actually changes it — an unrelated edit carrying the values the
      // form loaded would, on a refused save, leave somebody else's newer
      // consent overwritten by a stale echo while the user is told nothing was
      // saved. Both clients omit the untouched pair for exactly that reason
      // (student-stammdaten-tab.tsx, ui/database/database-form.tsx); an omitted
      // pair skips the write below. Send the pair or neither — this is a full
      // upsert, so a lone field resets the other one to its default.
      let consentSaved = false;
      if (
        privacy_consent_accepted !== undefined ||
        data_retention_days !== undefined
      ) {
        try {
          await updatePrivacyConsent(
            id,
            apiPut,
            token,
            privacy_consent_accepted,
            data_retention_days,
            "PUT Student",
          );
          consentSaved = true;
        } catch (consentError) {
          logger.error("privacy consent update failed", {
            student_id: id,
            error:
              consentError instanceof Error
                ? consentError.message
                : String(consentError),
          });
          throw new Error(
            "Datenschutzeinstellungen konnten nicht aktualisiert werden.",
            { cause: consentError },
          );
        }
      }

      // Transform frontend format to backend format
      const backendData = prepareStudentForBackend(studentData);

      // Call backend API to update student. A failure here is the partial
      // success the comment above describes whenever the consent write already
      // landed: the request is rejected, but the stored consent is NOT the one
      // the user saw before saving. Stamping the error is what lets the form
      // say that instead of "Fehler beim Speichern" — the user would otherwise
      // walk away from a consent change they never confirmed twice.
      let response: ApiStudentResponse;
      try {
        response = await apiPut<ApiStudentResponse>(
          `/api/students/${id}`,
          token,
          backendData,
        );
      } catch (studentError) {
        throw consentSaved
          ? markPrivacyConsentSaved(studentError)
          : studentError;
      }

      // Handle null or undefined response
      if (!response?.data) {
        throw new Error("Invalid response from backend");
      }

      // Map the response to frontend format
      const mappedStudent = mapStudentResponse(response.data as BackendStudent);

      // Read back the stored consent for the response — but never let that read
      // fail the request: everything is committed at this point, and turning an
      // unreadable GET into a 500 would send the client down its generic save
      // failure path for a write that succeeded (and, with companions in the
      // payload, into a `companions_changed` conflict on the retry). We know
      // what was just written, so fall back to it; with nothing written, the
      // keys stay out and the client keeps the consent state it already holds.
      const consentData = await fetchPrivacyConsent(id, apiGet, token).catch(
        (consentError: unknown) => {
          logger.warn("privacy consent read-back failed after update", {
            student_id: id,
            error:
              consentError instanceof Error
                ? consentError.message
                : String(consentError),
          });
          if (
            privacy_consent_accepted === undefined &&
            data_retention_days === undefined
          ) {
            return {};
          }
          return {
            ...(privacy_consent_accepted === undefined
              ? {}
              : { privacy_consent_accepted }),
            ...(data_retention_days === undefined
              ? {}
              : { data_retention_days }),
          };
        },
      );

      // The write's own verdict on the "läuft mit" links (#1694). Not part of
      // the student, so mapStudentResponse drops it — but the client decides
      // from it whether to tell mounted companion views to refetch, and doing
      // that for a save that changed nothing costs an open editor its draft.
      // Absent when the backend sent none; the client then falls back to its
      // conservative payload heuristic.
      const companionsChanged = (
        response.data as { companions_changed?: boolean }
      ).companions_changed;

      return {
        ...mappedStudent,
        ...consentData,
        ...(companionsChanged === undefined
          ? {}
          : { companions_changed: companionsChanged }),
      };
    } catch (error) {
      logger.error("student update failed", {
        student_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
);

/**
 * Handler for PATCH /api/students/[id]
 * Partially updates a student (same as PUT but for PATCH requests)
 */
export const PATCH = PUT;

/**
 * Handler for DELETE /api/students/[id]
 * Deletes a student
 */
interface StudentDeletionRequest {
  expected_fingerprint?: string;
  confirmation_name?: string;
  reason?: string;
  acknowledged?: boolean;
}

export const DELETE = createDeleteWithBodyHandler<
  { success: true; message: string },
  StudentDeletionRequest
>(
  async (
    _request: NextRequest,
    body: StudentDeletionRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.id as string;

    if (!id) {
      throw new Error("Student ID is required");
    }

    try {
      // Call backend API to delete student
      const hasConfirmationBody = Object.keys(body).length > 0;
      const response = hasConfirmationBody
        ? await apiDelete<{ message: string }, StudentDeletionRequest>(
            `/api/students/${id}`,
            token,
            body,
          )
        : await apiDelete<{ message: string }>(`/api/students/${id}`, token);

      return {
        success: true,
        message: response?.message ?? "Student deleted successfully",
      };
    } catch (error) {
      logger.error("student delete failed", {
        student_id: id,
        error: error instanceof Error ? error.message : String(error),
      });
      throw error;
    }
  },
);
