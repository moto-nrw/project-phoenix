import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const search = request.nextUrl.searchParams.toString();
    const response = await apiGet<{ data: unknown }>(
      `/api/calendar/recipient-options${search ? `?${search}` : ""}`,
      token,
    );
    return response.data;
  },
);
