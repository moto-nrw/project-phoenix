import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import type { NextRequest } from "next/server";

import { catererAuth, uncachedCatererAuth } from "~/server/auth/caterer";
import { withCatererAuth } from "~/server/auth/caterer-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";

// The caterer's delivery list of every school it supplies (ADR 0077), as
// PDF, Excel or CSV.
export const GET = withPrivateDownloadNoStore(
  withCatererAuth(async (request: NextRequest): Promise<Response> => {
    const session = await catererAuth();
    if (!session?.user?.token) {
      return problemResponse(401, "Unauthorized", request);
    }
    const makeRequest = (token: string) =>
      fetch(
        `${getServerApiUrl()}/caterer/orders/export${request.nextUrl.search}`,
        { headers: { Authorization: `Bearer ${token}` } },
      );
    let response = await makeRequest(session.user.token);
    if (response.status === 401) {
      const refreshed = await uncachedCatererAuth();
      if (
        refreshed?.user?.token &&
        refreshed.user.token !== session.user.token
      ) {
        response = await makeRequest(refreshed.user.token);
      }
    }
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
  }),
);
