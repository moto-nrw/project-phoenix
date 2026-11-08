import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import { buildQueryString } from "~/lib/route-wrapper-utils.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";

/**
 * Type definition for visit creation request
 */
interface VisitCreateRequest {
  student_id: string;
  active_group_id: string;
  start_time?: string;
}

export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const endpoint = `/api/active/visits${buildQueryString(request)}`;

    const response = await apiGet(endpoint, token);

    // Extract data from backend response structure {status, data, message}
    if (response && typeof response === "object" && "data" in response) {
      return response.data;
    }

    return response;
  },
);

export const POST = createPostHandler<unknown, VisitCreateRequest>(
  async (
    _request: NextRequest,
    body: VisitCreateRequest,
    token: string,
    _params,
  ) => {
    return await apiPost("/api/active/visits", token, body);
  },
);
