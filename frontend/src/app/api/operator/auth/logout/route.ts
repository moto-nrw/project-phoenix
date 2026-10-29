import { captureBffException } from "~/lib/sentry-bff.server";
import type { NextRequest } from "next/server";
import { operatorAuth, uncachedOperatorAuth } from "~/server/auth/operator";
import { withOperatorAuth } from "~/server/auth/operator-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import { forwardBackendResponse } from "~/lib/backend-proxy-response.server";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "OperatorAuthLogoutRoute" });

function revokeFamily(request: NextRequest, accessToken: string) {
  // The backend route runs behind the operator session chain and revokes the
  // family this access token names.
  return fetch(`${getServerApiUrl()}/operator/auth/logout`, {
    method: "POST",
    headers: {
      Authorization: `Bearer ${accessToken}`,
      "Content-Type": "application/json",
      ...getClientForwardHeaders(request),
    },
  });
}

// Operator logout (#5194): revokes the operator refresh-token family, so the
// operator's access tokens end at once, also in a cookie a late response
// writes back. The browser takes 401 for "nothing left to revoke", so a 401
// passes only once the session is known to be over: an expired access token
// whose renewal failed for a while gets 503 and the logout is retried.
async function POSTHandler(request: NextRequest) {
  try {
    const session = await operatorAuth();
    const accessToken = session?.user?.token;
    if (!accessToken) {
      return problemResponse(401, "No active session", request);
    }

    const response = await revokeFamily(request, accessToken);
    if (response.status !== 401) {
      return forwardBackendResponse(response, request);
    }
    // The access token was refused. Renew the session once: a revoked family
    // ends it, a live one yields a token to revoke it with.
    const renewed = await uncachedOperatorAuth();
    const renewedToken = renewed?.user?.token;
    if (!renewedToken) {
      return forwardBackendResponse(response, request);
    }
    if (renewedToken === accessToken) {
      return problemResponse(
        503,
        "Logout could not confirm the revocation",
        request,
      );
    }
    return forwardBackendResponse(
      await revokeFamily(request, renewedToken),
      request,
    );
  } catch (error) {
    captureBffException(error, request);
    logger.error("logout failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(502, "Backend unavailable", request);
  }
}

export const POST = withOperatorAuth(POSTHandler);
