import { type NextRequest, NextResponse } from "next/server";
import { schoolAuth } from "~/server/auth/school";
import { withSchoolAuth } from "~/server/auth/school-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "SchoolAuthLogoutRoute" });

// School logout (#2207): revokes the school refresh session through the
// shared scope-preserving backend /auth/logout — mirrors the tenant logout
// route, but reads the school NextAuth session.
async function POSTHandler(request: NextRequest) {
  try {
    const session = await schoolAuth();

    const refreshToken = session?.user?.refreshToken;
    if (!refreshToken) {
      return NextResponse.json({ error: "No active session" }, { status: 401 });
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

    if (!response.ok && response.status !== 204) {
      const errorText = await response.text();
      logger.error("logout backend error", {
        status: response.status,
        error: errorText,
      });
    }

    // Always return success to client
    return new NextResponse(null, { status: 204 });
  } catch (error) {
    logger.error("logout failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    // Still return success - logout should always succeed on client side
    return new NextResponse(null, { status: 204 });
  }
}

export const POST = withSchoolAuth(POSTHandler);
