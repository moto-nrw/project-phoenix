import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/holiday-weeks/{weekId}/own-children → backend (#6678): zu
 * jedem eigenen Kind der Woche, ob seine Anmeldung einem Kind der Schule
 * zugeordnet ist. Die Antwort nennt Kinder nur über die ID der
 * Teilnehmerliste.
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const weekId = requirePathSegmentParam(params, "weekId");
    const response = await apiGet<{ data: unknown }>(
      `/api/holiday-weeks/${weekId}/own-children`,
      token,
    );
    return response.data;
  },
);
