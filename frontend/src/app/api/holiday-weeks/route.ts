import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/holiday-weeks → backend (#4665): die Ferienwochen, die die
 * Schule für ihren Träger ausrichtet, mit der Zahl der bestätigten Kinder.
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    _params: Record<string, unknown>,
  ) => {
    const response = await apiGet<{ data: unknown }>(
      "/api/holiday-weeks/",
      token,
    );
    return response.data;
  },
);
