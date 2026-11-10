import type { NextRequest } from "next/server";
import { apiGet } from "~/lib/api-helpers.server";
import { buildQueryString } from "~/lib/route-wrapper-utils.server";
import { createGetHandler } from "~/lib/route-wrapper.server";

export const GET = createGetHandler(
  async (request: NextRequest, token: string) => {
    const endpoint = `/api/active/visits${buildQueryString(request)}`;

    const response = await apiGet(endpoint, token);

    // Extract data from backend response structure {status, data, message}
    if (response && typeof response === "object" && "data" in response) {
      return response.data;
    }

    return response;
  },
);
