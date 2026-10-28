import { captureBffException } from "~/lib/sentry-bff.server";
import type { NextRequest } from "next/server";
import { operatorAuth } from "~/server/auth/operator";
import { withOperatorAuth } from "~/server/auth/operator-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "OperatorAuthLogoutRoute" });

// Operator logout (#5194): revokes the operator refresh-token family, so the
// operator's access tokens end at once, also in a cookie a late response
// writes back. Reads the operator session.
async function POSTHandler(request: NextRequest) {
  try {
    const session = await operatorAuth();

    // The session read renews an access token near its expiry, so the token
    // the backend checks against its live family is current.
    const accessToken = session?.user?.token;
    if (!accessToken) {
      return problemResponse(401, "No active session", request);
    }

    // The backend route runs behind the operator session chain and revokes
    // the family this access token names.
    const response = await fetch(`${getServerApiUrl()}/operator/auth/logout`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${accessToken}`,
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

export const POST = withOperatorAuth(POSTHandler);
