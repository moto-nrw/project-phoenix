import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const search = request.nextUrl.searchParams.toString();
    const path = `/api/timetable/gaps${search ? `?${search}` : ""}`;
    const response = await apiGet<{ data: unknown }>(path, token);
    return response.data;
  },
);
