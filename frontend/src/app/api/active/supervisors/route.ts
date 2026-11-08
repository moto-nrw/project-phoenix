import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import { buildQueryString } from "~/lib/route-wrapper-utils.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";

/**
 * Type definition for supervisor creation request
 */
interface SupervisorCreateRequest {
  staff_id: string;
  active_group_id: string;
}

export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const endpoint = `/active/supervisors${buildQueryString(request)}`;

    return await apiGet(endpoint, token);
  },
);

export const POST = createPostHandler<unknown, SupervisorCreateRequest>(
  async (
    _request: NextRequest,
    body: SupervisorCreateRequest,
    token: string,
    _params,
  ) => {
    return await apiPost("/active/supervisors", token, body);
  },
);
