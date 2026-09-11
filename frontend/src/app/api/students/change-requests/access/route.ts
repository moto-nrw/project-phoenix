import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

type ParentRequestReviewAccess = "admin" | "group_leader" | "none";

interface ChangeRequestAccessResponse {
  readonly review_access: ParentRequestReviewAccess;
}

interface BackendEnvelope<T> {
  readonly data: T;
}

export const GET = createGetHandler<ChangeRequestAccessResponse>(
  async (_request, token) => {
    const response = await apiGet<BackendEnvelope<ChangeRequestAccessResponse>>(
      "/api/students/change-requests/access",
      token,
    );
    return response.data;
  },
);
