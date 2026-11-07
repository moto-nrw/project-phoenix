import { withPrivateDownloadNoStore } from "~/lib/private-no-store";
import type { NextRequest } from "next/server";
import { sendWithTokenRefresh } from "~/lib/token-refresh-retry.server";

import { auth, uncachedAuth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";

export const GET = withPrivateDownloadNoStore(
  withTenantAuth(async (request: NextRequest): Promise<Response> => {
    const session = await auth();
    if (!session?.user?.token) {
      return problemResponse(401, "Unauthorized", request);
    }
    const makeRequest = (token: string) =>
      fetch(
        `${getServerApiUrl()}/api/meal-plan/orders/month/export${request.nextUrl.search}`,
        { headers: { Authorization: `Bearer ${token}` } },
      );
    const response = await sendWithTokenRefresh(
      session.user.token,
      makeRequest,
      () => uncachedAuth(),
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
  }),
);
