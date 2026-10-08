import { apiGet } from "~/lib/api-helpers.server";
import type {
  ParentRequestReviewAccess,
  StudentRequestReviewCoverage,
} from "~/lib/change-request-access";
import { createGetHandler } from "~/lib/route-wrapper.server";

interface ChangeRequestAccessResponse {
  readonly review_access: ParentRequestReviewAccess;
  readonly student?: StudentRequestReviewCoverage;
}

interface BackendEnvelope<T> {
  readonly data: T;
}

const BASE_PATH = "/api/students/change-requests/access";

export const GET = createGetHandler<ChangeRequestAccessResponse>(
  async (request, token) => {
    // student_id asks whether the review scope reaches one child (#3886); the
    // backend validates it, so it is only passed on when present.
    const studentId = request.nextUrl.searchParams.get("student_id");
    const path =
      studentId === null
        ? BASE_PATH
        : `${BASE_PATH}?student_id=${encodeURIComponent(studentId)}`;
    const response = await apiGet<BackendEnvelope<ChangeRequestAccessResponse>>(
      path,
      token,
    );
    return response.data;
  },
);
