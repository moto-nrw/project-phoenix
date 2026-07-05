import type { NextRequest } from "next/server";
import { apiGet, apiPost } from "~/lib/api-helpers.server";
import {
  createGetHandler,
  createPostHandler,
} from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/parent-announcements → backend. Returns the tenant's parent
 * announcements (Elternmitteilungen) newest-first. `?include_inactive=true`
 * includes soft-disabled rows. Backend scopes to the staff member's tenant and
 * requires communications:announce.
 */
export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const includeInactive =
      request.nextUrl.searchParams.get("include_inactive") === "true";
    const endpoint = includeInactive
      ? "/api/parent-announcements?include_inactive=true"
      : "/api/parent-announcements";
    const response = await apiGet<{ data: unknown }>(endpoint, token);
    return response.data;
  },
);

/**
 * Proxy POST /api/parent-announcements → backend. Creates a draft announcement
 * with its targets.
 */
export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/parent-announcements",
      token,
      body,
    );
    return response.data;
  },
);
