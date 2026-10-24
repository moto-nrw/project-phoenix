import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { createLogger } from "~/lib/logger";
import { problemResponse } from "~/lib/api-helpers.server";

const logger = createLogger({ component: "AuthTokenRoute" });

async function POSTHandler(_request: NextRequest) {
  try {
    const session = await auth();

    if (!session?.user?.token || !session?.user?.refreshToken) {
      return problemResponse(401, "No refresh token found", _request);
    }

    return NextResponse.json({
      access_token: session.user.token,
      refresh_token: session.user.refreshToken,
    });
  } catch (error) {
    logger.error("token_refresh_error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return problemResponse(500, "Internal Server Error", _request);
  }
}

export const POST = withTenantAuth(POSTHandler);
