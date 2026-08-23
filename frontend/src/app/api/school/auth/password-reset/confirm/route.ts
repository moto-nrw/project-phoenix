import { type NextRequest, NextResponse } from "next/server";
import { getClientForwardHeaders } from "~/lib/client-headers.server";
import { createLogger } from "~/lib/logger";
import { getServerApiUrl } from "~/lib/server-api-url";

const logger = createLogger({ component: "SchoolPasswordResetConfirmRoute" });

export async function POST(request: NextRequest) {
  try {
    const response = await fetch(
      `${getServerApiUrl()}/school/auth/password-reset/confirm`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          ...getClientForwardHeaders(request),
        },
        body: JSON.stringify(await request.json()),
      },
    );
    const body = await response.text();
    return new NextResponse(body, {
      status: response.status,
      headers: {
        "Content-Type":
          response.headers.get("Content-Type") ?? "application/json",
      },
    });
  } catch (error) {
    logger.error("school password reset confirmation failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
