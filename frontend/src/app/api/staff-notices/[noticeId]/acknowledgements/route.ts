import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/staff-notices/{id}/acknowledgements → Backend (#2208). Die
 * Bestätigungsliste einer Tagesinformation: wer wann zur Kenntnis genommen
 * hat. Im Backend adminexklusiv wie die übrige Verwaltung.
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const id = params.noticeId as string;
    if (!id) throw new Error("Notice ID is required");
    const response = await apiGet<{ data: unknown }>(
      `/api/staff-notices/${encodeURIComponent(id)}/acknowledgements`,
      token,
    );
    return response.data;
  },
);
