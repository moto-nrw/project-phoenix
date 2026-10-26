import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/holiday-weeks/{weekId}/participants → backend (#4665): die
 * bestätigten Kinder einer Woche, die die Schule ausrichtet, mit den Zahlen
 * je Zeitmodell und Heimatschule. Das Backend schreibt jeden Abruf ins
 * Zugriffsprotokoll und antwortet nach dem letzten Tag der Woche mit 404.
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const weekId = requirePathSegmentParam(params, "weekId");
    const response = await apiGet<{ data: unknown }>(
      `/api/holiday-weeks/${weekId}/participants`,
      token,
    );
    return response.data;
  },
);
