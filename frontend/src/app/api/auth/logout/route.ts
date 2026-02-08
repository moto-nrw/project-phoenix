import { type NextRequest, NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthLogoutRoute" });

export async function POST(_request: NextRequest) {
  try {
    const session = await auth();

    if (!session?.user?.token) {
      return NextResponse.json({ error: "No active session" }, { status: 401 });
    }

    // Forward to backend
    const response = await fetch(`${getServerApiUrl()}/auth/logout`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${session.user.token}`,
        "Content-Type": "application/json",
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
