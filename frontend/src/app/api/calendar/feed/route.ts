import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

// Returns the current staff member's show-once calendar subscription link.
export const GET = createGetHandler(
  async (_request: NextRequest, token: string) => {
    const response = await apiGet<{ data: unknown }>(
      "/api/calendar/feed",
      token,
    );
    return response.data;
  },
);
