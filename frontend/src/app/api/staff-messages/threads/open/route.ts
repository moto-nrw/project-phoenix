import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy POST /api/staff-messages/threads/open → backend. Get-or-create for the
 * conversation with one colleague ({"account_id": "..."}).
 */
export const POST = createPostHandler(
  async (_request: NextRequest, body: unknown, token: string) => {
    const response = await apiPost<{ data: unknown }>(
      "/api/staff-messages/threads/open",
      token,
      body,
    );
    return response.data;
  },
);
