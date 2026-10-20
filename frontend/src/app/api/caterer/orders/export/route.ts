import type { NextRequest } from "next/server";

import { catererAuth } from "~/server/auth/caterer";
import { withCatererAuth } from "~/server/auth/caterer-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";

// The caterer's delivery list of every school it supplies (ADR 0075), as
// PDF, Excel or CSV.
export const GET = withCatererAuth(
  async (request: NextRequest): Promise<Response> => {
    const session = await catererAuth();
    if (!session?.user?.token) {
      return problemResponse(401, "Unauthorized", request);
    }
    const response = await fetch(
      `${getServerApiUrl()}/caterer/orders/export${request.nextUrl.search}`,
      { headers: { Authorization: `Bearer ${session.user.token}` } },
    );
    if (!response.ok) {
      return forwardBackendResponse(response, request);
    }
    return new Response(await response.arrayBuffer(), {
      status: response.status,
      headers: {
        "Content-Type":
          response.headers.get("Content-Type") ?? "application/octet-stream",
        "Content-Disposition":
          response.headers.get("Content-Disposition") ?? "attachment",
      },
    });
  },
);
