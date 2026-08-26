import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/staff-messages → backend. Returns the caller's OGS-internal
 * conversations (#2598), newest activity first. `only_unread=true` narrows it.
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const onlyUnread =
      request.nextUrl.searchParams.get("only_unread") === "true";
    const response = await apiGet<{ data: unknown }>(
      `/api/staff-messages${onlyUnread ? "?only_unread=true" : ""}`,
      token,
    );
    return response.data;
  },
);
