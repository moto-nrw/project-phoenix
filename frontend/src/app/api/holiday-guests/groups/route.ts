import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/holiday-guests/groups → backend (#6680): die Feriengruppen,
 * deren Woche noch nicht begonnen hat, mit der Zahl der Feriengäste, die am
 * ersten Tag dazukommen. Keine Namen, keine Kinderdaten.
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    _params: Record<string, unknown>,
  ) => {
    const response = await apiGet<{ data: unknown }>(
      "/api/holiday-guests/groups",
      token,
    );
    return response.data;
  },
);
