import { captureBffException } from "~/lib/sentry-bff.server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "AuthLogoutRoute" });

async function POSTHandler(request: NextRequest) {
  try {
    const session = await auth();

    const refreshToken = session?.user?.refreshToken;
    if (!refreshToken) {
      return problemResponse(401, "No active session", request);
    }

    // Forward the refresh token to the backend. The backend /auth/logout route
    // is guarded by AuthenticateRefreshJWT, so we must send the refresh token,
    // not the access token.
    const response = await fetch(`${getServerApiUrl()}/auth/logout`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${refreshToken}`,
        "Content-Type": "application/json",
        ...getClientForwardHeaders(request),
      },
    });

    return forwardBackendResponse(response, request);
  } catch (error) {
    captureBffException(error, request);
    logger.error("logout failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    // There is no backend response to forward. The browser still clears its
    // local session after this request, independently of this status.
    return problemResponse(502, "Backend unavailable", request);
  }
}

export const POST = withTenantAuth(POSTHandler);
