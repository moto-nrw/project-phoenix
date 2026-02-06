import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth, signIn } from "~/server/auth";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "AuthTokenRoute" });

interface TokenResponse {
  access_token: string;
  refresh_token: string;
}

export async function POST(_request: NextRequest) {
  try {
    const session = await auth();

    if (!session?.user?.refreshToken) {
      return NextResponse.json(
        { error: "No refresh token found" },
        { status: 401 },
      );
    }

    // Check for roles - continue even if no roles present

    // Send refresh token request to backend
    // Use server URL in server context (Docker environment)
    const apiUrl = getServerApiUrl();

    // The backend expects the refresh token in Authorization header
    const backendResponse = await fetch(`${apiUrl}/auth/refresh`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${session.user.refreshToken}`,
        "Content-Type": "application/json",
      },
    });

    if (!backendResponse.ok) {
      logger.error("backend refresh failed", {
        status: backendResponse.status,
      });
      return NextResponse.json(
        { error: "Failed to refresh token" },
        { status: backendResponse.status },
      );
    }

    const tokens = (await backendResponse.json()) as TokenResponse;

    logger.debug("backend token refresh successful");

    // Persist refreshed tokens back into the Auth.js session so subsequent requests reuse them
    try {
      await signIn("credentials", {
        redirect: false,
        internalRefresh: "true",
        token: tokens.access_token,
        refreshToken: tokens.refresh_token,
      });
    } catch (signInError) {
      logger.error("failed to update session after backend refresh", {
        error:
          signInError instanceof Error
            ? signInError.message
            : String(signInError),
      });
      return NextResponse.json(
        { error: "Failed to refresh token" },
        { status: 500 },
      );
    }

    return NextResponse.json({
      access_token: tokens.access_token,
      refresh_token: tokens.refresh_token,
    });
  } catch (error) {
    logger.error("token refresh error", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
