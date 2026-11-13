import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/holiday-weeks/home-school-children → backend (#6679): die
 * bestätigten Ferienbuchungen der eigenen Kinder in Wochen, die eine andere
 * OGS des Trägers ausrichtet. Ohne Gesundheitsangaben und Kontakte.
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    _params: Record<string, unknown>,
  ) => {
    const response = await apiGet<{ data: unknown }>(
      "/api/holiday-weeks/home-school-children",
      token,
    );
    return response.data;
  },
);
