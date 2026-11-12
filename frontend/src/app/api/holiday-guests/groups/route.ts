import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/holiday-guests/groups → backend: jede Feriengruppe einer
 * ausgerichteten Woche, die noch nicht vorbei ist, mit Woche und Zahl der
 * Feriengäste (ADR 0120, Nachtrag 2026-11-11). Das Backend verlangt das Recht
 * der Gruppenlisten.
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
