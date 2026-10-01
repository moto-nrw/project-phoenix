import type { NextRequest } from "next/server";

import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";

export const GET = withTenantAuth(
  async (request: NextRequest): Promise<Response> => {
    const session = await auth();
    if (!session?.user?.token) {
      return Response.json({ error: "Unauthorized" }, { status: 401 });
    }
    const response = await fetch(
      `${getServerApiUrl()}/api/meal-plan/participants/export${request.nextUrl.search}`,
      { headers: { Authorization: `Bearer ${session.user.token}` } },
    );
    if (!response.ok) {
      return forwardBackendResponse(response);
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
