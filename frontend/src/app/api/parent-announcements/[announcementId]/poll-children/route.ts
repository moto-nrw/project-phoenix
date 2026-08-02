import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/parent-announcements/{id}/poll-children → backend: every child
 * the Umfrage reaches with the answer given for them (empty = still open).
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.announcementId as string;
    if (!id) throw new Error("Announcement ID is required");
    const response = await apiGet<{ data: unknown }>(
      `/api/parent-announcements/${id}/poll-children`,
      token,
    );
    return response.data;
  },
);
