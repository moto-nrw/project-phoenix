import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/messages → backend. Returns the staff inbox of parent-OGS
 * message threads, optionally filtered to unread threads (`?unread=true`).
 * Backend scopes the threads to the staff member's tenant.
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const incoming = request.nextUrl.searchParams;
    const params = new URLSearchParams();
    const unread = incoming.get("unread");
    if (unread !== null) params.set("unread", unread);
    const query = params.toString();
    const endpoint = query ? `/api/messages?${query}` : "/api/messages";
    const response = await apiGet<{ data: unknown }>(endpoint, token);
    return response.data;
  },
);
