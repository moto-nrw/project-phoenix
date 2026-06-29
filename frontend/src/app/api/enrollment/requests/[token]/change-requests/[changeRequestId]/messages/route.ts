import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({
  component: "EnrollmentChangeRequestMessagesRoute",
});

interface RouteContext {
  params: Promise<{ token: string; changeRequestId: string }>;
}

export async function POST(request: NextRequest, context: RouteContext) {
  const { token, changeRequestId } = await context.params;
  if (!token || !changeRequestId) {
    return NextResponse.json(
      { error: "token and change request id required" },
      { status: 400 },
    );
  }

  try {
    const body = (await request.json()) as Record<string, unknown>;
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/requests/${encodeURIComponent(
        token,
      )}/change-requests/${encodeURIComponent(changeRequestId)}/messages`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      },
    );
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("change_request_reply_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
