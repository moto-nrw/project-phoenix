import type { NextRequest } from "next/server";
import { apiPut } from "~/lib/api-helpers.server";
import { createPutHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy PUT /api/parent-announcements/{id}/reminder → backend: move, reword
 * or remove (reminder_at null) the scheduled reminder of an announcement
 * (#3162). This is the one edit a published announcement still accepts, until
 * the reminder has been sent. Distinct from POST /remind, the manual nudge to
 * whoever still owes an answer.
 */
export const PUT = createPutHandler(
  async (
    _request: NextRequest,
    body: unknown,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.announcementId as string;
    if (!id) throw new Error("Announcement ID is required");
    const response = await apiPut<{ data: unknown }>(
      `/api/parent-announcements/${id}/reminder`,
      token,
      body,
    );
    return response.data;
  },
);
