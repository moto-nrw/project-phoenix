import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const search = request.nextUrl.searchParams.toString();
    const path = `/api/timetable/exception-conflicts${search ? `?${search}` : ""}`;
    const response = await apiGet<{ data: unknown }>(path, token);
    return response.data;
  },
);
