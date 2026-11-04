import { captureBffException } from "~/lib/sentry-bff.server";
import type { NextRequest } from "next/server";
import { parentAuth } from "~/server/auth/parent";
import { withParentAuth } from "~/server/auth/parent-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "ParentAuthLogoutRoute" });

// Parent logout (#5194): revokes the parent refresh-token family through the
// shared backend /auth/logout, so the parent's access tokens end at once,
// also in a cookie a late response writes back. Reads the parent session.
async function POSTHandler(request: NextRequest) {
  try {
    const session = await parentAuth();

    const refreshToken = session?.user?.refreshToken;
    if (!refreshToken) {
      return problemResponse(401, "No active session", request);
    }

    // The backend /auth/logout route is guarded by AuthenticateRefreshJWT,
    // so we must send the refresh token, not the access token.
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
    return problemResponse(502, "Backend unavailable", request);
  }
}

export const POST = withParentAuth(POSTHandler);
