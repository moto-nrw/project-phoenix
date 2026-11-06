import { apiDelete, apiGet, apiPut } from "~/lib/api-helpers.server";
import { problemBody } from "~/lib/api-error";
import {
  ApiResponseError,
  isApiResponseError,
} from "~/lib/api-response-error.server";
import {
  createDeleteWithBodyHandler,
  createGetHandler,
  createPutHandler,
} from "~/lib/route-wrapper.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { createLogger } from "~/lib/logger";
import type { Student } from "~/lib/student-helpers";
import {
  mapStudentDetailResponse,
  mapStudentResponse,
  prepareStudentForBackend,
  writeMayChangeCompanions,
  type BackendStudent,
  type BackendStudentDetail,
} from "~/lib/student-wire";
import {
  rejectRetiredStudentContacts,
  type StudentUpdateBody,
} from "~/lib/student-request.server";
import {
  readConsentAfterWrite,
  sentConsentPair,
  writeConsent,
} from "~/lib/student-privacy-consent.server";
import { PRIVACY_CONSENT_SAVED_DETAIL } from "~/lib/student-update-errors";

const logger = createLogger({ component: "StudentDetailRoute" });

/**
 * Re-shapes a failed student PUT into the SAME wire error plus the
 * PRIVACY_CONSENT_SAVED_DETAIL key. The two writes are separate backend
 * endpoints and cannot share a transaction, so the request is genuinely a
 * partial success — and reporting it as a plain failure is what lets a user
 * cancel the retry believing their consent change never landed.
 *
 * Everything the client branches on has to survive untouched: the HTTP status
 * (`students.companions_changed` and the plan conflict are both 409s), the
 * `code`, and the top-level `conflicts` list the confirmation dialog is built
 * from. So the backend payload is parsed and re-emitted rather than replaced,
 * and only `details` grows a key.
 */
function markPrivacyConsentSaved(error: unknown): ApiResponseError {
  const original = error instanceof Error ? error : new Error(String(error));
  const status = isApiResponseError(original) ? original.status : 500;
  const parsed = isApiResponseError(original)
    ? original.body<Record<string, unknown>>()
    : null;
  // A body that is not an envelope (network error, plain-text upstream)
  // becomes one, with the original text as its diagnostic detail.
  const envelope: Record<string, unknown> =
    parsed !== null && typeof parsed === "object" && !Array.isArray(parsed)
      ? parsed
      : { ...problemBody(status, original.message) };

  const details = {
    ...((envelope.details as Record<string, unknown> | undefined) ?? {}),
    [PRIVACY_CONSENT_SAVED_DETAIL]: true,
  };

  return new ApiResponseError(
    status,
    JSON.stringify({ ...envelope, details }),
    {
      cause: original,
      contentType: "application/problem+json",
      retryAfter: isApiResponseError(original) ? original.retryAfter : null,
    },
  );
}

function logFailure(message: string, id: string, error: unknown): void {
  logger.error(message, {
    student_id: id,
    error: error instanceof Error ? error.message : String(error),
  });
}

/** The child with access flags, feature flags and privacy consent. */
export const GET = createGetHandler<Student>(async (request, token, params) => {
  const id = requirePathSegmentParam(params);
  try {
    // The tenant-scoped backend projection carries the consent: one read-budget
    // unit and no BFF consent waterfall.
    const response = await apiGet<{ data: BackendStudentDetail }>(
      `/api/students/${id}?include_privacy_consent=true`,
      token,
      { signal: request.signal },
    );
    return mapStudentDetailResponse(response.data);
  } catch (error) {
    logFailure("student fetch failed", id, error);
    throw error;
  }
});

/**
 * Updates the child and, when the form changed it, the privacy consent.
 *
 * Consent BEFORE the student PUT, deliberately. Both writes can fail, so one
 * of them has to run while the other is still undone — and the two are not
 * interchangeable. The consent PUT is a plain upsert of the two values in this
 * request: failing it here leaves the student untouched, and the client's
 * retry replays the identical, idempotent write. The student PUT is not
 * replayable that cheaply — it carries the companion fingerprint (#1694), so
 * once it commits, a retry of the same payload is refused as
 * `students.companions_changed` and costs the user a reload. Running it last
 * means a reported failure never hides a committed companion write; a student
 * PUT failing on top of a committed consent write says so
 * (markPrivacyConsentSaved) instead of reporting a blanket failure.
 *
 * The consequence for callers: a consent pair in the body is WRITTEN, refused
 * student PUT or not, so it may only be sent by a save that actually changes
 * it. An unrelated edit echoing the loaded values would, on a refused save,
 * overwrite somebody else's newer consent while the user is told nothing was
 * saved.
 */
export const PUT = createPutHandler<Student, StudentUpdateBody>(
  async (_request, body, token, params) => {
    const id = requirePathSegmentParam(params);
    try {
      rejectRetiredStudentContacts(body);
      const consent = sentConsentPair(body);

      if (consent) {
        try {
          await writeConsent(id, token, consent);
        } catch (consentError) {
          logFailure("privacy consent update failed", id, consentError);
          throw consentError;
        }
      }

      let response: { data: BackendStudent };
      try {
        response = await apiPut<{ data: BackendStudent }>(
          `/api/students/${id}`,
          token,
          prepareStudentForBackend(body),
        );
      } catch (studentError) {
        throw consent ? markPrivacyConsentSaved(studentError) : studentError;
      }

      // Whether the write changed the "läuft mit" links (#1694): the
      // backend's verdict, or what the payload can change when it sent none.
      // The client tells mounted companion views to refetch from it.
      return {
        ...mapStudentResponse(response.data),
        ...(await readConsentAfterWrite(id, token, consent)),
        companions_changed:
          response.data.companions_changed ?? writeMayChangeCompanions(body),
      };
    } catch (error) {
      logFailure("student update failed", id, error);
      throw error;
    }
  },
);

interface StudentDeletionRequest {
  expected_fingerprint?: string;
  confirmation_name?: string;
  reason?: string;
  acknowledged?: boolean;
}

export const DELETE = createDeleteWithBodyHandler<
  { success: true; message: string },
  StudentDeletionRequest
>(async (_request, body, token, params) => {
  const id = requirePathSegmentParam(params);
  try {
    // The generic database list deletes without a confirmation body; the
    // deletion dialog sends the fingerprint the backend checks.
    const response =
      Object.keys(body).length > 0
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
    logFailure("student delete failed", id, error);
    throw error;
  }
});
