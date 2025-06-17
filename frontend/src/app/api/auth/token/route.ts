import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import { auth } from "~/server/auth";
import { env } from "~/env";

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
    const apiUrl = process.env.NODE_ENV === 'production' || process.env.DOCKER_ENV
      ? 'http://server:8080'
      : env.NEXT_PUBLIC_API_URL;
    const backendResponse = await fetch(
      `${apiUrl}/auth/refresh`,
      {
        method: "POST",
        headers: {
          Authorization: `Bearer ${session.user.refreshToken}`,
        },
      },
    );

    if (!backendResponse.ok) {
      return NextResponse.json(
        { error: "Failed to refresh token" },
        { status: backendResponse.status },
      );
    }

    const tokens = (await backendResponse.json()) as TokenResponse;

    return NextResponse.json({
      access_token: tokens.access_token,
      refresh_token: tokens.refresh_token,
    });
  } catch {
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
