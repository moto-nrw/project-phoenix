// app/api/settings/initialize/route.ts
import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers";
import { createPostHandler } from "~/lib/route-wrapper";

/**
 * Handler for POST /api/settings/initialize
 * Initializes/syncs setting definitions from code to database (admin only)
 */
export const POST = createPostHandler<unknown, Record<string, never>>(
  async (
    _request: NextRequest,
    _body: Record<string, never>,
    token: string,
  ) => {
    return await apiPost("/api/settings/initialize", token, {});
  },
);
