import type { NextRequest } from "next/server";
import { apiPost } from "~/lib/api-helpers.server";
import { createPostHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy POST /api/parent-announcements/{id}/remind → backend: notify the
 * guardians of children that have not answered the Umfrage yet.
 */
export const POST = createPostHandler(
  async (
    _request: NextRequest,
    _body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.announcementId as string;
    if (!id) throw new Error("Announcement ID is required");
    const response = await apiPost<{ data: unknown }>(
      `/api/parent-announcements/${id}/remind`,
      token,
      {},
    );
    return response.data;
  },
);
