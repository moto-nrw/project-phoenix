import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import { isApiResponseError } from "~/lib/api-response-error.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";
import { createLogger } from "~/lib/logger";
import type { Student, StudentPage } from "~/lib/student-helpers";
import {
  mapStudentListRow,
  mapStudentResponse,
  type BackendNameStudent,
  type BackendStudent,
  type StudentView,
} from "~/lib/student-wire";
import {
  buildStudentCreateRequest,
  type StudentCreateBody,
} from "~/lib/student-request.server";
import {
  readConsentAfterWrite,
  sentConsentPair,
  writeConsent,
} from "~/lib/student-privacy-consent.server";

const logger = createLogger({ component: "StudentsRoute" });

// Callers that don't paginate (Kindersuche, database list views) get every row
// in one trip; an explicit page_size is honored up to the cap, so an
// authenticated caller can't force an arbitrarily large query. The room-detail
// modal sends 200 and drives its own overflow notice from it (#1374).
const MAX_PAGE_SIZE = 1000;

function pageSize(raw: string | null): number {
  const parsed = raw === null ? Number.NaN : Number(raw);
  return Number.isInteger(parsed) && parsed > 0
    ? Math.min(parsed, MAX_PAGE_SIZE)
    : MAX_PAGE_SIZE;
}

function studentView(raw: string | null): StudentView {
  return raw === "slim" || raw === "names" ? raw : "full";
}

export const GET = createGetHandler(
  async (request: NextRequest, token: string): Promise<StudentPage> => {
    const query = new URLSearchParams(request.nextUrl.searchParams);
    query.set("page_size", String(pageSize(query.get("page_size"))));
    const view = studentView(query.get("view"));

    try {
      const response = await apiGet<{
        data: BackendNameStudent[];
        pagination: StudentPage["pagination"];
      }>(`/api/students?${query.toString()}`, token);
      return {
        data: response.data.map((row) => mapStudentListRow(row, view)),
        pagination: response.pagination,
      };
    } catch (error) {
      const context = {
        error: error instanceof Error ? error.message : String(error),
      };
      // A rate-limited list is expected under load and stays a warning.
      if (isApiResponseError(error, 429)) {
        logger.warn("students fetch failed", {
          ...context,
          rate_limited: true,
        });
      } else {
        logger.error("students fetch failed", context);
      }
      throw error;
    }
  },
);

/**
 * Creates the student (with guardians and weekly schedules in the same
 * backend transaction), then the privacy consent the form changed. The consent
 * lives at another backend endpoint, so it cannot join that transaction: when
 * it fails, the child exists anyway and the answer carries the consent that is
 * actually stored and `privacy_consent_saved: false`, so the form can say that
 * the input was lost and the person can correct it on the child's page (#5970).
 * The marker is absent whenever nothing was lost.
 */
export const POST = createPostHandler<Student, StudentCreateBody>(
  async (_request, body, token) => {
    const backendRequest = buildStudentCreateRequest(body);
    const consent = sentConsentPair(body);
    const response = await apiPost<{ data: BackendStudent }>(
      "/api/students",
      token,
      backendRequest,
    );
    const student = response.data;

    let written = consent;
    if (consent) {
      try {
        await writeConsent(student.id, token, consent);
      } catch (consentError) {
        written = null;
        logger.error("privacy consent write failed after student create", {
          student_id: student.id,
          error:
            consentError instanceof Error
              ? consentError.message
              : String(consentError),
        });
      }
    }

    return {
      ...mapStudentResponse(student),
      ...(await readConsentAfterWrite(student.id, token, written)),
      ...(consent && !written && { privacy_consent_saved: false }),
    };
  },
);
