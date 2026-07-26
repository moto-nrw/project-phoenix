import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler, isStringParam } from "~/lib/route-wrapper.server";

interface ChangeHistoryResponse {
  data: unknown[];
}

/**
 * Proxy for GET /api/students/[id]/change-history (issue #1455).
 *
 * The backend gates access on full access (admin / group supervisor) and
 * returns the per-child change history. We forward and unwrap the envelope.
 */
export const GET = createGetHandler<unknown[]>(
  async (_request, token, params) => {
    if (!isStringParam(params.id)) {
      throw new Error("Student ID is required");
    }

    const response = await apiGet<ChangeHistoryResponse>(
      `/api/students/${encodeURIComponent(params.id)}/change-history`,
      token,
    );
    return response.data;
  },
);
