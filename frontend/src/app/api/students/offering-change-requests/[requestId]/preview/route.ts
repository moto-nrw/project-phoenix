import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

interface PreviewBody {
  excluded_offering_ids: string[];
  /** Date currently chosen in the review card (#2484). */
  effective_from?: string;
}

interface BackendEnvelope<T> {
  data: T;
}

export const POST = createPostHandler<unknown, PreviewBody>(
  async (_request, body, token, params) => {
    const requestId = String(params.requestId);
    const response = await apiPost<BackendEnvelope<unknown>, PreviewBody>(
      `/api/students/offering-change-requests/${encodeURIComponent(requestId)}/preview`,
      token,
      body,
    );
    return response.data;
  },
);
