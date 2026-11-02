import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";
import {
  portalOfScope,
  sessionFamilyState,
} from "~/server/auth/token-validation";

const logger = createLogger({ component: "AuthTokenRoute" });

/**
 * Renews the tenant session for the browser's 401 retry. auth() runs the JWT
 * callback, which refreshes a token near expiry; withTenantAuth persists the
 * rotated cookie. The answer carries no token (#5005): the browser only
 * learns whether the session still holds one.
 */
async function POSTHandler(_request: NextRequest) {
  try {
    const session = await auth();

    if (!session?.user?.token || !session?.user?.refreshToken) {
      return problemResponse(401, "No refresh token found", _request);
    }
    // The browser retries after a backend 401. A revoked family writes no
    // ended cookie any more (#5399), so ask for it: the browser then signs
    // out instead of retrying with a session that opens nothing.
    if (
      (await sessionFamilyState(
        session.user.token,
        portalOfScope(session.user.scope),
      )) === "ended"
    ) {
      return problemResponse(401, "Session ended", _request);
    }

    return new Response(null, { status: 204 });
  } catch (error) {
    logger.error("token_refresh_error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal Server Error", _request);
  }
}

export const POST = withTenantAuth(POSTHandler);
