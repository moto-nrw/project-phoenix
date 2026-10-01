import type { NextRequest } from "next/server";
import { fetchBirthdayOverview } from "~/lib/birthdays-api.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

/**
 * GET /api/birthdays — the birthdays of one week for the dashboard (#1542,
 * #3777). `?week_start=YYYY-MM-DD` picks the week; without it the current
 * week is shown.
 */
export const GET = createGetHandler(
  async (
    request: NextRequest,
    token: string,
    _params: Record<string, unknown>,
  ) => {
    return await fetchBirthdayOverview(
      token,
      request.nextUrl.searchParams.get("week_start"),
    );
  },
);
