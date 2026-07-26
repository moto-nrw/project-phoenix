import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/messages/unread-count → backend. Returns the number of
 * unread parent-OGS message threads for the staff member's tenant, used to
 * drive the sidebar unread badge.
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string) => {
    const response = await apiGet<{ data: { unread_count: number } }>(
      "/api/messages/unread-count",
      token,
    );
    return response.data;
  },
);
