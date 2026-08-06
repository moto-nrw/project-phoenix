import type { NextRequest } from "next/server";
import { fetchBirthdayOverview } from "~/lib/birthdays-api.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * GET /api/birthdays — today's birthdays for the dashboard (#1542).
 * Monday additionally carries the weekend before it.
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    _params: Record<string, unknown>,
  ) => {
    return await fetchBirthdayOverview(token);
  },
);
