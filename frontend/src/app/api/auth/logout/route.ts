import { type NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthLogoutRoute" });

async function POSTHandler(request: NextRequest) {
  try {
    const session = await auth();

    const refreshToken = session?.user?.refreshToken;
    if (!refreshToken) {
      return NextResponse.json({ error: "No active session" }, { status: 401 });
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

export const POST = withTenantAuth(POSTHandler);
