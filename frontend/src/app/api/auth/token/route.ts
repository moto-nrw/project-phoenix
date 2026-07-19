import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthTokenRoute" });

async function POSTHandler(_request: NextRequest) {
  try {
    const session = await auth();

    if (!session?.user?.token || !session?.user?.refreshToken) {
      return NextResponse.json(
        { error: "No refresh token found" },
        { status: 401 },
      );
    }

    return NextResponse.json({
      access_token: session.user.token,
      refresh_token: session.user.refreshToken,
    });
  } catch (error) {
    logger.error("token_refresh_error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const POST = withTenantAuth(POSTHandler);
