import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/staff-messages/unread-count → backend. Drives the Team-Chat
 * sidebar badge.
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string) => {
    const response = await apiGet<{ data: { unread_count: number } }>(
      "/api/staff-messages/unread-count",
      token,
    );
    return response.data;
  },
);
