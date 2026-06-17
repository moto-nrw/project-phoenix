import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/messages → backend. Returns the staff inbox of parent-OGS
 * message threads, optionally filtered to unread threads via `?unread=true`.
 * Backend scopes the threads to the staff member's tenant.
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const unread = request.nextUrl.searchParams.get("unread");
    const endpoint =
      unread !== null
        ? `/api/messages?unread=${encodeURIComponent(unread)}`
        : "/api/messages";
    const response = await apiGet<{ data: unknown }>(endpoint, token);
    return response.data;
  },
);
