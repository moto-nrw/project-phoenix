import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/parent-announcements/{id}/declaration-status → backend.
 *
 * The Erklärung status (#3430): per child the state and who acted, the frozen
 * versions with their checksums, and the full history.
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
      `/api/parent-announcements/${encodeURIComponent(id)}/declaration-status`,
      token,
    );
    return response.data;
  },
);
