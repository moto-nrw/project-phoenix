import type { NextRequest } from "next/server";
import { apiGet, extractParams } from "~/lib/api-helpers";
import { createGetHandler } from "~/lib/route-wrapper";

export const GET = createGetHandler(
  async (request: NextRequest, token: string, params) => {
    const queryParams = extractParams(request, params);
    const queryString = new URLSearchParams(queryParams).toString();
    const endpoint = `/api/timetable/operations/planned-now${queryString ? `?${queryString}` : ""}`;
    const response = await apiGet<{ data: unknown }>(endpoint, token);
    return response.data;
  },
);
