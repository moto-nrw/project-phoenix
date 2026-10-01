import type { NextRequest } from "next/server";
import { apiGet, apiPut } from "~/lib/api-helpers.server";
import { createGetHandler, createPutHandler } from "~/lib/route-wrapper.server";

interface CountScopeResponse {
  scope: string;
  has_own_groups?: boolean;
}

/**
 * Proxy GET /api/messages/count-scope → backend. Returns which parent
 * conversations the caller's own counter at "Nachrichten" counts, and whether
 * the caller has an OGS group today (#3673).
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string) => {
    const response = await apiGet<{ data: CountScopeResponse }>(
      "/api/messages/count-scope",
      token,
    );
    return response.data;
  },
);

/**
 * Proxy PUT /api/messages/count-scope → backend. Stores the caller's own
 * count scope; the backend validates the value.
 */
export const PUT = createPutHandler<CountScopeResponse, { scope?: unknown }>(
  async (_request: NextRequest, body, token: string) => {
    const response = await apiPut<{ data: CountScopeResponse }>(
      "/api/messages/count-scope",
      token,
      { scope: body.scope },
    );
    return response.data;
  },
);
