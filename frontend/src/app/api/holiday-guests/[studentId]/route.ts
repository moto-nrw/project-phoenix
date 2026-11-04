import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { requirePathSegmentParam } from "~/lib/route-wrapper-utils.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/holiday-guests/{studentId} → backend (#4666): Betreuung,
 * Notfall und Abholung eines Feriengasts der ausrichtenden OGS. Das Backend
 * schreibt jeden Abruf ins Zugriffsprotokoll und antwortet sieben Tage nach
 * der Ferienwoche mit 404.
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const studentId = requirePathSegmentParam(params, "studentId");
    const response = await apiGet<{ data: unknown }>(
      `/api/holiday-guests/${studentId}`,
      token,
    );
    return response.data;
  },
);
