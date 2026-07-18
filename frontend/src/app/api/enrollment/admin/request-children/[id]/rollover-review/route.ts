import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { auth } from "~/server/auth";
import { withTenantAuth } from "~/server/auth/tenant-route";
import { getServerApiUrl } from "~/lib/server-api-url";
import { createLogger } from "~/lib/logger";

const logger = createLogger({
  component: "EnrollmentRolloverReviewDecideRoute",
});

interface RouteContext {
  params: Promise<{ id: string }>;
}

async function POSTHandler(request: NextRequest, context: RouteContext) {
  const { id } = await context.params;
  const session = await auth();
  const token = session?.user?.token;
  if (!token) {
    return NextResponse.json({ error: "Unauthenticated" }, { status: 401 });
  }
  try {
    const body = (await request.json()) as Record<string, unknown>;
    const response = await fetch(
      `${getServerApiUrl()}/api/enrollment/admin/request-children/${encodeURIComponent(id)}/rollover-review`,
      {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(body),
      },
    );
    const payload = await response.json().catch(() => ({}));
    return NextResponse.json(payload, { status: response.status });
  } catch (error) {
    logger.error("rollover_review_decide_failed", {
      error: error instanceof Error ? error.message : String(error),
    });
    return NextResponse.json(
      { error: "Internal Server Error" },
      { status: 500 },
    );
  }
}

export const POST = withTenantAuth(POSTHandler);
