import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/holiday-guests → backend (#5630): der Hinweis „Gast aus …“
 * für jeden Feriengast, den die ausrichtende OGS heute betreut. Das Backend
 * verlangt das Recht der Kinderliste.
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    _params: Record<string, unknown>,
  ) => {
    const response = await apiGet<{ data: unknown }>(
      "/api/holiday-guests/",
      token,
    );
    return response.data;
  },
);
