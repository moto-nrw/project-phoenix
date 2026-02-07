import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "InvitationValidateRoute" });

export async function GET(request: NextRequest) {
  const token = request.nextUrl.searchParams.get("token");
  if (!token) {
    return NextResponse.json(
      { error: "Missing invitation token" },
      { status: 400 },
    );
  }

  try {
    const response = await fetch(
      `${getServerApiUrl()}/auth/invitations/${encodeURIComponent(token)}`,
    );
    const contentType = response.headers.get("Content-Type") ?? "";
    let payload: unknown = null;

    if (contentType.includes("application/json")) {
      payload = await response.json();
    } else {
      const text = await response.text();
      payload = text ? { error: text } : null;
    }

    return NextResponse.json(payload ?? {}, { status: response.status });
  } catch (error) {
    logger.error("invitation validation failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
