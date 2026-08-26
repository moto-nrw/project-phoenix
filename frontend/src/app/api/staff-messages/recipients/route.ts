import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * Proxy GET /api/staff-messages/recipients → backend. Returns the colleagues
 * the caller may write to (active members of this school, minus themselves).
 */
export const GET = createGetHandler(
  async (_request: NextRequest, token: string) => {
    const response = await apiGet<{ data: unknown }>(
      "/api/staff-messages/recipients",
      token,
    );
    return response.data;
  },
);
