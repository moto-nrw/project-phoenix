import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "EnrollmentWithdrawRoute" });

interface RouteContext {
  params: Promise<{ token: string }>;
}

export async function POST(request: NextRequest, context: RouteContext) {
  const { token } = await context.params;
  if (!token) {
    return NextResponse.json({ error: "token required" }, { status: 400 });
  }
  try {
    let body: Record<string, unknown> = {};
    if (request.headers.get("content-length") !== "0") {
      try {
        body = (await request.json()) as Record<string, unknown>;
      } catch {
        body = {};
      }
    }
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/requests/${encodeURIComponent(token)}/withdraw`,
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      },
    );
    if (response.status === 204) {
      return new NextResponse(null, { status: 204 });
    }
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("status_withdraw_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}
