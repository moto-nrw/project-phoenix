// Settings value history API route
import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers";
import { createGetHandler, isStringParam } from "~/lib/route-wrapper";

export const GET = createGetHandler(
  async (request: NextRequest, token: string, params) => {
    if (!isStringParam(params.key)) {
      throw new Error("Invalid key parameter");
    }

    const queryParams = new URLSearchParams();
    request.nextUrl.searchParams.forEach((value, key) => {
      queryParams.append(key, value);
    });
    const queryString = queryParams.toString();
    const endpoint = `/api/settings/values/${encodeURIComponent(params.key)}/history${queryString ? `?${queryString}` : ""}`;

    return await apiGet(endpoint, token);
  },
);
